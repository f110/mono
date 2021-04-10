package controllertest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeinformers "k8s.io/client-go/informers"
	corefake "k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"

	"go.f110.dev/mono/go/pkg/k8s/client/versioned/fake"
	"go.f110.dev/mono/go/pkg/k8s/client/versioned/scheme"
	"go.f110.dev/mono/go/pkg/k8s/controllers/controllerutil"
	informers "go.f110.dev/mono/go/pkg/k8s/informers/externalversions"
	"go.f110.dev/mono/go/pkg/logger"
)

type TestRunner struct {
	Client                    *fake.Clientset
	CoreClient                *corefake.Clientset
	SharedInformerFactory     informers.SharedInformerFactory
	CoreSharedInformerFactory kubeinformers.SharedInformerFactory
}

func NewTestRunner() *TestRunner {
	logger.Init()

	client := fake.NewSimpleClientset()
	coreClient := corefake.NewSimpleClientset()

	sharedInformerFactory := informers.NewSharedInformerFactory(client, 30*time.Second)
	coreSharedInformerFactory := kubeinformers.NewSharedInformerFactory(coreClient, 30*time.Second)
	sharedInformerFactory.Harbor().V1alpha1().HarborProjects().Informer().GetIndexer()

	return &TestRunner{
		Client:                    client,
		CoreClient:                coreClient,
		SharedInformerFactory:     sharedInformerFactory,
		CoreSharedInformerFactory: coreSharedInformerFactory,
	}
}

func (r *TestRunner) Reconcile(c controllerutil.Controller, v runtime.Object) error {
	r.RegisterFixture(v)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.Reconcile(ctx, v)
}

func (r *TestRunner) Finalize(c controllerutil.Controller, v runtime.Object) error {
	r.RegisterFixture(v)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.Finalize(ctx, v)
}

func (r *TestRunner) AssertAction(t *testing.T, a Action) bool {
	matchVerb := false
	matchObj := false
Match:
	for _, v := range append(r.Client.Actions(), r.CoreClient.Actions()...) {
		if v.Matches(a.Verb.String(), a.Resource()) {
			matchVerb = true
			switch doneAction := v.(type) {
			case k8stesting.CreateAction:
				doneActionObjMeta, ok := doneAction.GetObject().(metav1.Object)
				if !ok {
					continue
				}
				objMeta, ok := a.Object.(metav1.Object)
				if !ok {
					continue
				}
				if doneActionObjMeta.GetNamespace() == objMeta.GetNamespace() &&
					doneActionObjMeta.GetName() == objMeta.GetName() {
					matchObj = true
					break Match
				}
			case k8stesting.UpdateAction:
				if reflect.DeepEqual(doneAction.GetObject(), a.Object) {
					matchObj = true
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

func (r *TestRunner) RegisterFixture(objs ...runtime.Object) {
	for _, obj := range objs {
		gvks, _, err := kubernetesscheme.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) == 1 {
			r.registerCoreObjectFixture(obj, gvks[0])
			continue
		}

		gvks, _, err = scheme.Scheme.ObjectKinds(obj)
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
	informer, err := r.SharedInformerFactory.ForResource(gvr)
	if err != nil {
		return
	}
	if err := informer.Informer().GetIndexer().Add(obj); err != nil {
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
}

func (a Action) Resource() string {
	if a.Subresource != "" {
		return resourceName(a.Object) + "/" + a.Subresource
	}
	return resourceName(a.Object)
}
