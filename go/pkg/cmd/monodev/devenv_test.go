package monodev

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComponentManager(t *testing.T) {
	component3 := &grpcServerComponent{Name: "3"}
	component2 := &grpcServerComponent{Name: "2", Deps: []component{component3}}
	component1 := &grpcServerComponent{Name: "1", Deps: []component{component2, component3}}

	cm := newComponentManager()
	cm.AddComponent(component1)

	root := cm.makeTree()
	assert.Equal(t, root.child[0].child[1].component.GetName(), root.child[0].child[0].child[0].component.GetName())
}
