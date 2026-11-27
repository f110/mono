package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.f110.dev/kubeproto/go/apis/metav1"
	"go.f110.dev/xerrors"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"go.f110.dev/mono/go/api/natsv1alpha1"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
)

const (
	natsJetStreamControllerFinalizerName = "nats-jetstream-controller.nats.f110.dev/finalizer"
)

type NATSJetStreamController struct {
	*controllerutil.GenericControllerBase[*natsv1alpha1.JetStream]

	client *client.NatsV1alpha1
	conn   *nats.Conn
	js     jetstream.StreamManager
}

func NewNATSJetStreamController(
	k8sClient kubernetes.Interface,
	apiClient *client.Set,
	factory *client.InformerFactory,
	natsURL string,
) (*NATSJetStreamController, error) {
	if natsURL == "" {
		return nil, xerrors.Define("the url of NATS is not specified").WithStack()
	}
	conn, err := nats.Connect(natsURL)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, xerrors.WithStack(err)
	}

	c := newNATSJetStreamController(k8sClient, apiClient, factory, js)
	c.conn = conn
	return c, nil
}

func newNATSJetStreamController(
	k8sClient kubernetes.Interface,
	apiClient *client.Set,
	factory *client.InformerFactory,
	js jetstream.StreamManager,
) *NATSJetStreamController {
	natsInformers := client.NewNatsV1alpha1Informer(factory.Cache(), apiClient.NatsV1alpha1, metav1.NamespaceAll, 30*time.Second)

	c := &NATSJetStreamController{
		client: apiClient.NatsV1alpha1,
		js:     js,
	}
	c.GenericControllerBase = controllerutil.NewGenericControllerBase[*natsv1alpha1.JetStream](
		"nats-jetstream-controller",
		c.newReconciler,
		k8sClient,
		[]cache.SharedIndexInformer{natsInformers.JetStreamInformer()},
		nil,
		[]string{natsJetStreamControllerFinalizerName},
		natsInformers.JetStreamLister().Get,
		apiClient.NatsV1alpha1.PatchJetStream,
	)
	return c
}

func (c *NATSJetStreamController) Shutdown() {
	c.GenericControllerBase.Shutdown()
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *NATSJetStreamController) newReconciler() controllerutil.GenericReconciler[*natsv1alpha1.JetStream] {
	return &natsJetStreamReconciler{client: c.client, js: c.js}
}

// ValidateJetStream denies the update of the fields which NATS can't change.
func ValidateJetStream(req *admissionv1.AdmissionRequest) error {
	if req.Operation != admissionv1.Update || req.SubResource != "" {
		return nil
	}

	obj := &natsv1alpha1.JetStream{}
	if err := json.Unmarshal(req.Object.Raw, obj); err != nil {
		return xerrors.WithStack(err)
	}
	old := &natsv1alpha1.JetStream{}
	if err := json.Unmarshal(req.OldObject.Raw, old); err != nil {
		return xerrors.WithStack(err)
	}

	if obj.Spec.Retention != old.Spec.Retention {
		return xerrors.Define("retention can't be changed").WithStack()
	}
	if obj.Spec.Storage != old.Spec.Storage {
		return xerrors.Define("storage can't be changed").WithStack()
	}
	if obj.Spec.MaxConsumers != old.Spec.MaxConsumers {
		return xerrors.Define("max_consumers can't be changed").WithStack()
	}
	return nil
}

type natsJetStreamReconciler struct {
	client *client.NatsV1alpha1
	js     jetstream.StreamManager
}

var _ controllerutil.GenericReconciler[*natsv1alpha1.JetStream] = (*natsJetStreamReconciler)(nil)

func (r *natsJetStreamReconciler) Reconcile(ctx context.Context, obj *natsv1alpha1.JetStream) error {
	cfg := streamConfig(obj)
	_, err := r.js.Stream(ctx, cfg.Name)
	switch {
	case errors.Is(err, jetstream.ErrStreamNotFound):
		_, err = r.js.CreateStream(ctx, cfg)
	case err != nil:
	case obj.Status.Phase != natsv1alpha1.JetStreamPhaseReady || obj.Status.ObservedGeneration != obj.Generation:
		_, err = r.js.UpdateStream(ctx, cfg)
	}

	updated := obj.DeepCopy()
	updated.Status.ObservedGeneration = obj.Generation
	if err != nil {
		updated.Status.Phase = natsv1alpha1.JetStreamPhaseFailed
		updated.Status.Message = err.Error()
	} else {
		updated.Status.Phase = natsv1alpha1.JetStreamPhaseReady
		updated.Status.Message = ""
	}
	if !reflect.DeepEqual(updated.Status, obj.Status) {
		if _, err := r.client.UpdateStatusJetStream(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return controllerutil.WrapRetryError(xerrors.WithStack(err))
		}
	}
	if err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func (r *natsJetStreamReconciler) Finalize(ctx context.Context, obj *natsv1alpha1.JetStream) error {
	if obj.Spec.FinalizePolicy == natsv1alpha1.FinalizePolicyDelete {
		if err := r.js.DeleteStream(ctx, obj.Name); err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
			return xerrors.WithStack(err)
		}
	}

	obj.Finalizers = enumerable.Delete(obj.Finalizers, natsJetStreamControllerFinalizerName)
	if _, err := r.client.UpdateJetStream(ctx, obj, metav1.UpdateOptions{}); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// streamConfig builds the configuration of the stream from the spec.
// The zero values are sent as they are, and NATS fills the default values.
func streamConfig(obj *natsv1alpha1.JetStream) jetstream.StreamConfig {
	cfg := jetstream.StreamConfig{
		Name:         obj.Name,
		Description:  obj.Spec.Description,
		Subjects:     obj.Spec.Subjects,
		Replicas:     obj.Spec.Replicas,
		MaxConsumers: obj.Spec.MaxConsumers,
		MaxMsgs:      obj.Spec.MaxMsgs,
		MaxBytes:     obj.Spec.MaxBytes,
	}
	switch obj.Spec.Retention {
	case natsv1alpha1.RetentionPolicyInterest:
		cfg.Retention = jetstream.InterestPolicy
	case natsv1alpha1.RetentionPolicyWorkqueue:
		cfg.Retention = jetstream.WorkQueuePolicy
	default:
		cfg.Retention = jetstream.LimitsPolicy
	}
	switch obj.Spec.Storage {
	case natsv1alpha1.StorageTypeMemory:
		cfg.Storage = jetstream.MemoryStorage
	default:
		cfg.Storage = jetstream.FileStorage
	}
	if obj.Spec.MaxAge != nil {
		cfg.MaxAge = obj.Spec.MaxAge.Duration
	}
	return cfg
}
