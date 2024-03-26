package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"go.f110.dev/xerrors"
	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/enumerable"
)

var preIncludedPackages = []string{
	"libc6",
	"base-files",
	"net-base",
	"tzdata",
	"libssl3", // debian12
}

var excludePackages map[string]any

func init() {
	excludePackages = make(map[string]any)
	for _, v := range preIncludedPackages {
		excludePackages[v] = struct{}{}
	}
}

type config struct {
	Ubuntu *ubuntu `json:"ubuntu"`
}

type ubuntu struct {
	Jammy *ubuntuVersion `json:"jammy"`
}

type ubuntuVersion struct {
	Packages []string `yaml:"packages"`
}

func generate(ctx context.Context, confFile, macroFile, outFile string) error {
	conf, err := readConfig(confFile)
	if err != nil {
		return err
	}

	rMacroFile, err := makeRepositoryMacroFile(outFile)
	if err != nil {
		return err
	}
	var distros []string

	if conf.Ubuntu != nil && conf.Ubuntu.Jammy != nil {
		repos := make(UbuntuRepositories, 0)
		repos = append(repos,
			NewUbuntuRepository("jammy-security", "main", "restricted", "universe", "multiverse"),
			NewUbuntuRepository("jammy-backports", "main", "restricted", "universe", "multiverse"),
			NewUbuntuRepository("jammy-updates", "main", "restricted", "universe", "multiverse"),
			NewUbuntuRepository("jammy", "main", "restricted", "universe", "multiverse"),
		)

		pi, err := NewUbuntuPackageIndex(ctx, repos)
		if err != nil {
			return err
		}
		allPackages := make(map[string]*Package)
		deps := conf.Ubuntu.Jammy.Packages
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
			allPackages[p.Name] = p
			for _, d := range p.Depends {
				if d == nil {
					continue
				}
				deps = append(deps, d.Name)
			}
			deps = deps[1:]
		}

		packages := make([]*Package, 0, len(allPackages))
		for _, v := range allPackages {
			packages = append(packages, v)
		}

		writePackageDep(rMacroFile, "jammy", packages)
		distros = append(distros, "jammy")
		if macroFile != "" {
			if err := makeMacroFile(macroFile, "jammy", conf.Ubuntu.Jammy.Packages, pi); err != nil {
				return err
			}
		}
	}
	return nil
}

func makeRepositoryMacroFile(outFile string) (*os.File, error) {
	out, err := os.Create(outFile)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	out.WriteString(`load("//build/rules/deb:deb.bzl", "deb_pkg")

def debian_packages():
`)

	return out, nil

}

func writePackageDep(w io.Writer, version string, packages []*Package) {
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })

	for _, v := range packages {
		if _, ok := excludePackages[v.Name]; ok {
			continue
		}
		fmt.Fprint(w, "    deb_pkg(\n")
		fmt.Fprintf(w, "        name = \"%s_%s\",\n", version, v.Name)
		fmt.Fprintf(w, "        package_name = \"%s\",\n", v.Name)
		fmt.Fprintf(w, "        sha256 = \"%s\",\n", v.SHA256)
		fmt.Fprintf(w, "        urls = [\"%s\"],\n", v.URL)
		fmt.Fprint(w, "    )\n")
		fmt.Fprint(w, "\n")
	}
}

func makeMacroFile(outFile, version string, packages []string, pi *UbuntuPackageIndex) error {
	out, err := os.Create(outFile)
	if err != nil {
		return xerrors.WithStack(err)
	}
	out.WriteString("package_dependencies = {\n")
	fmt.Fprintf(out, "    %q: {\n", version)
	for _, pkgName := range packages {
		p, err := pi.Find(pkgName)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "        %q: ", pkgName)
		deps := pi.ResolveDependency(p)
		depPkgNames := enumerable.Map(deps, func(t *Package) string { return t.Name })
		depPkgNames = enumerable.FindAll(depPkgNames, func(s string) bool { _, ok := excludePackages[s]; return !ok })
		depPkgNames = enumerable.Map(depPkgNames, func(v string) string { return fmt.Sprintf("%q", v) })
		sort.Strings(depPkgNames)
		fmt.Fprintf(out, "[%s],\n", strings.Join(depPkgNames, ", "))
	}
	out.WriteString("    }\n")
	out.WriteString("}\n\n")

	out.WriteString(`def deb_pkg(distro, *pkgs):
    all = {}
    for x in pkgs:
        all[x] = None
        for x in package_dependencies[distro][x]:
            all[x] = None
    return ["@%s_%s//:data" % (distro, k) for k in all]`)
	out.WriteString("\n")
	return nil
}

func readConfig(confFile string) (*config, error) {
	f, err := os.Open(confFile)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var conf config
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &conf, nil
}

func debManager(args []string) error {
	var confFile, utilityMacroFile, outFile string
	cmd := &cli.Command{
		Use: "deb-manager OUT_FILE",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return generate(ctx, confFile, utilityMacroFile, outFile)
		},
	}
	cmd.Flags().String("conf", "Config file").Var(&confFile)
	cmd.Flags().String("macro", "Macro file").Var(&utilityMacroFile)
	cmd.Flags().String("out", "Out file").Var(&outFile)

	return cmd.Execute(args)
}

func main() {
	if err := debManager(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
