package consul

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	consulv1alpha1 "go.f110.dev/mono/go/pkg/api/consul/v1alpha1"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/pkg/storage/storagetest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestBackupController_Reconcile(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		runner, controller := newController(t)
		target, fixtures := fixture()
		runner.RegisterFixture(fixtures...)
		lastSucceededTime := time.Now().Add(-time.Duration(target.Spec.IntervalInSecond+1) * time.Second)
		target.Status.Succeeded = true
		target.Status.LastSucceededTime = &metav1.Time{Time: lastSucceededTime}

		mockTransport := httpmock.NewMockTransport()
		controller.transport = mockTransport
		mockTransport.RegisterResponder(
			http.MethodGet,
			"http://127.0.0.1:8500/v1/snapshot",
			httpmock.NewStringResponder(http.StatusOK, "backup_data"),
		)
		mockMinio := storagetest.NewMockMinIO()
		mockMinio.AddBucket("backup")
		mockMinio.Transport(mockTransport)

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)

		expect, err := runner.Client.ConsulV1alpha1().ConsulBackups(target.Namespace).Get(context.TODO(), target.Name, metav1.GetOptions{})
		require.NoError(t, err)
		runner.AssertAction(t, controllertest.Action{
			Verb:        controllertest.ActionUpdate,
			Subresource: "status",
			Object:      expect,
		})
		runner.AssertNoUnexpectedAction(t)
		assert.True(t, expect.Status.Succeeded)
		assert.Equal(t, expect.Status.LastSucceededTime, expect.Status.History[0].ExecuteTime)
		assert.Equal(t, fmt.Sprintf("%s_%d", target.Name, expect.Status.LastSucceededTime.Unix()), expect.Status.History[0].Path)
	})

	t.Run("WithInInterval", func(t *testing.T) {
		runner, controller := newController(t)
		target, fixtures := fixture()
		runner.RegisterFixture(fixtures...)
		lastSucceededTime := time.Now().Add(-time.Duration(target.Spec.IntervalInSecond-1) * time.Second)
		target.Status.Succeeded = true
		target.Status.LastSucceededTime = &metav1.Time{Time: lastSucceededTime}

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)
	})

	t.Run("RotateHistory", func(t *testing.T) {
		runner, controller := newController(t)
		target, fixtures := fixture()
		runner.RegisterFixture(fixtures...)
		target.Status.History = append(target.Status.History,
			consulv1alpha1.ConsulBackupStatusHistory{Path: "/test_1", Succeeded: true},
			consulv1alpha1.ConsulBackupStatusHistory{Path: "/test_2", Succeeded: true},
			consulv1alpha1.ConsulBackupStatusHistory{Path: "/test_3", Succeeded: true},
			consulv1alpha1.ConsulBackupStatusHistory{Path: "/test_4", Succeeded: true},
			consulv1alpha1.ConsulBackupStatusHistory{Path: "/test_5", Succeeded: true},
			consulv1alpha1.ConsulBackupStatusHistory{Path: "/test_6", Succeeded: true},
		)

		mockTransport := httpmock.NewMockTransport()
		controller.transport = mockTransport
		mockTransport.RegisterResponder(
			http.MethodGet,
			"http://127.0.0.1:8500/v1/snapshot",
			httpmock.NewStringResponder(http.StatusOK, "backup_data"),
		)
		mockMinio := storagetest.NewMockMinIO()
		mockMinio.AddBucket("backup")
		mockMinio.AddObjects("backup",
			&minio.ObjectInfo{Key: "/test_1"},
			&minio.ObjectInfo{Key: "/test_2"},
			&minio.ObjectInfo{Key: "/test_3"},
			&minio.ObjectInfo{Key: "/test_4"},
			&minio.ObjectInfo{Key: "/test_5"},
			&minio.ObjectInfo{Key: "/test_6"},
		)
		mockMinio.Transport(mockTransport)

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)

		require.Len(t, mockMinio.Removed("backup"), 2)
		assert.ElementsMatch(t, []string{"/test_1", "/test_2"}, mockMinio.Removed("backup"))
		expect, err := runner.Client.ConsulV1alpha1().ConsulBackups(target.Namespace).Get(context.TODO(), target.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, expect.Status.History, expect.Spec.MaxBackups)
	})
}

func TestBackupController_ObjectToKeys(t *testing.T) {
	_, controller := newController(t)

	keys := controller.ObjectToKeys(&consulv1alpha1.ConsulBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "default/test", keys[0])
}

func fixture() (*consulv1alpha1.ConsulBackup, []runtime.Object) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "access",
			Namespace: metav1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"accesskey": []byte("test-accesskey"),
			"secret":    []byte("test-secret-access-key"),
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio",
			Namespace: metav1.NamespaceDefault,
		},
	}
	target := &consulv1alpha1.ConsulBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: consulv1alpha1.ConsulBackupSpec{
			MaxBackups:       5,
			IntervalInSecond: 600,
			Storage: consulv1alpha1.ConsulBackupStorageSpec{
				MinIO: &consulv1alpha1.BackupStorageMinIOSpec{
					Bucket: "backup",
					Path:   "/",
					Service: &consulv1alpha1.ObjectReference{
						Name:      service.Name,
						Namespace: service.Namespace,
					},
					Credential: consulv1alpha1.AWSCredential{
						AccessKeyID: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secret.Name,
							},
							Key: "accesskey",
						},
						SecretAccessKey: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secret.Name,
							},
							Key: "secret",
						},
					},
				},
			},
		},
	}

	return target, []runtime.Object{secret, service}
}

func newController(t *testing.T) (*controllertest.TestRunner, *BackupController) {
	runner := controllertest.NewTestRunner()
	controller, err := NewBackupController(
		runner.CoreSharedInformerFactory,
		runner.SharedInformerFactory,
		runner.CoreClient,
		runner.Client,
		nil,
		false,
	)
	require.NoError(t, err)

	return runner, controller
}
