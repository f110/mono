package main

import (
	"container/list"
	"context"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/parallel"
	"go.f110.dev/mono/go/pkg/logger"
)

type Manager []*cobra.Command

var CommandManager = make(Manager, 0)

func (m *Manager) Add(cmd *cobra.Command) {
	for _, v := range *m {
		cmd.AddCommand(v)
	}
}

func (m *Manager) Register(cmd *cobra.Command) {
	*m = append(*m, cmd)
}

type componentType int

const (
	componentTypeService = iota
	componentTypeOneshot
)

type component interface {
	GetName() string
	GetType() componentType
	GetDeps() []component
	Run(ctx context.Context)
}

type componentManager struct {
	components []component
	supervisor *parallel.Supervisor
}

func newComponentManager() *componentManager {
	return &componentManager{}
}

func (m *componentManager) Run(ctx context.Context) error {
	tree := m.makeTree()
	sorted := m.executionPlan(tree)

	m.supervisor = parallel.NewSupervisor(ctx)

	go m.scheduler(ctx, 2, sorted)

	// Wait for interrupts
	<-ctx.Done()

	logger.Log.Debug("Shutting down")
	ctx, cFunc := context.WithTimeout(context.Background(), 5*time.Second)
	if err := m.supervisor.Shutdown(ctx); err != nil {
		cFunc()
		return xerrors.WithStack(err)
	}
	cFunc()
	logger.Log.Info("All subprocesses finished")

	return nil
}

func (m *componentManager) scheduler(ctx context.Context, workers int, sorted *list.List) {
	sigCh := make(chan struct{}, 1)
	inFlight := int32(0)
	sigCh <- struct{}{}

	for {
		select {
		case <-sigCh:
		case <-ctx.Done():
			return
		}

		for inFlight < int32(workers) {
			n, e := m.nextExecutableNode(sorted)
			if n == nil {
				if sorted.Len() == 0 {
					// All nodes were executed
					return
				}

				// There is no executable node.
				// Wait for next signal
				break
			}

			atomic.AddInt32(&inFlight, 1)
			n.status = nodeStatusStarting
			go m.execute(ctx, &inFlight, sigCh, e, sorted)
		}
	}
}

func (m *componentManager) execute(ctx context.Context, inFlight *int32, sigCh chan struct{}, e *list.Element, sorted *list.List) {
	defer func() {
		atomic.AddInt32(inFlight, -1)

		select {
		case sigCh <- struct{}{}:
		default:
		}
	}()

	n := e.Value.(*componentNode)
	if n.component == nil {
		return
	}

	switch n.component.GetType() {
	case componentTypeOneshot:
		logger.Log.Info("Run oneshot script", zap.String("name", n.component.GetName()))
		n.component.Run(ctx)
		n.status = nodeStatusFinished
	case componentTypeService:
		logger.Log.Info("Start service", zap.String("name", n.component.GetName()))
		m.supervisor.Add(n.component.Run)

		go func(n *componentNode) {
			defer func() {
				select {
				case sigCh <- struct{}{}:
				default:
				}
			}()

			r, ok := n.component.(interface{ Ready() bool })
			if !ok {
				n.status = nodeStatusRunning
				logger.Log.Info("The service is ready (without readiness probe)", zap.String("name", n.component.GetName()))
				return
			}

			deadline := time.Now().Add(30 * time.Second)
			for {
				if r.Ready() {
					n.status = nodeStatusRunning
					logger.Log.Info("The service is ready", zap.String("name", n.component.GetName()))
					break
				}
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(n)
	}

	sorted.Remove(e)
}

func (m *componentManager) makeTree() *componentNode {
	root := &componentNode{}
	nodes := make(map[component]*componentNode)
	for _, v := range m.components {
		n := m.makeTreeNode(v, nodes)
		root.child = append(root.child, n)
	}

	return root
}

func (m *componentManager) makeTreeNode(c component, nodes map[component]*componentNode) *componentNode {
	node, ok := nodes[c]
	if !ok {
		node = &componentNode{component: c}
		nodes[c] = node
		for _, d := range c.GetDeps() {
			v := m.makeTreeNode(d, nodes)
			node.child = append(node.child, v)
		}
	}

	return node
}

func (m *componentManager) executionPlan(tree *componentNode) *list.List {
	sorted := list.New()
	var visit func(n *componentNode, s *list.List)
	visit = func(n *componentNode, s *list.List) {
		if n.visited {
			return
		}
		n.visited = true
		for _, v := range n.child {
			visit(v, s)
		}
		s.PushBack(n)
	}
	for _, v := range tree.child {
		visit(v, sorted)
	}
	sorted.PushBack(tree)

	return sorted
}

func (m *componentManager) nextExecutableNode(sorted *list.List) (*componentNode, *list.Element) {
	for e := sorted.Front(); e != nil; e = e.Next() {
		n := e.Value.(*componentNode)
		if n.Executable() {
			return n, e
		}
	}

	return nil, nil
}

func (m *componentManager) AddComponent(c component) {
	m.components = append(m.components, c)
}

type nodeStatus int

const (
	nodeStatusNotRunning nodeStatus = iota
	nodeStatusStarting
	nodeStatusRunning
	nodeStatusFinished
)

type componentNode struct {
	component component
	child     []*componentNode

	visited bool
	status  nodeStatus
}

func (n *componentNode) Executable() bool {
	if n.status != nodeStatusNotRunning {
		return false
	}

	for _, v := range n.child {
		switch v.status {
		case nodeStatusFinished, nodeStatusRunning:
		default:
			return false
		}
	}

	return true
}
