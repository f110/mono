package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger"
)

var codenameToName = map[string]string{
	"bullseye": "debian11",
	"bookworm": "debian12",
}

type config struct {
	Snapshot struct {
		Distro map[string]map[string]string `yaml:"distro"`
	} `yaml:"snapshot"`
	Packages []string `yaml:"packages"`
}

func generate(ctx context.Context, confFile, macroFile, outFile string) error {
	conf, err := readConfig(confFile)
	if err != nil {
		return err
	}

	distros := make([]string, 0)
	packageIndices := make(map[string]*PackageIndex)
	distroAndPackages := make(map[string][]*Package)
	for distro, v := range conf.Snapshot.Distro {
		distros = append(distros, distro)
		for name, snapshot := range v {
			ss := NewSnapShot(name, distro, snapshot)
			pi, err := NewPackageIndex(ctx, ss)
			if err != nil {
				return err
			}
			packageIndices[distro] = pi
			deps := conf.Packages
			allPackages := make(map[string]*Package)
			for {
				if len(deps) == 0 {
					break
				}
				v := deps[0]
				if _, ok := allPackages[v]; ok {
					deps = deps[1:]
					continue
				}

				p, err := pi.Find(v)
				if err != nil {
					return err
				}
				if p == nil {
					logger.Log.Info("package not found", zap.String("name", v))
					deps = deps[1:]
					continue
				}
				allPackages[p.Name] = p
				for _, d := range p.Depends {
					if d == nil {
						continue
					}
					d := d
					deps = append(deps, d.Name)
				}
				deps = deps[1:]
			}

			packages := make([]*Package, 0, len(allPackages))
			for _, v := range allPackages {
				packages = append(packages, v)
			}
			distroAndPackages[distro] = packages
		}
	}

	rMacroFile, err := makeRepositoryMacroFile(outFile)
	if err != nil {
		return err
	}
	for distro, _ := range conf.Snapshot.Distro {
		writePackageDep(rMacroFile, distro, distroAndPackages[distro])
	}

	if macroFile != "" {
		if err := makeMacroFile(macroFile, distros, conf.Packages, packageIndices); err != nil {
			return err
		}
	}
	return nil
}

func makeRepositoryMacroFile(outFile string) (*os.File, error) {
	out, err := os.Create(outFile)
	if err != nil {
		return nil, err
	}
	out.WriteString(`load("//build/rules/deb:deb.bzl", "deb_pkg")

def debian_packages():
`)

	return out, nil

}

func writePackageDep(w io.Writer, distro string, packages []*Package) {
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })

	for _, v := range packages {
		fmt.Fprint(w, "    deb_pkg(\n")
		fmt.Fprintf(w, "        name = \"%s_%s\",\n", codenameToName[distro], strings.Replace(v.Name, "+", "_", -1))
		fmt.Fprintf(w, "        package_name = \"%s\",\n", v.Name)
		fmt.Fprintf(w, "        sha256 = \"%s\",\n", v.SHA256)
		fmt.Fprintf(w, "        urls = [\"%s\"],\n", v.URL)
		fmt.Fprint(w, "    )\n")
		fmt.Fprint(w, "\n")
	}
}

func makeMacroFile(outFile string, distros, packages []string, packageIndices map[string]*PackageIndex) error {
	out, err := os.Create(outFile)
	if err != nil {
		return err
	}
	out.WriteString("package_dependencies = {\n")
	for _, distro := range distros {
		fmt.Fprintf(out, "    %q: {\n", codenameToName[distro])
		pi := packageIndices[distro]
		for _, pkgName := range packages {
			p, err := pi.Find(pkgName)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "        %q: ", pkgName)
			deps := packageIndices[distro].ResolveDependency(p)
			depPkgNames := enumerable.Map(deps, func(t *Package) string { return t.Name })
			depPkgNames = enumerable.Map(depPkgNames, func(v string) string { return fmt.Sprintf("%q", v) })
			sort.Strings(depPkgNames)
			fmt.Fprintf(out, "[%s],\n", strings.Join(depPkgNames, ", "))
		}
		out.WriteString("    }\n")
	}
	out.WriteString("}\n\n")

	out.WriteString(`def deb_pkg(distro, excludes = None, *pkgs):
    all = {}
    for x in pkgs:
        if x in excludes:
            continue
        all[x] = None
        for x in package_dependencies[distro][x]:
            all[x] = None
    return ["@%s_%s//:data" % (distro, k.replace("+", "_")) for k in all]`)
	return nil
}

func readConfig(confFile string) (*config, error) {
	f, err := os.Open(confFile)
	if err != nil {
		return nil, err
	}
	var conf config
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

func main() {
	logger.Init()
	var confFile, utilityMacroFile string
	flag.StringVar(&confFile, "conf", "", "Config file")
	flag.StringVar(&utilityMacroFile, "macro", "", "Macro file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := generate(ctx, confFile, utilityMacroFile, flag.Args()[0]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
