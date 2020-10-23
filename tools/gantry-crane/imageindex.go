package main

import (
	"bytes"
	"encoding/json"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type partialImageIndex struct {
	internal v1.ImageIndex
	arch     map[string]struct{}
}

func NewPartialImageIndex(src v1.ImageIndex, arch []string) *partialImageIndex {
	m := make(map[string]struct{})
	for _, v := range arch {
		m[v] = struct{}{}
	}

	return &partialImageIndex{
		internal: src,
		arch:     m,
	}
}

func (i *partialImageIndex) MediaType() (types.MediaType, error) {
	return i.internal.MediaType()
}

func (i *partialImageIndex) Digest() (v1.Hash, error) {
	return i.internal.Digest()
}

func (i *partialImageIndex) Size() (int64, error) {
	return i.internal.Size()
}

func (i *partialImageIndex) IndexManifest() (*v1.IndexManifest, error) {
	manifest, err := i.internal.IndexManifest()
	if err != nil {
		return nil, err
	}

	descriptors := make([]v1.Descriptor, 0)
	for _, desc := range manifest.Manifests {
		if _, ok := i.arch[desc.Platform.Architecture]; ok {
			descriptors = append(descriptors, desc)
		}
	}

	n := manifest.DeepCopy()
	n.Manifests = descriptors
	return n, nil
}

func (i *partialImageIndex) RawManifest() ([]byte, error) {
	m, err := i.IndexManifest()
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(m); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (i *partialImageIndex) Image(hash v1.Hash) (v1.Image, error) {
	return i.internal.Image(hash)
}

func (i *partialImageIndex) ImageIndex(hash v1.Hash) (v1.ImageIndex, error) {
	return i.internal.ImageIndex(hash)
}
