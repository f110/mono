package storage

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMock_AddTree(t *testing.T) {
	m := NewMock()
	m.AddTree("test/foobar", []byte("foobar"))
	m.AddTree("test/baz", []byte("baz"))
	m.AddTree("good", []byte("good"))

	assert.NotNil(t, m.root)
	require.Len(t, m.root.Children, 2)
	assert.Equal(t, "test", m.root.Children[0].Name)
	assert.Equal(t, "good", m.root.Children[1].Name)
	require.Len(t, m.root.Children[0].Children, 2)
	assert.Equal(t, "foobar", m.root.Children[0].Children[0].Name)
	assert.Equal(t, "baz", m.root.Children[0].Children[1].Name)

	data, err := m.Get(context.Background(), "test/foobar")
	require.NoError(t, err)
	assert.NotNil(t, data)
	buf, err := io.ReadAll(data.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte("foobar"), buf)

	data, err = m.Get(context.Background(), "good")
	require.NoError(t, err)
	buf, err = io.ReadAll(data.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte("good"), buf)

	objs, err := m.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, objs, 3)
	objs, err = m.List(context.Background(), "test")
	require.NoError(t, err)
	if assert.Len(t, objs, 2) {
		assert.Contains(t, objs[0].Name, "test")
		assert.Contains(t, objs[1].Name, "test")
	}

	_, err = m.Get(context.Background(), "unknown")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrObjectNotFound)

	err = m.Put(context.Background(), "newfile", []byte("newfile"))
	require.NoError(t, err)
	data, err = m.Get(context.Background(), "newfile")
	require.NoError(t, err)
	buf, err = io.ReadAll(data.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte("newfile"), buf)

	err = m.Delete(context.Background(), "newfile")
	require.NoError(t, err)
	_, err = m.Get(context.Background(), "newfile")
	assert.ErrorIs(t, err, ErrObjectNotFound)

	err = m.Put(context.Background(), "update", []byte("init"))
	require.NoError(t, err)
	err = m.Put(context.Background(), "update", []byte("2nd"))
	require.NoError(t, err)
	data, err = m.Get(context.Background(), "update")
	require.NoError(t, err)
	buf, err = io.ReadAll(data.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte("2nd"), buf)

	err = m.Put(context.Background(), "test/update", []byte("init"))
	require.NoError(t, err)
	err = m.Put(context.Background(), "test/update", []byte("2nd"))
	require.NoError(t, err)
	data, err = m.Get(context.Background(), "test/update")
	require.NoError(t, err)
	buf, err = io.ReadAll(data.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte("2nd"), buf)
}
