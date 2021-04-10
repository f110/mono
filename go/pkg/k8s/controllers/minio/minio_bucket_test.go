package minio

import (
	"log"
	"net/http"
	"regexp"
	"runtime/debug"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/minio/minio-go/v6"
	miniocontrollerv1beta1 "github.com/minio/minio-operator/pkg/apis/miniocontroller/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	miniov1alpha1 "go.f110.dev/mono/go/pkg/api/minio/v1alpha1"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllertest"
)

func TestBucketController(t *testing.T) {
	t.Run("CreateBucket", func(t *testing.T) {
		runner := newRunner()
		controller := newBucketController(t, runner)
		target, fixtures := minioFixture()
		runner.RegisterFixture(fixtures...)

		mockTransport := httpmock.NewMockTransport()
		controller.transport = mockTransport
		mockTransport.RegisterRegexpResponder(
			http.MethodGet,
			regexp.MustCompile(".*/bucket1/"),
			httpmock.NewStringResponder(
				http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?>
<LocationConstraint>
   <LocationConstraint>us-east-1</LocationConstraint>
</LocationConstraint>`,
			),
		)
		mockTransport.RegisterRegexpResponder(
			http.MethodHead,
			regexp.MustCompile(".*/bucket1/$"),
			httpmock.NewXmlResponderOrPanic(http.StatusNotFound, &minio.ErrorResponse{Code: "NoSuchBucket"}),
		)
		mockTransport.RegisterRegexpResponder(
			http.MethodPut,
			regexp.MustCompile(".*/bucket1/$"),
			httpmock.NewStringResponder(http.StatusOK, ""),
		)
		mockTransport.RegisterRegexpResponder(
			http.MethodDelete,
			regexp.MustCompile(".*/bucket1/\\?policy=$"),
			httpmock.NewStringResponder(http.StatusOK, ""),
		)
		mockTransport.RegisterRegexpResponder(
			http.MethodDelete,
			regexp.MustCompile(".*"),
			func(request *http.Request) (*http.Response, error) {
				log.Print(request.URL.String())
				debug.PrintStack()
				return nil, nil
			},
		)

		err := runner.Reconcile(controller, target)
		require.NoError(t, err)

		updated := target.DeepCopy()
		updated.Status.Ready = true
		runner.AssertAction(t, controllertest.Action{
			Verb:        controllertest.ActionUpdate,
			Subresource: "status",
			Object:      updated,
		})
		runner.AssertNoUnexpectedAction(t)
	})
}

func minioFixture() (*miniov1alpha1.MinIOBucket, []runtime.Object) {
	instance := &miniocontrollerv1beta1.MinIOInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance",
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: miniocontrollerv1beta1.MinIOInstanceSpec{
			CredsSecret: &corev1.LocalObjectReference{
				Name: "creds",
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "creds",
			Namespace: metav1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"accesskey": []byte("foo"),
			"secretkey": []byte("bar"),
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

	target := &miniov1alpha1.MinIOBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bucket1",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: miniov1alpha1.MinIOBucketSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: instance.ObjectMeta.Labels,
			},
			CreateIndexFile: false,
		},
	}

	return target, []runtime.Object{instance, secret, service}
}
