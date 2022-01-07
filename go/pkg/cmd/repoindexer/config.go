package repoindexer

import (
	"io"
	"os"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"

	"go.f110.dev/mono/go/pkg/regexp/regexputil"
)

type Config struct {
	RefreshSchedule string  `yaml:"refresh_schedule,omitempty"`
	Rules           []*Rule `yaml:"rules,omitempty"`
}

type Rule struct {
	Owner    string   `yaml:"owner,omitempty"`
	Name     string   `yaml:"name,omitempty"`
	Query    string   `yaml:"query,omitempty"`
	Branches []string `yaml:"branches,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`

	URLReplace string `yaml:"url_replace"`

	DisableVendoring bool `yaml:"disable_vendoring"`

	urlReplaceRule *replaceRule
}

func ReadConfigFile(p string) (*Config, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	defer f.Close()

	config, err := ReadConfig(f)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return config, nil
}

func ReadConfig(r io.Reader) (*Config, error) {
	config := &Config{}
	if err := yaml.NewDecoder(r).Decode(&config); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	for _, v := range config.Rules {
		if v.URLReplace == "" {
			continue
		}

		regexpLiteral, err := regexputil.ParseRegexpLiteral(v.URLReplace)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		v.urlReplaceRule = &replaceRule{re: regexpLiteral.Match, replace: regexpLiteral.Replace}
	}

	return config, nil
}
