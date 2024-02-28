package gomodule

import (
	"os"
	"regexp"

	"go.f110.dev/xerrors"
	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/regexp/regexputil"
)

type ModuleSetting struct {
	ModuleName string `yaml:"module_name"`
	URLReplace string `yaml:"url_replace"`

	match         *regexp.Regexp
	replaceRegexp *regexputil.RegexpLiteral
}

type Config []*ModuleSetting

func ReadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var conf Config
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, v := range conf {
		re, err := regexp.Compile(v.ModuleName)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		v.match = re

		if v.URLReplace != "" {
			regexpLiteral, err := regexputil.ParseRegexpLiteral(v.URLReplace)
			if err != nil {
				return nil, xerrors.WithStack(err)
			}
			v.replaceRegexp = regexpLiteral
		}
	}

	return conf, nil
}
