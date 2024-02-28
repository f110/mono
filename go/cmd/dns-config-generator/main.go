package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"go.f110.dev/xerrors"
	"gopkg.in/yaml.v3"
)

type ARecord struct {
	Name  string   `yaml:"name"`
	Addr  string   `yaml:"addr"`
	Addrs []string `yaml:"addrs,omitempty"`
	TTL   string   `yaml:"ttl,omitempty"`
}

type CNAMERecord struct {
	Name   string `yaml:"name"`
	Target string `yaml:"target"`
}

type Ignore struct {
	Name string `yaml:"name"`
}

type Record struct {
	A      *ARecord     `yaml:"A,omitempty"`
	CNAME  *CNAMERecord `yaml:"CNAME,omitempty"`
	Ignore *Ignore      `yaml:"IGNORE,omitempty"`
}

type Config struct {
	Domain        string    `yaml:"domain"`
	DefaultTTL    string    `yaml:"default_ttl,omitempty"`
	NameserverTTL string    `yaml:"nameserver_ttl,omitempty"`
	Records       []*Record `yaml:"records"`
}

func ReadConfig(p string) ([]Config, error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	conf := make([]Config, 0)
	if err := yaml.Unmarshal(b, &conf); err != nil {
		return nil, xerrors.WithStack(err)
	}

	return conf, nil
}

func (c *Config) ToDNSControl() string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "D('%s', REG_NONE, DnsProvider(DNS_GCLOUD),\n", c.Domain)

	lines := make([]string, 0)
	if c.NameserverTTL != "" {
		lines = append(lines, fmt.Sprintf("NAMESERVER_TTL('%s')", c.NameserverTTL))
	}
	if c.DefaultTTL != "" {
		lines = append(lines, fmt.Sprintf("DefaultTTL('%s')", c.DefaultTTL))
	}
	for _, r := range c.Records {
		switch {
		case r.A != nil:
			var addrs []string
			if r.A.Addrs != nil {
				addrs = r.A.Addrs
			} else {
				addrs = []string{r.A.Addr}
			}

			for _, v := range addrs {
				if r.A.TTL != "" {
					lines = append(lines, fmt.Sprintf("A('%s', '%s', TTL('%s'))", r.A.Name, v, r.A.TTL))
				} else {
					lines = append(lines, fmt.Sprintf("A('%s', '%s')", r.A.Name, v))
				}
			}
		case r.CNAME != nil:
			lines = append(lines, fmt.Sprintf("CNAME('%s', '%s')", r.CNAME.Name, r.CNAME.Target))
		case r.Ignore != nil:
			lines = append(lines, fmt.Sprintf("IGNORE('%s')", r.Ignore.Name))
		}
	}

	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	buf.WriteString(strings.Join(lines, ",\n"))
	buf.WriteString("\n);\n")

	return buf.String()
}

func main() {
	f := os.Args[1]
	conf, err := ReadConfig(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}

	fmt.Println(`var REG_NONE = NewRegistrar('none', 'NONE');`)
	fmt.Println(`var DNS_GCLOUD = NewDnsProvider("gcloud", "GCLOUD");`)
	fmt.Println("")
	for _, v := range conf {
		fmt.Print(v.ToDNSControl())
	}
}
