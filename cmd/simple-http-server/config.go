package main

import (
	"encoding/json"

	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/ucl"
)

type Config struct {
	Server json.RawMessage `json:"server"`

	servers []*ConfigServer
}

func (c *Config) Servers() []*ConfigServer {
	return c.servers
}

type ConfigServer struct {
	Listen    string `json:"listen"`
	Path      any    `json:"path"`
	AccessLog string `json:"access_log"`

	path []*PathConfig
}

type PathConfig struct {
	Path      string
	Proxy     string
	Root      string
	AccessLog string
}

func readConfigFile(p string) (*Config, error) {
	d, err := ucl.NewFileDecoder(p)
	if err != nil {
		return nil, err
	}

	return readConfig(d)
}

func readConfig(d *ucl.Decoder) (*Config, error) {
	buf, err := d.ToJSON(nil)
	if err != nil {
		return nil, err
	}
	var conf Config
	if err := json.Unmarshal(buf, &conf); err != nil {
		return nil, xerrors.WithStack(err)
	}

	var servers []*ConfigServer
	if err := json.Unmarshal(conf.Server, &servers); err == nil {
		conf.servers = servers
	} else {
		var server ConfigServer
		if err := json.Unmarshal(conf.Server, &server); err != nil {
			return nil, xerrors.WithStack(err)
		}
		conf.servers = append(conf.servers, &server)
	}
	return &conf, nil
}
