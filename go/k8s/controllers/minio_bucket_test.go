package controllers

import (
	"log"
	"net/http"
	"regexp"
	"runtime/debug"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/minio/minio-go/v6"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/miniov1alpha1"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
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
			regexp.MustCompile("./bucket1/\\?policy="),
			httpmock.NewStringResponder(http.StatusOK, ""),
		)
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
			func(req *http.Request) (*http.Response, error) {
				return httpmock.NewStringResponse(http.StatusOK, ""), nil
			},
		)
		mockTransport.RegisterRegexpResponder(
			http.MethodPut,
			regexp.MustCompile(".*/bucket1/\\?policy=$"),
			httpmock.NewStringResponder(http.StatusNoContent, ""),
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
	secret := k8sfactory.SecretFactory(nil,
		k8sfactory.Name("creds"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Data("accesskey", []byte("foo")),
		k8sfactory.Data("secretkey", []byte("bar")),
	)
	instance := k8sfactory.MinIOInstanceFactory(nil,
		k8sfactory.Name("instance"),
		k8sfactory.DefaultNamespace,
		k8sfactory.Labels(map[string]string{"app": "test"}),
		k8sfactory.MinIOCredential(k8sfactory.LocalObjectReference(secret)),
	)
	service := k8sfactory.ServiceFactory(nil,
		k8sfactory.Namef("%s-hl-svc", instance.Name),
		k8sfactory.DefaultNamespace,
		k8sfactory.Port("", corev1.ProtocolTCP, 9000),
	)

	target := k8sfactory.MinIOBucketFactory(nil,
		k8sfactory.Name("bucket1"),
		k8sfactory.DefaultNamespace,
		k8sfactory.MinIOSelector(k8sfactory.MatchLabel(instance.ObjectMeta.Labels)),
		k8sfactory.DisableCreatingIndexFile,
	)

	return target, []runtime.Object{instance, secret, service}
}
