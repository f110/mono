package main

type Config struct {
	Server []*ConfigServer `json:"server"`
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
