package controllers

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/consulv1alpha1"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/storage/storagetest"
)

func TestConsulBackupController_Reconcile(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		runner, controller := newConsulBackupController(t)
		target, fixtures := consulBackupControllerFixture()
		runner.RegisterFixture(fixtures...)
		target = k8sfactory.ConsulBackupFactory(target,
			k8sfactory.BackupSucceeded(time.Now().Add(-time.Duration(target.Spec.IntervalInSeconds+1)*time.Second)),
		)

		mockTransport := httpmock.NewMockTransport()
		controller.transport = mockTransport
		mockTransport.RegisterResponder(
			http.MethodGet,
			"http://consul-server.default.svc:8500/v1/snapshot",
			httpmock.NewStringResponder(http.StatusOK, "backup_data"),
		)
		mockMinio := storagetest.NewMockMinIO()
		mockMinio.AddBucket("backup")
		mockMinio.Transport(mockTransport)

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)

		expect, err := runner.Client.ConsulV1alpha1.GetConsulBackup(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		require.NoError(t, err)
		runner.AssertAction(t, controllertest.Action{
			Verb:        controllertest.ActionUpdate,
			Subresource: "status",
			Object:      expect,
		})
		runner.AssertNoUnexpectedAction(t)
		assert.True(t, expect.Status.Succeeded)
		assert.Equal(t, expect.Status.LastSucceededTime, expect.Status.BackupStatusHistory[0].ExecuteTime)
		assert.Equal(t, fmt.Sprintf("%s_%d", target.Name, expect.Status.LastSucceededTime.Unix()), expect.Status.BackupStatusHistory[0].Path)
	})

	t.Run("WithInInterval", func(t *testing.T) {
		runner, controller := newConsulBackupController(t)
		target, fixtures := consulBackupControllerFixture()
		runner.RegisterFixture(fixtures...)
		target = k8sfactory.ConsulBackupFactory(target,
			k8sfactory.BackupSucceeded(time.Now().Add(-time.Duration(target.Spec.IntervalInSeconds-1)*time.Second)),
		)

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)
	})

	t.Run("RotateHistory", func(t *testing.T) {
		runner, controller := newConsulBackupController(t)
		target, fixtures := consulBackupControllerFixture()
		runner.RegisterFixture(fixtures...)
		target.Status.BackupStatusHistory = append(target.Status.BackupStatusHistory,
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
			"http://consul-server.default.svc:8500/v1/snapshot",
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

		require.Len(t, mockMinio.Removed("backup"), 1)
		assert.ElementsMatch(t, []string{"/test_1"}, mockMinio.Removed("backup"))
		expect, err := runner.Client.ConsulV1alpha1.GetConsulBackup(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, expect.Status.BackupStatusHistory, expect.Spec.MaxBackups)
	})
}

func TestConsulBackupController_ObjectToKeys(t *testing.T) {
	_, controller := newConsulBackupController(t)

	keys := controller.ObjectToKeys(&consulv1alpha1.ConsulBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "default/test", keys[0])
}

func consulBackupControllerFixture() (*consulv1alpha1.ConsulBackup, []runtime.Object) {
	secret := k8sfactory.SecretFactory(nil,
		k8sfactory.Name("access"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Data("accesskey", []byte("test-accesskey")),
		k8sfactory.Data("secret", []byte("test-secret-access-key")),
	)
	service := k8sfactory.ServiceFactory(nil,
		k8sfactory.Name("minio"),
		k8sfactory.DefaultNamespace,
	)
	target := k8sfactory.ConsulBackupFactory(nil,
		k8sfactory.Name("test"),
		k8sfactory.DefaultNamespace,
		k8sfactory.MaxBackup(5),
		k8sfactory.BackupInterval(600),
		k8sfactory.ServiceReference(corev1.LocalObjectReference{Name: "consul-server"}),
		k8sfactory.BackupStorage(
			k8sfactory.BackupMinIOStorageFactory(nil,
				k8sfactory.Bucket("backup"),
				k8sfactory.StoragePath("/"),
				k8sfactory.BackupService(k8sfactory.ObjectReference(service)),
				k8sfactory.AWSCredential(
					k8sfactory.AWSCredentialFactory(nil,
						k8sfactory.AccessKey(k8sfactory.SecretKeySelector(secret, "accesskey")),
						k8sfactory.SecretAccessKey(k8sfactory.SecretKeySelector(secret, "secret")),
					),
				),
			),
		),
	)

	return target, []runtime.Object{secret, service}
}

func newConsulBackupController(t *testing.T) (*controllertest.TestRunner, *ConsulBackupController) {
	runner := controllertest.NewTestRunner()
	controller, err := NewConsulBackupController(
		runner.CoreSharedInformerFactory,
		runner.Factory,
		runner.CoreClient,
		&runner.Client.Set,
		nil,
		false,
	)
	require.NoError(t, err)

	return runner, controller
}
