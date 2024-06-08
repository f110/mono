package graph

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDotEncoder(t *testing.T) {
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

	buf := new(bytes.Buffer)
	err := NewDotEncoder[string](buf).Rankdir("LR").Encode(g)
	require.NoError(t, err)
}
