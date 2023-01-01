package minio

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllertest"
)

func TestUserController(t *testing.T) {
	t.Run("CreateSecret", func(t *testing.T) {
		runner := newRunner()
		controller, mockTransport := newUserController(t, runner)
		mockTransport.RegisterResponder(
			http.MethodPut,
			"/minio/admin/v3/add-user",
			httpmock.NewStringResponder(
				http.StatusOK,
				"",
			),
		)
		mockTransport.RegisterResponder(
			http.MethodPut,
			"/v1/secret/data/test",
			httpmock.NewStringResponder(
				http.StatusOK,
				"",
			),
		)
		target, fixtures := minIOUserFixture()
		runner.RegisterFixture(fixtures...)

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)

		runner.AssertAction(t, controllertest.Action{
			Verb: controllertest.ActionCreate,
			Object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-accesskey", target.Name),
					Namespace: target.Namespace,
				},
			},
		})
		runner.AssertNoUnexpectedAction(t)
	})
}

func minIOUserFixture() (*miniov1alpha1.MinIOUser, []runtime.Object) {
	user := &miniov1alpha1.MinIOUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: miniov1alpha1.MinIOUserSpec{
			Path:      "/test",
			MountPath: "/secret",
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "minio",
				},
			},
		},
	}

	instance := &miniocontrollerv1beta1.MinIOInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				"app": "minio",
			},
		},
		Spec: miniocontrollerv1beta1.MinIOInstanceSpec{
			CredsSecret: &corev1.LocalObjectReference{
				Name: "root-accesskey",
			},
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-hl-svc",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: 9000},
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "root-accesskey",
			Namespace: metav1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"accesskey": []byte("rootaccesskey"),
			"secretkey": []byte("rootsecretkey"),
		},
	}

	return user, []runtime.Object{instance, service, secret}
}
