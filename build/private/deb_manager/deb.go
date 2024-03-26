package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/logger"
)

type UbuntuPackageIndex struct {
	Root string

	packageIndexes []*PackageIndex
}

func NewUbuntuPackageIndex(ctx context.Context, repos UbuntuRepositories) (*UbuntuPackageIndex, error) {
	pi := make(map[string]*bufio.Scanner)
	var wg sync.WaitGroup
	for _, repo := range repos {
		for i := len(repo.components) - 1; i >= 0; i-- {
			c := repo.components[i]
			wg.Add(1)
			go func(repo *UbuntuRepository, component string) {
				defer wg.Done()

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://www.ftp.ne.jp/Linux/packages/ubuntu/archive/dists/%s/%s/binary-%s/Packages.gz", repo.suite, component, repo.arch), nil)
				if err != nil {
					logger.Log.Error("Failed to create http request", logger.Error(err))
					return
				}
				client := &http.Client{
					Timeout: 10 * time.Minute,
				}
				logger.Log.Info("Request", zap.String("url", req.URL.String()))
				res, err := client.Do(req)
				if err != nil {
					logger.Log.Error("Failed to send http request", logger.Error(err))
					return
				}
				defer res.Body.Close()

				switch res.StatusCode {
				case http.StatusOK:
				default:
					logger.Log.Error("Got unexpected status", zap.Int("status", res.StatusCode), zap.String("suite", repo.suite), zap.String("component", component))
					return
				}

				r, err := gzip.NewReader(res.Body)
				if err != nil {
					logger.Log.Error("Failed to read Packages.gz", logger.Error(err))
					return
				}
				tmpF, err := os.CreateTemp("", "packages.gz")
				if err != nil {
					logger.Log.Error("Failed to read Packages.gz", logger.Error(err))
					return
				}
				if _, err := io.Copy(tmpF, r); err != nil {
					logger.Log.Error("Failed to read Packages.gz", logger.Error(err))
					return
				}
				if _, err := tmpF.Seek(0, io.SeekStart); err != nil {
					logger.Log.Error("Failed to read Packages.gz", logger.Error(err))
					return
				}
				pi[fmt.Sprintf("%s/%s", repo.suite, c)] = bufio.NewScanner(tmpF)
			}(repo, c)
		}
	}
	wg.Wait()

	var packageIndexes []*PackageIndex
	for _, repo := range repos {
		for i := len(repo.components) - 1; i >= 0; i-- {
			c := repo.components[i]
			pi := &PackageIndex{scanner: pi[fmt.Sprintf("%s/%s", repo.suite, c)], packages: make(map[string]*Package), root: "https://www.ftp.ne.jp/Linux/packages/ubuntu/archive"}
			if err := pi.parseIndex(); err != nil {
				return nil, err
			}
			packageIndexes = append(packageIndexes, pi)
		}
	}
	return &UbuntuPackageIndex{
		Root:           "https://www.ftp.ne.jp/Linux/packages/ubuntu/archive",
		packageIndexes: packageIndexes,
	}, nil
}

func NewPackageIndexFromFile(f string, _ *UbuntuRepository) (*UbuntuPackageIndex, error) {
	file, err := os.Open(f)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var reader io.Reader = file
	if strings.HasSuffix(f, ".gz") {
		r, err := gzip.NewReader(file)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		reader = r
	}
	return &UbuntuPackageIndex{
		Root:           "https://www.ftp.ne.jp/Linux/packages/ubuntu/archive",
		packageIndexes: []*PackageIndex{{scanner: bufio.NewScanner(reader), packages: make(map[string]*Package)}},
	}, nil
}

func (pi *UbuntuPackageIndex) Find(name string) (*Package, error) {
	for _, v := range pi.packageIndexes {
		if p, err := v.Find(name); err != nil {
			return nil, err
		} else if p != nil {
			return p, nil
		}
	}

	return nil, nil
}

func (*UbuntuPackageIndex) ResolveDependency(p *Package) []*Package {
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

type PackageIndex struct {
	root     string
	scanner  *bufio.Scanner
	packages map[string]*Package
}

func (pi *PackageIndex) Find(name string) (*Package, error) {
	p, ok := pi.packages[name]
	if !ok {
		return nil, nil
	}
	if p.depends != nil && p.Depends == nil {
		depends := make([]*Package, len(p.depends))
		for i, v := range p.depends {
			if _, ok := pi.packages[v]; !ok {
				continue
			}
			depends[i] = pi.packages[v]
		}
		p.Depends = depends
	}
	p.URL = pi.root + p.Filename
	return p, nil
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
			return xerrors.New("invalid line")
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

type UbuntuRepository struct {
	suite      string
	components []string
	arch       string
}

func NewUbuntuRepository(suite string, components ...string) *UbuntuRepository {
	return &UbuntuRepository{suite: suite, arch: "amd64", components: components}
}

type UbuntuRepositories []*UbuntuRepository

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
