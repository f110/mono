package macports

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePortfile(t *testing.T) {
	in := `# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4

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

	err := ParsePortfile([]byte(in))
	require.NoError(t, err)
}

func TestTokenize(t *testing.T) {
	in := `# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4


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

	expectTokens := []*PortfileToken{
		{
			Type:  PortfileTokenComment,
			Value: "# -*- coding: utf-8; mode: tcl; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- vim:fenc=utf-8:ft=tcl:et:sw=4:ts=4:sts=4",
		},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "PortSystem"},
		{Type: PortfileTokenIdent, Value: "1.0"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "PortGroup"},
		{Type: PortfileTokenIdent, Value: "golang 1.0"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "go.setup"},
		{Type: PortfileTokenIdent, Value: "github.com/kubecfg/kubecfg 0.26.0 v"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "revision"},
		{Type: PortfileTokenIdent, Value: "0"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "name"},
		{Type: PortfileTokenIdent, Value: "kubecfg"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "homepage"},
		{Type: PortfileTokenIdent, Value: "https://github.com/kubecfg/kubecfg"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "description"},
		{Type: PortfileTokenIdent, Value: "A tool for managing complex enterprise Kubernetes environments as code."},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "long_description"},
		{Type: PortfileTokenIdent, Value: "kubecfg allows you to express the patterns across your infrastructure and reuse these powerful \"templates\" across many services, and then manage those templates as files in version control. The more complex your infrastructure is, the more you will gain from using kubecfg. objects through ownersReferences on them."},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "license"},
		{Type: PortfileTokenIdent, Value: "Apache-2.0"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "checksums"},
		{Type: PortfileTokenIdent, Value: "rmd160  f0dfa68de7f98847399f064aa8930d39483db97e sha256  322ed2b6d4214bafac63ee3d666aa240b077a0949d68bc97e5b6dfc484345b7e size    266525"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "categories"},
		{Type: PortfileTokenIdent, Value: "sysutils"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "platforms"},
		{Type: PortfileTokenIdent, Value: "darwin"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "supported_archs"},
		{Type: PortfileTokenIdent, Value: "x86_64 arm64"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "installs_libs"},
		{Type: PortfileTokenIdent, Value: "no"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "build.cmd"},
		{Type: PortfileTokenIdent, Value: "make"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "build.target"},
		{Type: PortfileTokenIdent, Value: "kubecfg"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "build.post_args"},
		{Type: PortfileTokenIdent, Value: "VERSION=v${version}"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "build.env-delete"},
		{Type: PortfileTokenIdent, Value: "GO111MODULE=off GOPROXY=off"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "destroot"},
		{Type: PortfileTokenLBracket},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenIdent, Value: "xinstall -m 0755 ${worksrcpath}/${name} ${destroot}${prefix}/bin/"},
		{Type: PortfileTokenLineBreak},
		{Type: PortfileTokenRBracket},
	}

	lexer := NewLexer(strings.NewReader(in))
	for i := range expectTokens {
		token, err := lexer.Scan()
		require.NoError(t, err)
		assert.Equalf(t, expectTokens[i].Type, token.Type, "token %d", i)
		assert.Equal(t, expectTokens[i].Value, token.Value)
	}
	_, err := lexer.Scan()
	assert.ErrorIs(t, err, io.EOF)
}
