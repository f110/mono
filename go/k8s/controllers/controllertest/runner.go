package controllertest

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeinformers "k8s.io/client-go/informers"
	corefake "k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"

	"go.f110.dev/mono/go/k8s/client"
	"go.f110.dev/mono/go/k8s/client/testingclient"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
)

type TestRunner struct {
	Client                    *testingclient.Set
	CoreClient                *corefake.Clientset
	Factory                   *client.InformerFactory
	CoreSharedInformerFactory kubeinformers.SharedInformerFactory
	Actions                   []*Action
}

func NewTestRunner() *TestRunner {
	logger.Init()

	apiClient := testingclient.NewSet()
	coreClient := corefake.NewSimpleClientset()

	coreSharedInformerFactory := kubeinformers.NewSharedInformerFactory(coreClient, 30*time.Second)

	return &TestRunner{
		Client:                    apiClient,
		CoreClient:                coreClient,
		CoreSharedInformerFactory: coreSharedInformerFactory,
		Factory:                   client.NewInformerFactory(&apiClient.Set, client.NewInformerCache(), metav1.NamespaceAll, 30*time.Second),
	}
}

func (r *TestRunner) Reconcile(c controllerutil.Controller, v runtime.Object) error {
	r.RegisterFixture(v)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if fn, ok := c.(interface {
		NewReconciler(*zap.Logger) controllerutil.Reconciler
	}); ok {
		return fn.NewReconciler(logger.Log).Reconcile(ctx, v)
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
		gvks, _, err := kubernetesscheme.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) == 1 {
			r.registerCoreObjectFixture(obj, gvks[0])
			continue
		}

		gvks, _, err = client.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) == 1 {
			r.registerObjectFixture(obj, gvks[0])
			continue
		}
	}
}

func (r *TestRunner) registerCoreObjectFixture(obj runtime.Object, gvk schema.GroupVersionKind) {
	if err := r.CoreClient.Tracker().Add(obj); err != nil {
		return
	}
	gvr := gvk.GroupVersion().WithResource(resourceName(obj))
	informer, err := r.CoreSharedInformerFactory.ForResource(gvr)
	if err != nil {
		return
	}
	if err := informer.Informer().GetIndexer().Add(obj); err != nil {
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
)

func (a ActionVerb) String() string {
	return string(a)
}

type Action struct {
	Verb        ActionVerb
	Subresource string
	Object      runtime.Object
	Visited     bool
}

func (a Action) Resource() string {
	if a.Subresource != "" {
		return resourceName(a.Object) + "/" + a.Subresource
	}
	return resourceName(a.Object)
}
