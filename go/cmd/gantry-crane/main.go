package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/logger"
)

var defaultArch = []string{"amd64"}

type config struct {
	Src      string
	Dst      string
	Platform []string
	Tags     []string
}

func gantryCrane(args []string) error {
	confFile := ""
	execute := false
	fs := pflag.NewFlagSet("gantry-crane", pflag.ContinueOnError)
	fs.StringVar(&confFile, "config", confFile, "Config file path")
	fs.BoolVar(&execute, "execute", execute, "Execute")
	if err := fs.Parse(args); err != nil {
		return xerrors.WithStack(err)
	}

	logs.Progress.SetOutput(os.Stdout)

	buf, err := ioutil.ReadFile(confFile)
	if err != nil {
		return xerrors.WithStack(err)
	}
	conf := make([]*config, 0)
	if err := yaml.Unmarshal(buf, &conf); err != nil {
		return xerrors.WithStack(err)
	}

	for _, v := range conf {
		platform := defaultArch
		if len(v.Platform) > 0 {
			platform = v.Platform
		}

	Tags:
		for _, tag := range v.Tags {
			logger.Log.Info("Synchronize %s:%s", zap.String("tag", fmt.Sprintf("%s:%s", v.Dst, tag)))
			oldDigest, dstIM, err := getIndexManifest(fmt.Sprintf("%s:%s", v.Dst, tag))
			if err != nil {
				return xerrors.WithStack(err)
			}
			exists := 0
			for _, a := range platform {
				if existImage(dstIM, a) {
					exists++
					continue
				}
			}

			srcRef, err := name.ParseReference(fmt.Sprintf("%s:%s", v.Src, tag))
			if err != nil {
				return xerrors.WithStack(err)
			}
			srcDescriptor, err := remote.Get(srcRef)
			if err != nil {
				return xerrors.WithStack(err)
			}

			logger.Log.Info(fmt.Sprintf("Old digest: %s, Src digest: %s", oldDigest.String(), srcDescriptor.Digest.String()))
			if srcDescriptor.Digest.String() == oldDigest.String() {
				logger.Log.Info("The image has been synced", zap.String("tag", fmt.Sprintf("%s:%s", v.Dst, tag)))
				continue
			}

			switch srcDescriptor.MediaType {
			case types.DockerManifestSchema2:
				if dstIM != nil && dstIM.MediaType == types.DockerManifestList {
					for _, d := range dstIM.Manifests {
						if d.Digest == srcDescriptor.Digest {
							logger.Log.Info("The image has been synced", zap.String("tag", fmt.Sprintf("%s:%s", v.Dst, tag)))
							continue Tags
						}
					}
				}
			}

			if !execute {
				logger.Log.Info("Image will be synchronized", zap.String("tag", fmt.Sprintf("%s:%s", v.Src, tag)), zap.Int("number_of_images", len(platform)-exists))
				continue
			}

			var newIndex v1.ImageIndex
			switch srcDescriptor.MediaType {
			case types.DockerManifestSchema2, types.OCIManifestSchema1:
				img, err := srcDescriptor.Image()
				if err != nil {
					return xerrors.WithStack(err)
				}
				newIndex = NewImageIndex(img)
			case types.DockerManifestList:
				newIndex, err = srcDescriptor.ImageIndex()
				if err != nil {
					return xerrors.WithStack(err)
				}
			default:
				index, err := srcDescriptor.ImageIndex()
				if err != nil {
					return xerrors.WithStack(err)
				}
				newIndex = NewPartialImageIndex(index, platform)
			}

			dstRef, err := name.ParseReference(fmt.Sprintf("%s:%s", v.Dst, tag))
			logger.Log.Info("Write index", zap.String("index", dstRef.String()))
			if err := remote.WriteIndex(dstRef, newIndex, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
				return xerrors.WithStack(err)
			}

			if oldDigest != emptyHash {
				oldD, err := name.NewDigest(fmt.Sprintf("%s@%s", v.Dst, oldDigest.String()))
				if err != nil {
					return xerrors.WithStack(err)
				}
				if err := remote.Delete(oldD, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
					return xerrors.WithStack(err)
				}
			}
		}
	}

	return nil
}

func existImage(indexM *v1.IndexManifest, arch string) bool {
	if indexM == nil {
		return false
	}

	for _, v := range indexM.Manifests {
		if v.Platform.Architecture == arch {
			return true
		}
	}

	return false
}

var emptyHash = v1.Hash{}

func getIndexManifest(ref string) (v1.Hash, *v1.IndexManifest, error) {
	r, err := name.ParseReference(ref)
	if err != nil {
		return emptyHash, nil, xerrors.WithStack(err)
	}
	desc, err := remote.Get(r, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		if tErr, ok := err.(*transport.Error); ok {
			for _, dErr := range tErr.Errors {
				// Manifest not found
				if dErr.Code == transport.ManifestUnknownErrorCode {
					logger.Log.Debug("Not found", zap.String("ref", ref))
					return emptyHash, nil, nil
				}
				// Harbor will return "NOT_FOUND"
				if dErr.Code == "NOT_FOUND" {
					logger.Log.Debug("NOT_FOUND", zap.String("ref", ref))
					return emptyHash, nil, nil
				}
			}
		}
		return emptyHash, nil, xerrors.WithStack(err)
	}
	index, err := desc.ImageIndex()
	if err != nil {
		return emptyHash, nil, xerrors.WithStack(err)
	}

	indexManifest, err := index.IndexManifest()
	if err != nil {
		return emptyHash, nil, err
	}

	return desc.Digest, indexManifest, nil
}

func main() {
	if err := gantryCrane(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}
