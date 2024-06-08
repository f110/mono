package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirectedGraph(t *testing.T) {
	g := NewDirectedGraph[string]()
	n1 := g.NewNode("component1")
	n2 := g.NewNode("component2")
	n3 := g.NewNode("component3")
	n4 := g.NewNode("component4")
	n5 := g.NewNode("component5")
	n6 := g.NewNode("component6")
	n7 := g.NewNode("component7")
	n8 := g.NewNode("component8")
	n9 := g.NewNode("component9")
	g.AddEdge(n1, n2)
	g.AddEdge(n2, n3)
	g.AddEdge(n3, n4)
	g.AddEdge(n4, n2)
	g.AddEdge(n4, n5)
	g.AddEdge(n5, n6)
	g.AddEdge(n6, n7)
	g.AddEdge(n7, n5)
	g.AddEdge(n5, n8)
	g.AddEdge(n8, n7)
	g.AddEdge(n7, n9)
}

func TestStronglyConnectedComponents(t *testing.T) {
	g := NewDirectedGraph[string]()
	n1 := g.NewNode("component1")
	n2 := g.NewNode("component2")
	n3 := g.NewNode("component3")
	n4 := g.NewNode("component4")
	n5 := g.NewNode("component5")
	n6 := g.NewNode("component6")
	n7 := g.NewNode("component7")
	n8 := g.NewNode("component8")
	n9 := g.NewNode("component9")
	g.AddEdge(n1, n2)
	g.AddEdge(n2, n3)
	g.AddEdge(n3, n4)
	g.AddEdge(n4, n2)
	g.AddEdge(n4, n5)
	g.AddEdge(n5, n6)
	g.AddEdge(n6, n7)
	g.AddEdge(n7, n5)
	g.AddEdge(n5, n8)
	g.AddEdge(n8, n7)
	g.AddEdge(n7, n9)

	loops := StronglyConnectedComponents(g)
	assert.Len(t, loops, 4)
	for _, v := range loops {
		if len(v) == 4 {
			assert.Contains(t, Values(v), "component5")
			assert.Contains(t, Values(v), "component6")
			assert.Contains(t, Values(v), "component7")
			assert.Contains(t, Values(v), "component8")
		}
		if len(v) == 3 {
			assert.Contains(t, Values(v), "component2")
			assert.Contains(t, Values(v), "component3")
			assert.Contains(t, Values(v), "component4")
		}
	}
}
