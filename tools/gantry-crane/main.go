package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

var amd64Platform = &v1.Platform{
	Architecture: "amd64",
	OS:           "linux",
}
var arm64Platform = &v1.Platform{
	Architecture: "arm64",
	OS:           "linux",
}

type config struct {
	Src      string
	Dst      string
	Platform string
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
		var pf *v1.Platform
		switch v.Platform {
		case "arm64":
			pf = arm64Platform
		default:
			pf = amd64Platform
		}
		repo, err := name.NewRepository(v.Dst)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		t, err := remote.List(repo, remote.WithPlatform(*pf), remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		dstTags := make(map[string]struct{})
		for _, v := range t {
			dstTags[v] = struct{}{}
		}

		for _, tag := range v.Tags {
			if _, ok := dstTags[tag]; ok {
				log.Printf("%s:%s is already exist", v.Dst, tag)
				continue
			}
			if !execute {
				log.Printf("[Dry run] Copy %s:%s to %s:%s", v.Src, tag, v.Dst, tag)
				continue
			}

			if err := crane.Copy(fmt.Sprintf("%s:%s", v.Src, tag), fmt.Sprintf("%s:%s", v.Dst, tag), crane.WithPlatform(pf)); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
	}

	return nil
}

func main() {
	if err := gantryCrane(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}
