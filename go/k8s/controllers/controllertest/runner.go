package controllertest

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.f110.dev/kubeproto/go/k8sclient"
	"go.f110.dev/kubeproto/go/k8stestingclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"

	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/client/testingclient"
	"go.f110.dev/mono/go/k8s/controllers/controllerutil"
	"go.f110.dev/mono/go/k8s/thirdpartyclient"
	"go.f110.dev/mono/go/k8s/thirdpartyclient/testingthirdpartyclient"
	"go.f110.dev/mono/go/logger/slogger"
)

type TestRunner struct {
	Client                    *testingclient.Set
	K8sClient                 *fake.Clientset
	CoreClient                *k8stestingclient.Set
	ThirdPartyClient          *testingthirdpartyclient.Set
	Factory                   *client.InformerFactory
	CoreSharedInformerFactory *k8sclient.InformerFactory
	ThirdPartyInformerFactory *thirdpartyclient.InformerFactory
	Actions                   []*Action
}

func NewTestRunner() *TestRunner {
	slogger.Init()
	slogger.Init()

	apiClient := testingclient.NewSet()
	coreClient := k8stestingclient.NewSet()
	k8sClient := fake.NewClientset()
	tpClient := testingthirdpartyclient.NewSet()

	return &TestRunner{
		Client:                    apiClient,
		K8sClient:                 k8sClient,
		CoreClient:                coreClient,
		ThirdPartyClient:          tpClient,
		Factory:                   client.NewInformerFactory(&apiClient.Set, client.NewInformerCache(), metav1.NamespaceAll, 30*time.Second),
		CoreSharedInformerFactory: k8sclient.NewInformerFactory(&coreClient.Set, k8sclient.NewInformerCache(), metav1.NamespaceAll, 30*time.Second),
		ThirdPartyInformerFactory: thirdpartyclient.NewInformerFactory(&tpClient.Set, thirdpartyclient.NewInformerCache(), metav1.NamespaceAll, 30*time.Second),
	}
}

func (r *TestRunner) Reconcile(c controllerutil.Controller, v runtime.Object) error {
	r.RegisterFixture(v)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if fn, ok := c.(interface {
		NewReconciler(*slog.Logger) controllerutil.Reconciler
	}); ok {
		return fn.NewReconciler(slogger.Log).Reconcile(ctx, v)
	} else {
		return c.Reconcile(ctx, v)
	}
}

func (r *TestRunner) Finalize(c controllerutil.Controller, v runtime.Object) error {
	r.RegisterFixture(v)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if fn, ok := c.(interface {
		NewReconciler() controllerutil.Reconciler
	}); ok {
		return fn.NewReconciler().Finalize(ctx, v)
	} else {
		return c.Finalize(ctx, v)
	}
}

func (r *TestRunner) Reset() {
	r.CoreClient.ClearActions()
	r.Actions = nil
}

func (r *TestRunner) editActions() []*Action {
	if r.Actions != nil {
		return r.Actions
	}

	actions := make([]*Action, 0)
	for _, v := range append(r.Client.Actions(), r.CoreClient.Actions()...) {
		switch a := v.(type) {
		case k8stesting.CreateActionImpl:
			actions = append(actions, &Action{
				Verb:        ActionVerb(v.GetVerb()),
				Subresource: v.GetSubresource(),
				Object:      a.GetObject(),
			})
		case k8stesting.UpdateActionImpl:
			actions = append(actions, &Action{
				Verb:        ActionVerb(v.GetVerb()),
				Subresource: v.GetSubresource(),
				Object:      a.GetObject(),
			})
		case k8stesting.DeleteActionImpl:
			actions = append(actions, &Action{
				Verb:        ActionVerb(v.GetVerb()),
				Subresource: v.GetSubresource(),
			})
		}
	}
	r.Actions = actions

	return actions
}

func (r *TestRunner) AssertCreateAction(t *testing.T, obj runtime.Object) bool {
	t.Helper()

	return r.AssertAction(t, Action{
		Verb:   ActionCreate,
		Object: obj,
	})
}

func (r *TestRunner) AssertUpdateAction(t *testing.T, subresource string, obj runtime.Object) bool {
	t.Helper()

	return r.AssertAction(t, Action{
		Verb:        ActionUpdate,
		Subresource: subresource,
		Object:      obj,
	})
}

func (r *TestRunner) AssertDeleteAction(t *testing.T, obj runtime.Object) bool {
	t.Helper()

	m, ok := obj.(metav1.Object)
	if !ok {
		assert.Failf(t, "Failed type assertion", "%T is not metav1.Object", obj)
	}
	return r.AssertAction(t, Action{
		Verb:      ActionDelete,
		Object:    obj,
		Name:      m.GetName(),
		Namespace: m.GetNamespace(),
	})
}

func (r *TestRunner) AssertAction(t *testing.T, e Action) bool {
	t.Helper()

	matchVerb := false
	matchObj := false
Match:
	for _, v := range r.editActions() {
		if v.Verb == e.Verb {
			matchVerb = true
			switch v.Verb {
			case ActionCreate:
				if reflect.TypeOf(v.Object) != reflect.TypeOf(e.Object) {
					continue
				}
				actualActionObjMeta, ok := v.Object.(metav1.Object)
				if !ok {
					continue
				}
				objMeta, ok := e.Object.(metav1.Object)
				if !ok {
					continue
				}
				if actualActionObjMeta.GetNamespace() == objMeta.GetNamespace() &&
					actualActionObjMeta.GetName() == objMeta.GetName() {
					matchObj = true
					v.Visited = true
					break Match
				}
			case ActionUpdate:
				if reflect.DeepEqual(v.Object, e.Object) {
					matchObj = true
					v.Visited = true
					break Match
				}
			}
		}
	}
	if !matchVerb {
		assert.Fail(t, "The expect action was not called")
	} else if !matchObj {
		assert.Fail(t, "The expect action was called but the matched object was not found")
	}

	return matchVerb && matchObj
}

func (r *TestRunner) AssertNoUnexpectedAction(t *testing.T) {
	unexpectedActions := make([]*Action, 0)
	for _, v := range r.editActions() {
		if v.Visited {
			continue
		}
		unexpectedActions = append(unexpectedActions, v)
	}

	msg := ""
	if len(unexpectedActions) > 0 {
		line := make([]string, 0, len(unexpectedActions))
		for _, v := range unexpectedActions {
			key := ""
			meta, ok := v.Object.(metav1.Object)
			if ok {
				key = fmt.Sprintf(" %s/%s", meta.GetNamespace(), meta.GetName())
			}
			kind := ""
			if v.Object != nil {
				kind = reflect.TypeOf(v.Object).Elem().Name()
			}
			line = append(line, fmt.Sprintf("%s %s%s", v.Verb, kind, key))
		}
		msg = strings.Join(line, " ")
	}

	assert.Len(t, unexpectedActions, 0, "There are %d unexpected actions: %s", len(unexpectedActions), msg)
}

func (r *TestRunner) RegisterFixture(objs ...runtime.Object) {
	for _, obj := range objs {
		gvks, _, err := k8sclient.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) == 1 {
			r.registerCoreObjectFixture(obj, gvks[0])
			continue
		}

		gvks, _, err = client.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) == 1 {
			r.registerObjectFixture(obj, gvks[0])
			continue
		}

		gvks, _, err = thirdpartyclient.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) == 1 {
			r.registerThirdPartyObjectFixture(obj, gvks[0])
			continue
		}
	}
}

func (r *TestRunner) registerCoreObjectFixture(obj runtime.Object, gvk schema.GroupVersionKind) {
	if err := r.CoreClient.Tracker().Add(obj); err != nil {
		return
	}
	gvr := gvk.GroupVersion().WithResource(resourceName(obj))
	informer := r.CoreSharedInformerFactory.InformerForResource(gvr)
	if informer == nil {
		return
	}
	if err := informer.GetIndexer().Add(obj); err != nil {
		return
	}
}

func (r *TestRunner) registerObjectFixture(obj runtime.Object, gvk schema.GroupVersionKind) {
	if err := r.Client.Tracker().Add(obj); err != nil {
		return
	}
	gvr := gvk.GroupVersion().WithResource(resourceName(obj))
	informer := r.Factory.InformerForResource(gvr)
	if informer == nil {
		return
	}
	if err := informer.GetIndexer().Add(obj); err != nil {
		return
	}
}

func (r *TestRunner) registerThirdPartyObjectFixture(obj runtime.Object, gvk schema.GroupVersionKind) {
	if err := r.ThirdPartyClient.Tracker().Add(obj); err != nil {
		return
	}
	gvr := gvk.GroupVersion().WithResource(resourceName(obj))
	informer := r.ThirdPartyInformerFactory.InformerForResource(gvr)
	if informer == nil {
		return
	}
	if err := informer.GetIndexer().Add(obj); err != nil {
		return
	}
}

type GenericTestRunner[T runtime.Object] struct {
	*TestRunner
}

func NewGenericTestRunner[T runtime.Object]() *GenericTestRunner[T] {
	return &GenericTestRunner[T]{
		TestRunner: NewTestRunner(),
	}
}

func (r *GenericTestRunner[T]) Reconcile(c controllerutil.GenericReconciler[T], v T) error {
	r.RegisterFixture(v)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.Reconcile(ctx, v)
}

func (r *GenericTestRunner[T]) Finalize(c controllerutil.GenericReconciler[T], v T) error {
	r.RegisterFixture(v)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.Finalize(ctx, v)
}

func resourceName(v runtime.Object) string {
	t := reflect.TypeOf(v)
	kind := t.Elem().Name()

	plural := namer.NewAllLowercasePluralNamer(nil)
	return plural.Name(&types.Type{
		Name: types.Name{
			Name: kind,
		},
	})
}

type ActionVerb string

const (
	ActionUpdate ActionVerb = "update"
	ActionCreate ActionVerb = "create"
	ActionDelete ActionVerb = "delete"
)

func (a ActionVerb) String() string {
	return string(a)
}

type Action struct {
	Verb        ActionVerb
	Subresource string
	Object      runtime.Object
	Name        string
	Namespace   string
	Visited     bool
}

func (a Action) Resource() string {
	if a.Subresource != "" {
		return resourceName(a.Object) + "/" + a.Subresource
	}
	return resourceName(a.Object)
}
