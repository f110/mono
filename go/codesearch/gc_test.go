package codesearch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

func TestGC(t *testing.T) {
	logger.Init()
	mockStorage := storage.NewMock()
	mockStorage.AddTree("f110/mono/200/index.zoekt", []byte("foo")) // This file should be deleted
	manager := NewManifestManager(mockStorage)
	manifest := NewManifest(1654964861, map[string]string{
		"f110/mono": "mock://test/f110/mono/1654964861",
	}, 10)
	err := manager.Update(context.Background(), manifest)
	require.NoError(t, err)

	objs, _ := mockStorage.List(context.Background(), "/")
	for _, v := range objs {
		t.Log(v.Name)
	}

	gc := &IndexGC{backend: mockStorage, bucket: "test"}
	err = gc.GC(context.Background())
	require.NoError(t, err)

	_, err = mockStorage.Get(context.Background(), "f110/mono/200/index.zoekt")
	assert.Error(t, err, "storage: object not found")
}
