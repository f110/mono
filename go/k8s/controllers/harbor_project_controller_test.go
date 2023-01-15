package controllers

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"go.f110.dev/mono/go/api/harborv1alpha1"
	"go.f110.dev/mono/go/harbor"
	"go.f110.dev/mono/go/k8s/k8sfactory"
)

func TestHarborProjectController(t *testing.T) {
	runner, controller := newHarborProjectController(t)
	target, fixtures := newHarborProjectFixture()
	runner.RegisterFixture(fixtures...)

	mockTransport := httpmock.NewMockTransport()
	controller.transport = mockTransport
	mockTransport.RegisterRegexpResponder(
		http.MethodHead,
		regexp.MustCompile(`.+/api/v2.0/projects.+`),
		httpmock.NewStringResponder(http.StatusNotFound, ""),
	)
	mockTransport.RegisterRegexpResponder(
		http.MethodPost,
		regexp.MustCompile(`.+/api/v2.0/projects$`),
		httpmock.NewStringResponder(http.StatusCreated, ""),
	)
	mockTransport.RegisterRegexpResponder(
		http.MethodGet,
		regexp.MustCompile(`.+/api/v2.0/projects$`),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, []harbor.Project{
			{Id: 1, Name: target.Name},
		}),
	)

	err := runner.Reconcile(controller, target)
	require.NoError(t, err)

	expect := k8sfactory.HarborProjectFactory(target,
		k8sfactory.ReadyProject(1),
	)
	expect.Status.Registry = "test-registry.f110.dev"
	runner.AssertUpdateAction(t, "status", expect)
	runner.AssertNoUnexpectedAction(t)
}

func newHarborProjectFixture() (*harborv1alpha1.HarborProject, []runtime.Object) {
	target := k8sfactory.HarborProjectFactory(nil,
		k8sfactory.Name("test1"),
		k8sfactory.DefaultNamespace,
	)

	return target, []runtime.Object{}
}
