package storage

import (
	"bytes"
	"container/list"
	"context"
	"io"
	"strings"

	"go.f110.dev/xerrors"
)

type Mock struct {
	root *objectNode
}

type objectNode struct {
	Name string

	Data     []byte
	Children []*objectNode

	parent *objectNode
}

func (n *objectNode) FullPath() string {
	p := n.Name

	for v := n.parent; v != nil; v = v.parent {
		if v.Name != "" {
			p = v.Name + "/" + p
		}
	}

	return p
}

var _ storageInterface = &Mock{}

func NewMock() *Mock {
	return &Mock{root: &objectNode{}}
}

func (m *Mock) Name() string {
	return "mock"
}

func (m *Mock) AddTree(name string, data []byte) {
	m.addNode(name, data)
}

func (m *Mock) addNode(name string, data []byte) {
	p := strings.Split(name, "/")
	if len(p) == 0 {
		return
	}
	if len(p) == 1 {
		m.root.Children = append(m.root.Children, &objectNode{
			Name:   p[0],
			Data:   data,
			parent: m.root,
		})
	}

	nodeName := p[0]
	p = p[1:]
	curr := m.root
NextChild:
	for {
		for _, v := range curr.Children {
			if v.Name == nodeName {
				if len(p) == 0 {
					break NextChild
				}
				curr = v
				nodeName = p[0]
				p = p[1:]
				continue NextChild
			}
		}

		newNode := &objectNode{Name: nodeName, parent: curr}
		curr.Children = append(curr.Children, newNode)
		curr = newNode
		if len(p) > 0 {
			nodeName = p[0]
			p = p[1:]
		} else {
			newNode.Data = data
			break
		}
	}

	return
}

func (m *Mock) findNode(name string) *objectNode {
	if name == "" {
		return m.root
	}
	p := strings.Split(name, "/")

	nodeName := p[0]
	p = p[1:]
	curr := m.root
NextChild:
	for {
		for _, v := range curr.Children {
			if v.Name == nodeName {
				curr = v
				if len(p) == 0 {
					return v
				}

				nodeName = p[0]
				p = p[1:]
				continue NextChild
			}
		}
		break
	}

	return nil
}

func (m *Mock) deleteNode(name string) {
	n := m.findNode(name)
	if n == nil {
		return
	}
	p := strings.Split(name, "/")
	last := p[len(p)-1]
	for i, v := range n.parent.Children {
		if v.Name == last {
			n.parent.Children = append(n.parent.Children[:i], n.parent.Children[i+1:]...)
			break
		}
	}
}

func (m *Mock) Get(_ context.Context, name string) (io.ReadCloser, error) {
	n := m.findNode(name)
	if n != nil {
		return io.NopCloser(bytes.NewReader(n.Data)), nil
	}

	return nil, xerrors.WithStack(ErrObjectNotFound)
}

func (m *Mock) List(_ context.Context, prefix string) ([]*Object, error) {
	n := m.findNode(prefix)
	if n == nil {
		return nil, nil
	}
	var objs []*Object
	stack := list.New()
	for _, v := range n.Children {
		stack.PushBack(v)
	}
	for stack.Len() > 0 {
		e := stack.Back()
		stack.Remove(e)

		obj := e.Value.(*objectNode)
		if obj.Data != nil {
			objs = append(objs, &Object{Name: obj.FullPath(), Size: int64(len(obj.Data))})
		}
		for _, v := range obj.Children {
			stack.PushBack(v)
		}
	}
	return objs, nil
}

func (m *Mock) Put(_ context.Context, name string, data []byte) error {
	m.addNode(name, data)
	return nil
}

func (m *Mock) PutReader(ctx context.Context, name string, data io.Reader) error {
	buf, err := io.ReadAll(data)
	if err != nil {
		return xerrors.WithStack(err)
	}
	return m.Put(ctx, name, buf)
}

func (m *Mock) Delete(_ context.Context, name string) error {
	m.deleteNode(name)
	return nil
}
