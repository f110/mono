package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

type PackageIndex struct {
	Root string

	scanner  *bufio.Scanner
	packages map[string]*Package
}

func NewPackageIndex(ctx context.Context, ss *Snapshot) (*PackageIndex, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://snapshot.debian.org/archive/%s/%s/dists/%s/main/binary-%s/Packages.gz", ss.repository, ss.snapshot, ss.codename, ss.arch), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 10 * time.Minute,
	}
	logger.Log.Info("Request", zap.String("url", req.URL.String()))
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	default:
		return nil, xerrors.Definef("got %s", res.Status).WithStack()
	}

	r, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, err
	}
	tmpF, err := os.CreateTemp("", "packages.gz")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(tmpF, r); err != nil {
		return nil, err
	}
	if _, err := tmpF.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return &PackageIndex{
		Root:     fmt.Sprintf("https://snapshot.debian.org/archive/%s/%s/", ss.repository, ss.snapshot),
		scanner:  bufio.NewScanner(tmpF),
		packages: make(map[string]*Package),
	}, nil
}

func NewPackageIndexFromFile(f string, ss *Snapshot) (*PackageIndex, error) {
	file, err := os.Open(f)
	if err != nil {
		return nil, err
	}

	var reader io.Reader = file
	if strings.HasSuffix(f, ".gz") {
		r, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		reader = r
	}
	return &PackageIndex{
		Root:     fmt.Sprintf("https://snapshot.debian.org/archive/%s/%s/", ss.repository, ss.snapshot),
		scanner:  bufio.NewScanner(reader),
		packages: make(map[string]*Package),
	}, nil
}

func (pi *PackageIndex) Find(name string) (*Package, error) {
	if err := pi.parseIndex(); err != nil {
		return nil, err
	}

	p := pi.packages[name]
	if p == nil {
		return nil, nil
	}
	if p.depends != nil && p.Depends == nil {
		depends := make([]*Package, 0, len(p.depends))
		for _, v := range p.depends {
			if _, ok := pi.packages[v]; !ok {
				logger.Log.Info("Not found package", zap.String("name", name))
				continue
			}
			depends = append(depends, pi.packages[v])
		}
		p.Depends = depends
	}
	p.URL = pi.Root + p.Filename
	return p, nil
}

func (pi *PackageIndex) ResolveDependency(p *Package) []*Package {
	deps := make(map[string]*Package)
	for _, v := range p.Depends {
		p := v
		deps[v.Name] = p
	}
	if p.Depends == nil {
		return nil
	}

	m := make(map[string]struct{})
	s := p.Depends
	for ; len(s) > 0; s = s[1:] {
		v := s[0]
		if v == nil || v.Depends == nil {
			continue
		}
		if _, ok := m[v.Name]; ok {
			continue
		}
		s = append(s, v.Depends...)
		if _, ok := deps[v.Name]; !ok {
			deps[v.Name] = v
		}
		m[v.Name] = struct{}{}
	}

	res := make([]*Package, 0, len(deps))
	for _, v := range deps {
		v := v
		res = append(res, v)
	}
	return res
}

func (pi *PackageIndex) parseIndex() error {
	if pi.scanner == nil {
		return nil
	}

	buf := make([]byte, 1024*1024)
	pi.scanner.Buffer(buf, len(buf))
	p := &Package{}
	for pi.scanner.Scan() {
		line := pi.scanner.Text()
		if line == "" {
			pi.packages[p.Name] = p
			p = &Package{}
			continue
		}

		i := strings.Index(line, ":")
		if i == -1 {
			return errors.New("invalid line")
		}
		switch line[:i] {
		case "Package":
			p.Name = line[i+2:]
		case "SHA256":
			p.SHA256 = line[i+2:]
		case "Filename":
			p.Filename = line[i+2:]
		case "Depends":
			p.depends = append(p.depends, parseDepends(line[i+2:])...)
		case "Pre-Depends":
			p.depends = append(p.depends, parseDepends(line[i+2:])...)
		}
	}

	pi.scanner = nil
	return nil
}

type Package struct {
	Name     string
	SHA256   string
	Filename string
	Depends  []*Package
	URL      string
	depends  []string
}

type Snapshot struct {
	repository string
	snapshot   string
	arch       string
	codename   string
}

func NewSnapShot(repo, codename, snapshot string) *Snapshot {
	return &Snapshot{repository: repo, snapshot: snapshot, arch: "amd64", codename: codename}
}

func parseDepends(in string) []string {
	depends := make([]string, 0, strings.Count(in, ","))
	for _, v := range strings.Split(in, ", ") {
		i := strings.Index(v, " ")
		if i > 0 {
			depends = append(depends, v[:i])
		} else {
			depends = append(depends, v)
		}
	}
	return depends
}
