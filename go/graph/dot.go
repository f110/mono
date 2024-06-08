package graph

import (
	"fmt"
	"io"

	"go.f110.dev/xerrors"
)

type DotEncoder[T comparable] struct {
	w       io.Writer
	rankdir string
}

func NewDotEncoder[T comparable](out io.Writer) *DotEncoder[T] {
	return &DotEncoder[T]{w: out}
}

func (e *DotEncoder[T]) Rankdir(rd string) *DotEncoder[T] {
	e.rankdir = rd
	return e
}

func (e *DotEncoder[T]) Encode(g *DirectedGraph[T]) error {
	if len(g.nodes) == 0 {
		return xerrors.Define("the graph is empty")
	}

	fmt.Fprintln(e.w, "digraph {")
	if e.rankdir != "" {
		fmt.Fprintln(e.w, "  rankdir=\""+e.rankdir+"\"")
	}
	m := make(map[*Node[T]]int)
	for i, v := range g.nodes {
		m[v] = i + 1
		fmt.Fprintf(e.w, "  n%d [label=%v]\n", i+1, v.Value)
	}
	for _, v := range g.nodes {
		s := fmt.Sprintf("n%d", m[v])
		for _, edge := range v.outEdges {
			fmt.Fprintf(e.w, "  %s -> n%d\n", s, m[edge.destination])
		}
	}
	fmt.Fprintln(e.w, "}")
	return nil
}
