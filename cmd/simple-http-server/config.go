package main

type Config struct {
	Server []*ConfigServer `json:"server"`
}

type ConfigServer struct {
	Listen string `json:"listen"`
	Path   any    `json:"path"`

	path []*PathConfig
}

type PathConfig struct {
	Path  string
	Proxy string
	Root  string
}
