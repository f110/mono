package controllerutil

import (
	"context"

	"k8s.io/client-go/tools/cache"
)

type ControllerBase struct {
	informers []cache.SharedIndexInformer
}

func NewBase() *ControllerBase {
	return &ControllerBase{}
}

func (b *ControllerBase) UseInformer(v cache.SharedIndexInformer) {
	b.informers = append(b.informers, v)
}

func (b *ControllerBase) Run(ctx context.Context) {
	hasSynced := make([]cache.InformerSynced, len(b.informers))
	for i := range b.informers {
		hasSynced[i] = b.informers[i].HasSynced
	}

	if !cache.WaitForCacheSync(ctx.Done(), hasSynced...) {
		return
	}
}

func (b *ControllerBase) WaitForSync(ctx context.Context) bool {
	hasSynced := make([]cache.InformerSynced, len(b.informers))
	for i := range b.informers {
		hasSynced[i] = b.informers[i].HasSynced
	}

	return cache.WaitForCacheSync(ctx.Done(), hasSynced...)
}
