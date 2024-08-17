package graph

type DirectedGraph[T comparable] struct {
	nodes []*Node[T]
}

type Node[T comparable] struct {
	Value T

	inEdges  map[*Node[T]]*Edge[T]
	outEdges map[*Node[T]]*Edge[T]
}

type Edge[T comparable] struct {
	source      *Node[T]
	destination *Node[T]
}

func NewDirectedGraph[T comparable]() *DirectedGraph[T] {
	return &DirectedGraph[T]{}
}

func (g *DirectedGraph[T]) NewNode(v T) *Node[T] {
	n := &Node[T]{Value: v, inEdges: make(map[*Node[T]]*Edge[T]), outEdges: make(map[*Node[T]]*Edge[T])}
	g.nodes = append(g.nodes, n)
	return n
}

func (g *DirectedGraph[T]) AddEdge(source, destination *Node[T]) {
	e := &Edge[T]{destination: destination}
	source.outEdges[destination] = e
	e = &Edge[T]{source: source}
	destination.inEdges[source] = e
}

func Values[T comparable](in []*Node[T]) []T {
	v := make([]T, len(in))
	for i := range in {
		v[i] = in[i].Value
	}
	return v
}

func StronglyConnectedComponents[T comparable](g *DirectedGraph[T]) [][]*Node[T] {
	var visit func(n *Node[T])
	seen := make(map[*Node[T]]bool)
	index := 0
	order := make(map[int]*Node[T])
	visit = func(n *Node[T]) {
		seen[n] = true
		for _, v := range n.outEdges {
			if seen[v.destination] {
				continue
			}
			visit(v.destination)
		}
		order[index] = n
		index++
	}
	for _, v := range g.nodes {
		if seen[v] {
			continue
		}
		visit(v)
	}

	groups := make(map[*Node[T]]int)
	seen = make(map[*Node[T]]bool)
	var rvisit func(n *Node[T], group int)
	rvisit = func(n *Node[T], group int) {
		groups[n] = group
		seen[n] = true
		for _, v := range n.inEdges {
			if seen[v.source] {
				continue
			}
			rvisit(v.source, group)
		}
	}

	rnodes := make([]*Node[T], len(g.nodes))
	for i := range len(g.nodes) {
		rnodes[i] = order[len(g.nodes)-i-1]
	}
	for i, v := range rnodes {
		if seen[v] {
			continue
		}
		rvisit(v, i)
	}

	groupBy := make(map[int][]*Node[T])
	for v, i := range groups {
		groupBy[i] = append(groupBy[i], v)
	}
	if len(groupBy) == len(g.nodes) {
		return nil
	}

	loops := make([][]*Node[T], 0)
	for _, v := range groupBy {
		loops = append(loops, v)
	}
	return loops
}
