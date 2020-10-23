package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
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
		return xerrors.Errorf(": %w", err)
	}

	logs.Progress.SetOutput(os.Stdout)

	buf, err := ioutil.ReadFile(confFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	conf := make([]*config, 0)
	if err := yaml.Unmarshal(buf, &conf); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	for _, v := range conf {
		platform := defaultArch
		if len(v.Platform) > 0 {
			platform = v.Platform
		}

		for _, tag := range v.Tags {
			oldDigest, dstIM, err := getIndexManifest(fmt.Sprintf("%s:%s", v.Dst, tag))
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			exists := 0
			for _, a := range platform {
				if existImage(dstIM, a) {
					exists++
					continue
				}
			}

			if !execute {
				log.Printf("%s:%s will be synchronized %d images", v.Src, tag, len(platform)-exists)
				continue
			}
			if exists == len(platform) {
				log.Printf("%s:%s: all images have been synced", v.Dst, tag)
			}

			srcRef, err := name.ParseReference(fmt.Sprintf("%s:%s", v.Src, tag))
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			desc, err := remote.Get(srcRef)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			index, err := desc.ImageIndex()
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			partialIndex := NewPartialImageIndex(index, platform)

			dstRef, err := name.ParseReference(fmt.Sprintf("%s:%s", v.Dst, tag))
			if err := remote.WriteIndex(dstRef, partialIndex, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
				return xerrors.Errorf(": %w", err)
			}

			if oldDigest != emptyHash {
				oldD, err := name.NewDigest(fmt.Sprintf("%s@%s", v.Dst, oldDigest.String()))
				if err != nil {
					return xerrors.Errorf(": %w", err)
				}
				if err := remote.Delete(oldD, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
					return xerrors.Errorf(": %w", err)
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
		return emptyHash, nil, xerrors.Errorf(": %w", err)
	}
	desc, err := remote.Get(r, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		if tErr, ok := err.(*transport.Error); ok {
			for _, dErr := range tErr.Errors {
				// Manifest not found
				if dErr.Code == transport.ManifestUnknownErrorCode {
					return emptyHash, nil, nil
				}
			}
		}
		return emptyHash, nil, xerrors.Errorf(": %w", err)
	}
	index, err := desc.ImageIndex()
	if err != nil {
		return emptyHash, nil, xerrors.Errorf(": %w", err)
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
