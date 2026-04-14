package macports

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.f110.dev/mono/go/enumerable"
)

var portfile = `# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4


PortSystem 1.0
PortGroup  golang 1.0

go.setup   github.com/kubecfg/kubecfg 0.26.0 v
revision   0

name             kubecfg
homepage         https://github.com/kubecfg/kubecfg
description      A tool for managing complex enterprise Kubernetes environments as code.
long_description kubecfg allows you to express the patterns across your infrastructure and \
    reuse these powerful "templates" across many services, and then manage those templates \
    as files in version control. The more complex your infrastructure is, the more you will \
    gain from using kubecfg. objects through ownersReferences on them.
license          Apache-2.0
maintainers      {gmail.com:fmhrit @f110}

checksums           rmd160  f0dfa68de7f98847399f064aa8930d39483db97e \
                    sha256  322ed2b6d4214bafac63ee3d666aa240b077a0949d68bc97e5b6dfc484345b7e \
                    size    266525

categories      sysutils
platforms       darwin
supported_archs x86_64 arm64
installs_libs   no

build.cmd        make
build.target     kubecfg
build.post_args  VERSION=v${version}
build.env-delete GO111MODULE=off GOPROXY=off

destroot {
    xinstall -m 0755 ${worksrcpath}/${name} ${destroot}${prefix}/bin/
}`

func TestParsePortfile(t *testing.T) {
	port, err := ParsePortfile(strings.NewReader(portfile))
	require.NoError(t, err)
	require.NotNil(t, port)

	assert.Equal(t, "1.0", port.PortSystem)
	assert.Equal(t, "kubecfg", port.Name)
	assert.Equal(t, "https://github.com/kubecfg/kubecfg", port.Homepage)
	assert.Equal(t, "A tool for managing complex enterprise Kubernetes environments as code.", port.Description)
	assert.Equal(t,
		"kubecfg allows you to express the patterns across your infrastructure and reuse these powerful \"templates\" across many services, and then manage those templates as files in version control. The more complex your infrastructure is, the more you will gain from using kubecfg. objects through ownersReferences on them.",
		port.LongDescription,
	)
	assert.Equal(t, "Apache-2.0", port.License)

	assert.Equal(t, "f0dfa68de7f98847399f064aa8930d39483db97e", port.Checksum["rmd160"])
	assert.Equal(t, "322ed2b6d4214bafac63ee3d666aa240b077a0949d68bc97e5b6dfc484345b7e", port.Checksum["sha256"])
	assert.Equal(t, int64(266525), port.Size)

	assert.Equal(t, "golang 1.0", port.Attrs["PortGroup"][0])
	assert.Equal(t, "github.com/kubecfg/kubecfg 0.26.0 v", port.Attrs["go.setup"][0])
	assert.Equal(t, "0", port.Attrs["revision"][0])
	assert.Equal(t, "make", port.Attrs["build.cmd"][0])
	assert.Equal(t, "kubecfg", port.Attrs["build.target"][0])
	assert.Equal(t, "VERSION=v${version}", port.Attrs["build.post_args"][0])
	assert.Equal(t, "GO111MODULE=off GOPROXY=off", port.Attrs["build.env-delete"][0])
	assert.Equal(t, "sysutils", port.Attrs["categories"][0])
	assert.Equal(t, "darwin", port.Attrs["platforms"][0])
	assert.Equal(t, "x86_64 arm64", port.Attrs["supported_archs"][0])
	assert.Equal(t, "no", port.Attrs["installs_libs"][0])
}

func TestParseAsTokens(t *testing.T) {
	tokens, err := ParseAsTokens(strings.NewReader(portfile))
	require.NoError(t, err)
	assert.Len(t, tokens, 85)

	assert.Len(t, enumerable.FindAll(tokens, func(token *PortfileToken) bool { return token.Type == PortfileTokenLineBreak }), 35)
}

func TestTokenize(t *testing.T) {
	cases := []struct {
		In           string
		ExpectTokens []*PortfileToken
	}{
		{
			In: portfile,
			ExpectTokens: []*PortfileToken{
				{
					Type:  PortfileTokenComment,
					Value: "# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4",
				},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "PortSystem"},
				{Type: PortfileTokenIdent, Value: "1.0", StartPos: 11},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "PortGroup"},
				{Type: PortfileTokenIdent, Value: "golang 1.0", StartPos: 11},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "go.setup"},
				{Type: PortfileTokenIdent, Value: "github.com/kubecfg/kubecfg 0.26.0 v", StartPos: 11},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "revision"},
				{Type: PortfileTokenIdent, Value: "0", StartPos: 11},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "name"},
				{Type: PortfileTokenIdent, Value: "kubecfg", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "homepage"},
				{Type: PortfileTokenIdent, Value: "https://github.com/kubecfg/kubecfg", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "description"},
				{Type: PortfileTokenIdent, Value: "A tool for managing complex enterprise Kubernetes environments as code.", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "long_description"},
				{Type: PortfileTokenIdent, StartPos: 17, Value: "kubecfg allows you to express the patterns across your infrastructure and \\"},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, StartPos: 4, Value: "reuse these powerful \"templates\" across many services, and then manage those templates \\"},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, StartPos: 4, Value: "as files in version control. The more complex your infrastructure is, the more you will \\"},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, StartPos: 4, Value: "gain from using kubecfg. objects through ownersReferences on them."},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "license"},
				{Type: PortfileTokenIdent, Value: "Apache-2.0", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "maintainers"},
				{Type: PortfileTokenLBracket, StartPos: 17},
				{Type: PortfileTokenIdent, Value: "gmail.com:fmhrit @f110", StartPos: 18},
				{Type: PortfileTokenRBracket, StartPos: 40},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "checksums"},
				{Type: PortfileTokenIdent, StartPos: 20, Value: "rmd160  f0dfa68de7f98847399f064aa8930d39483db97e \\"},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, StartPos: 20, Value: "sha256  322ed2b6d4214bafac63ee3d666aa240b077a0949d68bc97e5b6dfc484345b7e \\"},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, StartPos: 20, Value: "size    266525"},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "categories"},
				{Type: PortfileTokenIdent, Value: "sysutils", StartPos: 16},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "platforms"},
				{Type: PortfileTokenIdent, Value: "darwin", StartPos: 16},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "supported_archs"},
				{Type: PortfileTokenIdent, Value: "x86_64 arm64", StartPos: 16},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "installs_libs"},
				{Type: PortfileTokenIdent, Value: "no", StartPos: 16},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "build.cmd"},
				{Type: PortfileTokenIdent, Value: "make", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "build.target"},
				{Type: PortfileTokenIdent, Value: "kubecfg", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "build.post_args"},
				{Type: PortfileTokenIdent, Value: "VERSION=v${version}", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "build.env-delete"},
				{Type: PortfileTokenIdent, Value: "GO111MODULE=off GOPROXY=off", StartPos: 17},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "destroot"},
				{Type: PortfileTokenLBracket, StartPos: 9},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "xinstall -m 0755 ${worksrcpath}/${name} ${destroot}${prefix}/bin/", StartPos: 4},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenRBracket, StartPos: 0},
			},
		},
		{
			In: `maintainers         {gmail.com:fmhrit @f110} \
                    openmaintainer`,
			ExpectTokens: []*PortfileToken{
				{Type: PortfileTokenIdent, Value: "maintainers", StartPos: 0},
				{Type: PortfileTokenLBracket, StartPos: 20},
				{Type: PortfileTokenIdent, Value: "gmail.com:fmhrit @f110", StartPos: 21},
				{Type: PortfileTokenRBracket, StartPos: 43},
				{Type: PortfileTokenIdent, Value: "\\", StartPos: 45},
				{Type: PortfileTokenLineBreak},
				{Type: PortfileTokenIdent, Value: "openmaintainer", StartPos: 20},
			},
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tc.In))
			for i := range tc.ExpectTokens {
				token, err := lexer.Scan()
				require.NoError(t, err)
				assert.Equalf(t, tc.ExpectTokens[i].Type, token.Type, "token %d", i)
				assert.Equalf(t, tc.ExpectTokens[i].Value, token.Value, "token %d", i)
				assert.Equalf(t, tc.ExpectTokens[i].StartPos, token.StartPos, "token %d", i)
			}
			_, err := lexer.Scan()
			assert.ErrorIs(t, err, io.EOF)
		})
	}
}

func TestPortfileToken_String(t *testing.T) {
	token := &PortfileToken{Type: PortfileTokenIdent, Value: "1.0", StartPos: 11}
	assert.Equal(t, "           1.0", token.String())
}
