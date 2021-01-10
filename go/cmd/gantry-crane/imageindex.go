package main

import (
	"bytes"
	"encoding/json"
	"errors"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/xerrors"
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

type imageIndex struct {
	images    []v1.Image
	mediaType types.MediaType
}

func NewImageIndex(img v1.Image) *imageIndex {
	mediaType := types.OCIImageIndex
	mt, _ := img.MediaType()
	switch mt {
	case types.DockerManifestSchema1, types.DockerManifestSchema2:
		mediaType = types.DockerManifestList
	}

	return &imageIndex{images: []v1.Image{img}, mediaType: mediaType}
}

func (i *imageIndex) AppendImage(img v1.Image) {
	i.images = append(i.images, img)
}

func (i *imageIndex) MediaType() (types.MediaType, error) {
	return i.mediaType, nil
}

func (i *imageIndex) Digest() (v1.Hash, error) {
	return emptyHash, nil
}

func (i *imageIndex) Size() (int64, error) {
	return 0, nil
}

func (i *imageIndex) IndexManifest() (*v1.IndexManifest, error) {
	desc := make([]v1.Descriptor, 0)
	for _, v := range i.images {
		mt, err := v.MediaType()
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		s, err := v.Size()
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		d, err := v.Digest()
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}

		desc = append(desc, v1.Descriptor{
			MediaType: mt,
			Size:      s,
			Digest:    d,
			// We assume that the image is for linux/amd64.
			Platform: &v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
		})
	}

	return &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     i.mediaType,
		Manifests:     desc,
	}, nil
}

func (i *imageIndex) RawManifest() ([]byte, error) {
	m, err := i.IndexManifest()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(m); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return buf.Bytes(), nil
}

func (i *imageIndex) Image(hash v1.Hash) (v1.Image, error) {
	for _, v := range i.images {
		d, err := v.Digest()
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		if d.String() == hash.String() {
			return v, nil
		}
	}

	return nil, errors.New("image not found")
}

func (i *imageIndex) ImageIndex(hash v1.Hash) (v1.ImageIndex, error) {
	panic("This ImageIndex is not ImageIndex")
}
