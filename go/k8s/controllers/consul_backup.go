package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/hashicorp/consul/api"
	"go.f110.dev/xerrors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"go.f110.dev/mono/go/api/consulv1alpha1"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/storage"
)

type ConsulBackupController struct {
	*controllerutil.ControllerBase

	client            *client.ConsulV1alpha1
	coreClient        kubernetes.Interface
	config            *rest.Config
	runOutsideCluster bool

	backupLister  *client.ConsulV1alpha1ConsulBackupLister
	serviceLister corev1listers.ServiceLister
	secretLister  corev1listers.SecretLister

	// for testing
	transport http.RoundTripper
}

var _ controllerutil.Controller = &ConsulBackupController{}

func NewConsulBackupController(
	coreSharedInformerFactory kubeinformers.SharedInformerFactory,
	factory *client.InformerFactory,
	coreClient kubernetes.Interface,
	apiClient *client.Set,
	config *rest.Config,
	runOutsideCluster bool,
) (*ConsulBackupController, error) {
	serviceInformer := coreSharedInformerFactory.Core().V1().Services()
	secretInformer := coreSharedInformerFactory.Core().V1().Secrets()

	informers := client.NewConsulV1alpha1Informer(factory.Cache(), apiClient.ConsulV1alpha1, metav1.NamespaceAll, 30*time.Second)
	backupInformer := informers.ConsulBackupInformer()

	b := &ConsulBackupController{
		client:            apiClient.ConsulV1alpha1,
		coreClient:        coreClient,
		config:            config,
		runOutsideCluster: runOutsideCluster,
		backupLister:      informers.ConsulBackupLister(),
		serviceLister:     serviceInformer.Lister(),
		secretLister:      secretInformer.Lister(),
	}
	b.ControllerBase = controllerutil.NewBase(
		"consul-backup-controller",
		b,
		coreClient,
		[]cache.SharedIndexInformer{backupInformer},
		[]cache.SharedIndexInformer{serviceInformer.Informer(), secretInformer.Informer()},
		[]string{},
	)

	return b, nil
}

func (b *ConsulBackupController) ObjectToKeys(obj interface{}) []string {
	switch v := obj.(type) {
	case *consulv1alpha1.ConsulBackup:
		key, err := cache.MetaNamespaceKeyFunc(v)
		if err != nil {
			return nil
		}
		return []string{key}
	default:
		return nil
	}
}

func (b *ConsulBackupController) GetObject(key string) (runtime.Object, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	backup, err := b.backupLister.Get(namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, xerrors.WithStack(err)
	}
	if apierrors.IsNotFound(err) {
		return nil, nil
	}

	return backup, nil
}

func (b *ConsulBackupController) UpdateObject(ctx context.Context, obj runtime.Object) (runtime.Object, error) {
	backup, ok := obj.(*consulv1alpha1.ConsulBackup)
	if !ok {
		return nil, xerrors.Definef("unexpected object type: %T", obj).WithStack()
	}

	updatedBackup, err := b.client.UpdateConsulBackup(ctx, backup, metav1.UpdateOptions{})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return updatedBackup, nil
}

func (b *ConsulBackupController) Reconcile(ctx context.Context, obj runtime.Object) error {
	backup := obj.(*consulv1alpha1.ConsulBackup)
	updated := backup.DeepCopy()
	now := metav1.Now()

	if backup.Status.Succeeded && backup.Status.LastSucceededTime != nil {
		nextBackupTime := backup.Status.LastSucceededTime.Add(time.Duration(backup.Spec.IntervalInSeconds) * time.Second)
		if now.Before(&metav1.Time{Time: nextBackupTime}) {
			return nil
		}
	}

	consulClient, err := api.NewClient(&api.Config{
		Address: fmt.Sprintf("http://%s.%s.svc:8500", backup.Spec.Service.Name, backup.Namespace),
		HttpClient: &http.Client{
			Transport: b.transport,
		},
	})
	if err != nil {
		return xerrors.WithStack(err)
	}
	buf, _, err := consulClient.Snapshot().Save(&api.QueryOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}

	history := &consulv1alpha1.ConsulBackupStatusHistory{
		ExecuteTime: &now,
	}
	if err := b.storeBackupFile(ctx, backup, history, buf, 0, now); err != nil {
		return xerrors.WithStack(err)
	}

	if err := b.rotateBackupFiles(ctx, backup); err != nil {
		return xerrors.WithStack(err)
	}

	if history.Succeeded {
		updated.Status.Succeeded = true
		updated.Status.LastSucceededTime = &now
	}
	updated.Status.BackupStatusHistory = append(updated.Status.BackupStatusHistory, *history)
	succeededCount := 0
	firstIndex := 0
	for i := len(updated.Status.BackupStatusHistory) - 1; i >= 0; i-- {
		if updated.Status.BackupStatusHistory[i].Succeeded {
			succeededCount++
			firstIndex = i
		}
		if succeededCount == updated.Spec.MaxBackups {
			break
		}
	}
	if succeededCount == updated.Spec.MaxBackups && firstIndex+1 < len(updated.Status.BackupStatusHistory) {
		updated.Status.BackupStatusHistory = updated.Status.BackupStatusHistory[firstIndex:]
	}
	if !reflect.DeepEqual(backup.Status, updated.Status) {
		_, err := b.client.UpdateStatusConsulBackup(ctx, updated, metav1.UpdateOptions{})
		if err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func (b *ConsulBackupController) Finalize(ctx context.Context, obj runtime.Object) error {
	panic("implement me")
}

func (b *ConsulBackupController) storeBackupFile(
	ctx context.Context,
	backup *consulv1alpha1.ConsulBackup,
	history *consulv1alpha1.ConsulBackupStatusHistory,
	data io.Reader,
	dataSize int64,
	t metav1.Time,
) error {
	switch {
	case backup.Spec.Storage.MinIO != nil:
		spec := backup.Spec.Storage.MinIO

		accessKeySecret, err := b.secretLister.Secrets(backup.Namespace).Get(spec.Credential.AccessKeyID.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}
		accessKey, ok := accessKeySecret.Data[spec.Credential.AccessKeyID.Key]
		if !ok {
			return xerrors.Definef("access key %s not found in %s", spec.Credential.AccessKeyID.Key, accessKeySecret.Name).WithStack()
		}
		secretAccessKeySecret, err := b.secretLister.Secrets(backup.Namespace).Get(spec.Credential.SecretAccessKey.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}
		secretAccessKey, ok := secretAccessKeySecret.Data[spec.Credential.SecretAccessKey.Key]
		if !ok {
			return xerrors.Definef("secret access key %s not found in %s", spec.Credential.AccessKeyID.Key, accessKeySecret.Name).WithStack()
		}

		mcOpt := storage.NewMinIOOptionsViaService(b.coreClient, b.config, spec.Service.Name, spec.Service.Namespace, 9000, string(accessKey), string(secretAccessKey), b.runOutsideCluster)
		mcOpt.Transport = b.transport
		mc := storage.NewMinIOStorage(spec.Bucket, mcOpt)
		filename := fmt.Sprintf("%s_%d", backup.Name, t.Unix())
		path := spec.Path
		if path[0] == '/' {
			path = path[1:]
		}
		history.Path = filepath.Join(path, filename)
		if err := mc.PutReader(ctx, filepath.Join(path, filename), data); err != nil {
			return xerrors.WithStack(err)
		}

		history.Succeeded = true
		return nil
	case backup.Spec.Storage.GCS != nil:
		spec := backup.Spec.Storage.GCS
		credential, err := b.secretLister.Secrets(backup.Namespace).Get(spec.Credential.ServiceAccountJSON.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}
		b, ok := credential.Data[spec.Credential.ServiceAccountJSON.Key]
		if !ok {
			return xerrors.Definef("%s is not found in %s", spec.Credential.ServiceAccountJSON.Key, spec.Credential.ServiceAccountJSON.Name).WithStack()
		}

		gcsClient := storage.NewGCS(b, spec.Bucket, storage.GCSOptions{})
		filename := fmt.Sprintf("%s_%d", backup.Name, t.Unix())
		history.Path = filepath.Join(spec.Path, filename)
		if err := gcsClient.PutReader(ctx, filepath.Join(spec.Path, filename), data); err != nil {
			return xerrors.WithStack(err)
		}

		history.Succeeded = true
		return nil
	default:
		return xerrors.New("Not configured a storage")
	}
}

func (b *ConsulBackupController) rotateBackupFiles(ctx context.Context, backup *consulv1alpha1.ConsulBackup) error {
	if backup.Spec.MaxBackups == 0 {
		// In this case, we don't have to rotate a backup file.
		return nil
	}

	switch {
	case backup.Spec.Storage.MinIO != nil:
		spec := backup.Spec.Storage.MinIO

		accessKeySecret, err := b.secretLister.Secrets(backup.Namespace).Get(spec.Credential.AccessKeyID.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}
		accessKey, ok := accessKeySecret.Data[spec.Credential.AccessKeyID.Key]
		if !ok {
			return xerrors.Definef("access key %s not found in %s", spec.Credential.AccessKeyID.Key, accessKeySecret.Name).WithStack()
		}
		secretAccessKeySecret, err := b.secretLister.Secrets(backup.Namespace).Get(spec.Credential.SecretAccessKey.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}
		secretAccessKey, ok := secretAccessKeySecret.Data[spec.Credential.SecretAccessKey.Key]
		if !ok {
			return xerrors.Definef("secret access key %s not found in %s", spec.Credential.AccessKeyID.Key, accessKeySecret.Name).WithStack()
		}

		mcOpt := storage.NewMinIOOptionsViaService(b.coreClient, b.config, spec.Service.Name, spec.Service.Namespace, 9000, string(accessKey), string(secretAccessKey), b.runOutsideCluster)
		mcOpt.Transport = b.transport
		mc := storage.NewMinIOStorage(spec.Bucket, mcOpt)

		files, err := mc.List(ctx, spec.Path)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if len(files) <= backup.Spec.MaxBackups {
			return nil
		}
		filenames := make([]string, 0)
		for _, v := range files {
			filenames = append(filenames, v.Name)
		}
		sort.Strings(filenames)
		sort.Sort(sort.Reverse(sort.StringSlice(filenames)))
		purgeTargets := filenames[backup.Spec.MaxBackups:]
		for _, v := range purgeTargets {
			if err := mc.Delete(ctx, v); err != nil {
				return xerrors.WithStack(err)
			}
		}
	case backup.Spec.Storage.GCS != nil:
		spec := backup.Spec.Storage.GCS
		credential, err := b.secretLister.Secrets(backup.Namespace).Get(spec.Credential.ServiceAccountJSON.Name)
		if err != nil {
			return xerrors.WithStack(err)
		}
		b, ok := credential.Data[spec.Credential.ServiceAccountJSON.Key]
		if !ok {
			return xerrors.Definef("%s is not found in %s", spec.Credential.ServiceAccountJSON.Key, spec.Credential.ServiceAccountJSON.Name).WithStack()
		}

		gcsClient := storage.NewGCS(b, spec.Bucket, storage.GCSOptions{})
		files, err := gcsClient.List(ctx, spec.Path)
		if err != nil {
			return xerrors.WithStack(err)
		}
		if len(files) <= backup.Spec.MaxBackups {
			return nil
		}
		sort.Slice(files, func(i, j int) bool {
			return files[j].Name < files[i].Name
		})
		purgeTargets := files[backup.Spec.MaxBackups:]
		for _, v := range purgeTargets {
			if err := gcsClient.Delete(ctx, v.Name); err != nil {
				return xerrors.WithStack(err)
			}
		}
	}

	return nil
}
