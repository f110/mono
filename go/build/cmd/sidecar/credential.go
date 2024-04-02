package sidecar

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
)

type CredentialCommand struct {
	dir string
	out string
}

func NewCredentialCommand() *CredentialCommand {
	return &CredentialCommand{}
}

func (c *CredentialCommand) SetGlobalFlags(_ *cli.FlagSet) {}

func (c *CredentialCommand) SetContainerRegistryFlags(fs *cli.FlagSet) {
	fs.String("dir", "Directory path").Var(&c.dir).Required()
	fs.String("out", "Output path").Var(&c.out)
}

type containerRegistryAuthFile struct {
	Auths map[string]*containerRegistryHostAuth `json:"auths"`
}

type containerRegistryHostAuth struct {
	Auth string `json:"auth"`
}

func (c *CredentialCommand) ContainerRegistry(_ context.Context) error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return xerrors.WithStack(err)
	}

	conf := &containerRegistryAuthFile{Auths: make(map[string]*containerRegistryHostAuth)}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		host := e.Name()
		files, err := os.ReadDir(filepath.Join(c.dir, e.Name()))
		if err != nil {
			return xerrors.WithStack(err)
		}
		for _, v := range files {
			if v.IsDir() {
				continue
			}
			buf, err := os.ReadFile(filepath.Join(c.dir, e.Name(), v.Name()))
			if err == nil {
				conf.Auths[host] = &containerRegistryHostAuth{Auth: base64.StdEncoding.EncodeToString(append([]byte(v.Name()+":"), buf...))}
			}
		}
	}

	f, err := os.Create(c.out)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := json.NewEncoder(f).Encode(conf); err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}
