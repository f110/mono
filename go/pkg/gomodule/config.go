package gomodule

import (
	"os"
	"regexp"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

type ModuleSetting struct {
	ModuleName string `yaml:"module_name"`

	match *regexp.Regexp
}

type Config []*ModuleSetting

func ReadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	var conf Config
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	for _, v := range conf {
		re, err := regexp.Compile(v.ModuleName)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		v.match = re
	}

	return conf, nil
}
