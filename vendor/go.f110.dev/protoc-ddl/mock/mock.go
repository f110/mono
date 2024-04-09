package mock

import (
	"fmt"

	"go.f110.dev/xerrors"
)

type call struct {
	args  map[string]interface{}
	value interface{}
	err   error
}

var ErrNotRegistered = xerrors.New("not registered")

type Mock struct {
	mock   map[string][]*call
	called map[string][]*call
}

func New() *Mock {
	return &Mock{mock: make(map[string][]*call), called: make(map[string][]*call)}
}

func (m *Mock) Register(name string, args map[string]interface{}, value interface{}, err error) {
	if _, ok := m.mock[name]; !ok {
		m.mock[name] = make([]*call, 0)
	}
	m.mock[name] = append(m.mock[name], &call{args: args, value: value, err: err})
}

func (m *Mock) Call(name string, args map[string]interface{}) (interface{}, error) {
	if _, ok := m.called[name]; !ok {
		m.called[name] = make([]*call, 0)
	}
	m.called[name] = append(m.called[name], &call{args: args})

	if _, ok := m.mock[name]; !ok {
		return nil, ErrNotRegistered
	}

	for _, v := range m.mock[name] {
		if fmt.Sprint(v.args) == fmt.Sprint(args) {
			return v.value, v.err
		}
	}

	return nil, ErrNotRegistered
}

func (m *Mock) Reset() {
	m.mock = make(map[string][]*call)
}

type Call struct {
	Args map[string]interface{}
}

func (m *Mock) Called(name string) []Call {
	if _, ok := m.called[name]; !ok {
		return nil
	}

	c := make([]Call, len(m.called[name]))
	for i, v := range m.called[name] {
		c[i] = Call{Args: v.args}
	}

	return c
}
