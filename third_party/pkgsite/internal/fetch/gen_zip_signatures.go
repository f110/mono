// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

// This program generates zip_signatures.gen.go.

// The program depends on its own generated file. To regenerate from scratch,
// manually edit the generated file, leaving an empty map literal.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"log"
	"os"
	"sort"
	"text/template"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/fetch"
	"golang.org/x/pkgsite/internal/proxy"
)

var (
	verbose = flag.Bool("v", false, "verbose output")
	check   = flag.Bool("check", false, "check signatures of module@version command-line args")
)

// The list of modules whose exact forks will be excluded from processing. The
// second field is the highest (in the semver sense) without a go.mod file;
// versions with go.mod files are handled by the alternative-module logic.
var largeNoMods = []struct {
	modulePath          string
	highestNoModVersion string
}{
	{"github.com/aws/aws-sdk-go", "v1.14.30"},
	{"github.com/kubernetes/kubernetes", "v1.15.0-alpha.0"},
	{"github.com/Azure/azure-sdk-for-go", "v63.2.0"},
	{"github.com/ethereum/go-ethereum", "v1.9.7"},
	{"github.com/moby/moby", "v20.10.8"},
	{"github.com/influxdata/influxdb", "v1.7.9"},
	{"github.com/etcd-io/etcd", "v3.3.25"},
}

const goFile = "zip_signatures.gen.go"

type sig struct {
	Modver    internal.Modver
	Signature string
}

func main() {
	flag.Parse()
	ctx := context.Background()

	prox, err := proxy.New("https://proxy.golang.org", nil)
	if err != nil {
		log.Fatal(err)
	}

	if *check {
		err = checkSignatures(ctx, prox, flag.Args())
	} else {
		err = generateSignatures(ctx, prox)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func generateSignatures(ctx context.Context, prox *proxy.Client) error {
	// Remember all the module versions we've already computed, to avoid doing so again.
	seen := map[internal.Modver]bool{}
	for _, mvs := range fetch.ZipSignatures {
		for _, mv := range mvs {
			seen[mv] = true
		}
	}

	for _, m := range largeNoMods {
		// Get all tagged versions of the module.
		versions, err := prox.Versions(ctx, m.modulePath)
		if err != nil {
			return err
		}
		// Keep versions that are less than or equal to the highest version without a go.mod file.
		var noGoModVersions []string
		for _, v := range versions {
			if semver.Compare(v, m.highestNoModVersion) <= 0 {
				noGoModVersions = append(noGoModVersions, v)
			}
		}
		// Compute the signature for each of those versions.
		for _, v := range noGoModVersions {
			modver := internal.Modver{Path: m.modulePath, Version: v}
			if seen[modver] {
				if *verbose {
					fmt.Printf("%-40s already computed\n", modver)
				}
				continue
			}
			s, err := computeSignature(ctx, prox, modver)
			if err != nil {
				log.Printf("skipping %s: %v", modver, err)
			} else {
				fetch.ZipSignatures[s] = append(fetch.ZipSignatures[s], modver)
			}
		}
	}
	return writeGoFile(goFile)
}

func checkSignatures(ctx context.Context, prox *proxy.Client, args []string) error {
	for _, arg := range args {
		mv, err := internal.ParseModver(arg)
		if err != nil {
			return err
		}
		sig, err := computeSignature(ctx, prox, mv)
		if err != nil {
			return err
		}
		matches := fetch.ZipSignatures[sig]
		fmt.Printf("%s: signature %s matches %v\n", arg, sig, matches)
	}
	return nil
}

func computeSignature(ctx context.Context, prox *proxy.Client, mv internal.Modver) (string, error) {
	start := time.Now()
	zr, err := prox.Zip(ctx, mv.Path, mv.Version)
	if err != nil {
		return "", err
	}
	contentDir, err := fs.Sub(zr, mv.String())
	if err != nil {
		return "", err
	}
	sig, err := fetch.FSSignature(contentDir)
	if err != nil {
		return "", err
	}
	dur := time.Since(start)
	if *verbose {
		fmt.Printf("%-40s %s    %.1fs\n", mv, sig, dur.Seconds())
	}
	return sig, nil
}

// writeGoFile writes ZipSignatures back to the generated file.
func writeGoFile(filename string) error {
	// Convert the ZipSignatures map to a slice of key-value pairs.
	type kv struct {
		Signature string
		Modvers   []internal.Modver
		key       string
	}

	var kvs []kv
	for sig, mvs := range fetch.ZipSignatures {
		kvs = append(kvs, kv{sig, mvs, mvs[0].String()})
	}
	// Sort the slice so that the diffs will show only the changes. Otherwise
	// random map iteration order will result in messy diffs even if no
	// signatures were added.
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].key < kvs[j].key })

	// Execute the template.
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, kvs); err != nil {
		return err
	}
	// Run gofmt.
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(filename, src, 0644)
}

// Template for the generated source file.
// The map value is a []Modver because it's possible for two modules to have the same contents.
// For example, github.com/aws/aws-sdk-go v1.9.0 and v1.9.44 (perhaps due to a bad tag?).
var tmpl = template.Must(template.New("").Parse(`
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by gen_zip_signatures.go; DO NOT EDIT.

package fetch

import "golang.org/x/pkgsite/internal"

var ZipSignatures = map[string][]internal.Modver{
{{range .}}
    "{{.Signature}}": []internal.Modver{
	{{range .Modvers -}}
		{Path: "{{.Path}}", Version: "{{.Version}}"},
	{{- end}}
	},
{{- end}}
}
`))
