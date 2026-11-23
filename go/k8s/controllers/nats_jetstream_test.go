package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.f110.dev/kubeproto/go/apis/metav1"

	"go.f110.dev/mono/go/api/natsv1alpha1"
	"go.f110.dev/mono/go/k8s/controllers/controllertest"
	"go.f110.dev/mono/go/k8s/k8sfactory"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestNATSJetStreamController(t *testing.T) {
	newTarget := func(traits ...k8sfactory.Trait) *natsv1alpha1.JetStream {
		base := k8sfactory.JetStreamFactory(nil, k8sfactory.Name("test"), k8sfactory.DefaultNamespace, k8sfactory.Generation(1))
		base.Spec = natsv1alpha1.JetStreamSpec{
			Subjects:  []string{"test.*"},
			Retention: natsv1alpha1.RetentionPolicyInterest,
			Storage:   natsv1alpha1.StorageTypeMemory,
			Replicas:  3,
			MaxAge:    &metav1.Duration{Duration: time.Hour},
		}
		return k8sfactory.JetStreamFactory(base, traits...)
	}
	ready := func(generation int64) k8sfactory.Trait {
		return func(object any) {
			obj := object.(*natsv1alpha1.JetStream)
			obj.Status = natsv1alpha1.JetStreamStatus{Phase: natsv1alpha1.JetStreamPhaseReady, ObservedGeneration: generation}
		}
	}

	t.Run("CreateStream", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*natsv1alpha1.JetStream]()
		js := newFakeStreamManager()
		controller := newNATSJetStreamController(runner.K8sClient, &runner.Client.Set, runner.Factory, js)
		target := newTarget()

		err := runner.Reconcile(controller.newReconciler(), target)
		assertion.MustNoError(t, err)

		assertion.Equal(t, js.created, []jetstream.StreamConfig{
			{
				Name:      "test",
				Subjects:  []string{"test.*"},
				Retention: jetstream.InterestPolicy,
				Storage:   jetstream.MemoryStorage,
				Replicas:  3,
				MaxAge:    time.Hour,
			},
		})
		assertion.Len(t, js.updated, 0)
		updated, err := runner.Client.NatsV1alpha1.GetJetStream(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		assertion.MustNoError(t, err)
		assertion.Equal(t, updated.Status, natsv1alpha1.JetStreamStatus{Phase: natsv1alpha1.JetStreamPhaseReady, ObservedGeneration: 1})
	})

	t.Run("UpdateStreamWhenGenerationIsChanged", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*natsv1alpha1.JetStream]()
		js := newFakeStreamManager("test")
		controller := newNATSJetStreamController(runner.K8sClient, &runner.Client.Set, runner.Factory, js)
		target := newTarget(k8sfactory.Generation(2), ready(1))

		err := runner.Reconcile(controller.newReconciler(), target)
		assertion.MustNoError(t, err)

		assertion.Len(t, js.created, 0)
		assertion.MustLen(t, js.updated, 1)
		assertion.Equal(t, js.updated[0].Name, "test")
		updated, err := runner.Client.NatsV1alpha1.GetJetStream(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		assertion.MustNoError(t, err)
		assertion.Equal(t, updated.Status, natsv1alpha1.JetStreamStatus{Phase: natsv1alpha1.JetStreamPhaseReady, ObservedGeneration: 2})
	})

	t.Run("NotUpdateStreamWhenGenerationIsSame", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*natsv1alpha1.JetStream]()
		js := newFakeStreamManager("test")
		controller := newNATSJetStreamController(runner.K8sClient, &runner.Client.Set, runner.Factory, js)
		target := newTarget(ready(1))

		err := runner.Reconcile(controller.newReconciler(), target)
		assertion.MustNoError(t, err)

		assertion.Len(t, js.created, 0)
		assertion.Len(t, js.updated, 0)
		runner.AssertNoUnexpectedAction(t)
	})

	t.Run("Failed", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*natsv1alpha1.JetStream]()
		js := newFakeStreamManager()
		js.err = errors.New("replicas > 1 not supported in non-clustered mode")
		controller := newNATSJetStreamController(runner.K8sClient, &runner.Client.Set, runner.Factory, js)
		target := newTarget()

		err := runner.Reconcile(controller.newReconciler(), target)
		assertion.MustError(t, err)

		updated, err := runner.Client.NatsV1alpha1.GetJetStream(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		assertion.MustNoError(t, err)
		assertion.Equal(t, updated.Status, natsv1alpha1.JetStreamStatus{
			Phase:              natsv1alpha1.JetStreamPhaseFailed,
			ObservedGeneration: 1,
			Message:            "replicas > 1 not supported in non-clustered mode",
		})
	})

	t.Run("RecoverFromFailed", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*natsv1alpha1.JetStream]()
		js := newFakeStreamManager("test")
		controller := newNATSJetStreamController(runner.K8sClient, &runner.Client.Set, runner.Factory, js)
		target := newTarget(func(object any) {
			obj := object.(*natsv1alpha1.JetStream)
			obj.Status = natsv1alpha1.JetStreamStatus{Phase: natsv1alpha1.JetStreamPhaseFailed, ObservedGeneration: 1, Message: "error"}
		})

		err := runner.Reconcile(controller.newReconciler(), target)
		assertion.MustNoError(t, err)

		assertion.Len(t, js.updated, 1)
		updated, err := runner.Client.NatsV1alpha1.GetJetStream(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		assertion.MustNoError(t, err)
		assertion.Equal(t, updated.Status, natsv1alpha1.JetStreamStatus{Phase: natsv1alpha1.JetStreamPhaseReady, ObservedGeneration: 1})
	})

	t.Run("FinalizeKeep", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*natsv1alpha1.JetStream]()
		js := newFakeStreamManager("test")
		controller := newNATSJetStreamController(runner.K8sClient, &runner.Client.Set, runner.Factory, js)
		target := newTarget(k8sfactory.Delete, k8sfactory.Finalizer(natsJetStreamControllerFinalizerName))

		err := runner.Finalize(controller.newReconciler(), target)
		assertion.MustNoError(t, err)

		assertion.Len(t, js.deleted, 0)
		updated, err := runner.Client.NatsV1alpha1.GetJetStream(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		assertion.MustNoError(t, err)
		assertion.Len(t, updated.Finalizers, 0)
	})

	t.Run("FinalizeDelete", func(t *testing.T) {
		runner := controllertest.NewGenericTestRunner[*natsv1alpha1.JetStream]()
		js := newFakeStreamManager("test")
		controller := newNATSJetStreamController(runner.K8sClient, &runner.Client.Set, runner.Factory, js)
		target := newTarget(k8sfactory.Delete, k8sfactory.Finalizer(natsJetStreamControllerFinalizerName), func(object any) {
			object.(*natsv1alpha1.JetStream).Spec.FinalizePolicy = natsv1alpha1.FinalizePolicyDelete
		})

		err := runner.Finalize(controller.newReconciler(), target)
		assertion.MustNoError(t, err)

		assertion.Equal(t, js.deleted, []string{"test"})
		updated, err := runner.Client.NatsV1alpha1.GetJetStream(context.Background(), target.Namespace, target.Name, metav1.GetOptions{})
		assertion.MustNoError(t, err)
		assertion.Len(t, updated.Finalizers, 0)
	})
}

type fakeStreamManager struct {
	jetstream.StreamManager

	streams map[string]struct{}
	err     error
	created []jetstream.StreamConfig
	updated []jetstream.StreamConfig
	deleted []string
}

func newFakeStreamManager(streams ...string) *fakeStreamManager {
	s := make(map[string]struct{})
	for _, v := range streams {
		s[v] = struct{}{}
	}
	return &fakeStreamManager{streams: s}
}

func (f *fakeStreamManager) Stream(_ context.Context, name string) (jetstream.Stream, error) {
	if _, ok := f.streams[name]; !ok {
		return nil, jetstream.ErrStreamNotFound
	}
	return nil, nil
}

func (f *fakeStreamManager) CreateStream(_ context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created = append(f.created, cfg)
	f.streams[cfg.Name] = struct{}{}
	return nil, nil
}

func (f *fakeStreamManager) UpdateStream(_ context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.updated = append(f.updated, cfg)
	return nil, nil
}

func (f *fakeStreamManager) DeleteStream(_ context.Context, name string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, name)
	delete(f.streams, name)
	return nil
}
