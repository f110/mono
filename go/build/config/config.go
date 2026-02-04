package config

import (
	"bytes"
	_ "embed"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"reflect"
	"strings"

	"go.f110.dev/xerrors"
	"go.starlark.net/starlark"
)

func init() {
	gob.Register(&Secret{})
	gob.Register(&RegistrySecret{})
}

//go:embed config.star
var configModule string

type EventType string

const (
	EventPush        EventType = "push"
	EventPullRequest EventType = "pull_request"
	EventRelease     EventType = "release"
	EventManual      EventType = "manual"
)

type Config struct {
	Jobs            []*JobV2
	BazelVersion    string
	RepositoryOwner string
	RepositoryName  string
}

func (c *Config) Job(event EventType) []*JobV2 {
	var jobs []*JobV2
	for _, job := range c.Jobs {
		for _, e := range job.Event {
			if e == event {
				jobs = append(jobs, job)
				break
			}
		}
	}

	return jobs
}

type Secret struct {
	MountPath  string `attr:"mount_path" yaml:"mount_path,omitempty" json:"mount_path,omitempty"`
	Host       string `yaml:"host,omitempty" json:"host,omitempty"`
	VaultMount string `attr:"vault_mount" yaml:"vault_mount" json:"vault_mount"`
	VaultPath  string `attr:"vault_path" yaml:"vault_path" json:"vault_path"`
	VaultKey   string `attr:"vault_key" yaml:"vault_key" json:"vault_key"`
}

var _ starlark.Value = (*Secret)(nil)

func (s *Secret) String() string {
	return fmt.Sprintf("%s/%s:%s", s.VaultMount, s.VaultPath, s.VaultKey)
}

func (s *Secret) Type() string {
	return "secret"
}

func (s *Secret) Freeze() {}

func (s *Secret) Truth() starlark.Bool {
	return true
}

func (s *Secret) Hash() (uint32, error) {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(s); err != nil {
		return 0, err
	}
	return crc32.ChecksumIEEE(buf.Bytes()), nil
}

type RegistrySecret struct {
	Host       string `attr:"host"`
	VaultMount string `attr:"vault_mount"`
	VaultPath  string `attr:"vault_path"`
	VaultKey   string `attr:"vault_key"`
}

var _ starlark.Value = (*RegistrySecret)(nil)

func (s *RegistrySecret) String() string {
	return fmt.Sprintf("%s/%s:%s", s.VaultMount, s.VaultPath, s.VaultKey)
}

func (s *RegistrySecret) Type() string {
	return "secret"
}

func (s *RegistrySecret) Freeze() {}

func (s *RegistrySecret) Truth() starlark.Bool {
	return true
}

func (s *RegistrySecret) Hash() (uint32, error) {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(s); err != nil {
		return 0, err
	}
	return crc32.ChecksumIEEE(buf.Bytes()), nil
}

type Job struct {
	// Name is a job name
	Name  string      `attr:"name"`
	Event []EventType `attr:"event"`
	// If true, build at each revision
	AllRevision bool   `attr:"all_revision,allowempty"`
	Command     string `attr:"command"`
	Container   string `attr:"container,allowempty"`
	// Limit of CPU
	CPULimit string `attr:"cpu_limit,allowempty"`
	// Limit of memory
	MemoryLimit  string   `attr:"memory_limit,allowempty"`
	GitHubStatus bool     `attr:"github_status,allowempty"`
	Platforms    []string `attr:"platforms"`
	Targets      []string `attr:"targets"`
	Args         []string `attr:"args,allowempty"`
	// Do not allow parallelized build in this job
	Exclusive bool `attr:"exclusive,allowempty"`
	// The name of config
	ConfigName string `attr:"config_name,allowempty"`
	// Job schedule
	Schedule string           `attr:"schedule,allowempty"`
	Secrets  []starlark.Value `attr:"secrets,allowempty"`
	Env      map[string]any   `attr:"env,allowempty"`

	RepositoryOwner string
	RepositoryName  string
}

func (j *Job) ToV2() *JobV2 {
	var secrets []*Secret
	for _, s := range j.Secrets {
		switch s := s.(type) {
		case *Secret:
			secrets = append(secrets, &Secret{
				MountPath:  s.MountPath,
				VaultMount: s.VaultMount,
				VaultPath:  s.VaultPath,
				VaultKey:   s.VaultKey,
			})
		case *RegistrySecret:
			secrets = append(secrets, &Secret{
				Host:       s.Host,
				VaultMount: s.VaultMount,
				VaultPath:  s.VaultPath,
				VaultKey:   s.VaultKey,
			})
		}
	}
	return &JobV2{
		Name:            j.Name,
		Event:           j.Event,
		AllRevision:     j.AllRevision,
		Command:         j.Command,
		Container:       j.Container,
		CPULimit:        j.CPULimit,
		MemoryLimit:     j.MemoryLimit,
		GitHubStatus:    j.GitHubStatus,
		Platforms:       j.Platforms,
		Targets:         j.Targets,
		Args:            j.Args,
		Exclusive:       j.Exclusive,
		ConfigName:      j.ConfigName,
		Schedule:        j.Schedule,
		Env:             j.Env,
		Secrets:         secrets,
		RepositoryOwner: j.RepositoryOwner,
		RepositoryName:  j.RepositoryName,
	}
}

func (j *Job) Copy() *Job {
	n := &Job{}
	*n = *j

	return n
}

func (j *Job) Identification() string {
	return fmt.Sprintf("%s-%s-%s", j.RepositoryOwner, j.RepositoryName, j.Name)
}

func (j *Job) IsValid() error {
	if j.Name == "" {
		return xerrors.Define("name is required").WithStack()
	}

	var keys []string
	requiredField := make(map[string]struct{})
	typ := reflect.TypeOf(j).Elem()
	val := reflect.ValueOf(j).Elem()
	for i := range typ.NumField() {
		ft := typ.Field(i)
		if v := ft.Tag.Get("attr"); v != "" {
			t := strings.Split(v, ",")
			if len(t) == 2 && t[1] == "allowempty" {
				continue
			}
			requiredField[t[0]] = struct{}{}

			if !val.Field(i).IsZero() {
				keys = append(keys, t[0])
			}
		}
	}
	for _, v := range keys {
		delete(requiredField, v)
	}
	if len(requiredField) > 0 {
		k := make([]string, 0, len(requiredField))
		for v := range requiredField {
			k = append(k, v)
		}
		return xerrors.Definef("all mandatory fields are not set at %s: %s", j.Name, strings.Join(k, ", ")).WithStack()
	}

	switch j.Command {
	case "test":
	case "run":
		if len(j.Targets) != 1 {
			return xerrors.Define("can't specify multiple targets if the command is run").WithStack()
		}
	default:
		return xerrors.Definef("%s is not supported command", j.Command).WithStack()
	}

	if j.Args != nil && j.Command != "run" {
		return xerrors.Definef("specifying argument is not allowed in %s command", j.Command).WithStack()
	}
	return nil
}

func MarshalJob(j *JobV2) ([]byte, error) {
	j.SchemaVersion = "2"
	b, err := json.Marshal(j)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return b, nil
}

func UnmarshalJobV2(b []byte, j *JobV2) error {
	if len(b) == 0 || b[0] != '{' {
		return xerrors.New("this json is not JobV2, probably Job")
	}
	sv := &struct {
		SchemaVersion string `json:"schema_version"`
	}{}
	if err := json.Unmarshal(b, sv); err != nil {
		return xerrors.WithStack(err)
	}
	if sv.SchemaVersion != "2" {
		return xerrors.New("this json is not JobV2")
	}
	if err := json.Unmarshal(b, j); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func UnmarshalJob(b []byte, j *Job) error {
	if len(b) > 0 && b[0] == '{' {
		sv := &struct {
			SchemaVersion string `json:"schema_version"`
		}{}
		if err := json.Unmarshal(b, sv); err != nil {
			return xerrors.WithStack(err)
		}
		if sv.SchemaVersion != "" {
			return xerrors.New("this json is not Job, probably JobV2")
		}

		if err := json.Unmarshal(b, j); err != nil {
			return xerrors.WithStack(err)
		}
		return nil
	}

	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(j); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

func Read(r io.Reader, owner, repo string) (*Config, error) {
	config := &Config{RepositoryOwner: owner, RepositoryName: repo}

	thread := &starlark.Thread{
		Name:  "example",
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	mod, err := starlark.ExecFile(thread, "", strings.NewReader(configModule), nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	mod["job"] = starlark.NewBuiltin("job", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		job := &Job{RepositoryOwner: owner, RepositoryName: repo}

		var keys []string
		for _, v := range kwargs {
			name := v.Index(0)
			value := v.Index(1)
			keys = append(keys, name.(starlark.String).GoString())

			switch name.Type() {
			case "string":
				s := name.(starlark.String)

				typ := reflect.TypeOf(job).Elem()
				val := reflect.ValueOf(job).Elem()
				for i := range typ.NumField() {
					ft := typ.Field(i)
					if v := ft.Tag.Get("attr"); v != "" {
						t := strings.Split(v, ",")
						if t[0] == s.GoString() {
							fv := val.Field(i)
							if err := setValue(ft.Type, fv, value); err != nil {
								log.Println(err)
							}
							break
						}
					}
				}
			}
		}
		if err := job.IsValid(); err != nil {
			return nil, err
		}

		config.Jobs = append(config.Jobs, job.ToV2())
		return starlark.String(""), nil
	})
	mod["secret"] = starlark.NewBuiltin("secret", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		s := &Secret{}
		err := starlark.UnpackArgs(fn.Name(), args, kwargs, argPairs(s)...)
		if err != nil {
			return nil, err
		}
		return s, nil
	})
	mod["registry_secret"] = starlark.NewBuiltin("registry_secret", func(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		s := &RegistrySecret{}
		err := starlark.UnpackArgs(fn.Name(), args, kwargs, argPairs(s)...)
		if err != nil {
			return nil, err
		}
		return s, nil
	})

	_, err = starlark.ExecFile(thread, "", r, mod)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return config, nil
}

func setValue(ft reflect.Type, fv reflect.Value, val starlark.Value) error {
	switch ft.Kind() {
	case reflect.String:
		v, ok := val.(starlark.String)
		if !ok {
			return xerrors.Definef("expect starlark.String field: %T", val).WithStack()
		}
		fv.SetString(v.GoString())
		return nil
	case reflect.Bool:
		v, ok := val.(starlark.Bool)
		if !ok {
			return xerrors.Definef("expect starlark.Bool field: %T", val).WithStack()
		}
		fv.SetBool(bool(v))
		return nil
	case reflect.Slice:
		v, ok := val.(*starlark.List)
		if !ok {
			return xerrors.Definef("expect *starlark.List field: %T", val).WithStack()
		}
		if v.Len() == 0 {
			return nil
		}

		iter := v.Iterate()
		var item starlark.Value
		for iter.Next(&item) {
			switch ft.Elem().Kind() {
			case reflect.String, reflect.Bool:
				newValue := reflect.New(ft.Elem()).Elem()
				if err := setValue(ft.Elem(), newValue, item); err != nil {
					return err
				}
				fv.Set(reflect.Append(fv, newValue))
			default:
				fv.Set(reflect.Append(fv, reflect.ValueOf(item)))
			}
		}
		return nil
	case reflect.Map:
		v, ok := val.(*starlark.Dict)
		if !ok {
			return xerrors.Definef("expect *starlark.Dict field: %T", val).WithStack()
		}
		m := make(map[string]any)
		for _, t := range v.Items() {
			k, ok := t.Index(0).(starlark.String)
			if !ok {
				return xerrors.Definef("the type of the key is not string: %T", t.Index(0)).WithStack()
			}
			key := k.GoString()

			switch v := t.Index(1).(type) {
			case starlark.String:
				m[key] = v.GoString()
			case starlark.Int:
				m[key] = v.String()
			case *Secret:
				m[key] = v
			}
		}
		fv.Set(reflect.ValueOf(m))
		return nil
	}

	return xerrors.Definef("unsupported field type: %s", ft.Kind()).WithStack()
}

func argPairs(obj any) []any {
	var pairs []any
	st := reflect.TypeOf(obj).Elem()
	sv := reflect.ValueOf(obj).Elem()
	for i := range st.NumField() {
		ft := st.Field(i)
		starTag := ft.Tag.Get("attr")
		if starTag == "" {
			continue
		}

		keyName := starTag
		var optional bool
		if strings.IndexRune(keyName, ',') > 0 {
			s := strings.Split(starTag, ",")
			keyName = s[0]
			for _, v := range s[1:] {
				if v == "allowempty" {
					optional = true
				}
			}
		}
		if optional {
			keyName = keyName + "?"
		}

		fv := sv.Field(i)
		pairs = append(pairs, keyName, fv.Addr().Interface())
	}

	return pairs
}

type JobV2 struct {
	SchemaVersion string `yaml:"schema_version,omitempty" json:"schema_version,omitempty"`

	// Name is a job name
	Name  string      `yaml:"name" json:"name"`
	Event []EventType `yaml:"event" json:"event"`
	// If true, build at each revision
	AllRevision bool   `yaml:"all_revision,omitempty" json:"all_revision,omitempty"`
	Command     string `yaml:"command" json:"command,omitempty"`
	Container   string `yaml:"container,omitempty" json:"container,omitempty"`
	// Limit of CPU
	CPULimit string `yaml:"cpu_limit,omitempty" json:"cpu_limit,omitempty"`
	// Limit of memory
	MemoryLimit  string   `yaml:"memory_limit,omitempty" json:"memory_limit,omitempty"`
	GitHubStatus bool     `yaml:"github_status,omitempty" json:"github_status,omitempty"`
	Platforms    []string `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	Targets      []string `yaml:"targets,omitempty" json:"targets,omitempty"`
	Args         []string `yaml:"args,omitempty" json:"args,omitempty"`
	// Do not allow parallelized build in this job
	Exclusive bool `yaml:"exclusive,omitempty" json:"exclusive,omitempty"`
	// The name of config
	ConfigName string `yaml:"config_name,omitempty" json:"config_name,omitempty"`
	// Job schedule
	Schedule string         `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Secrets  []*Secret      `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Env      map[string]any `yaml:"env,omitempty" json:"env,omitempty"`

	RepositoryOwner string `yaml:"-" json:"-"`
	RepositoryName  string `yaml:"-" json:"-"`
}

func (j *JobV2) Identification() string {
	return fmt.Sprintf("%s-%s-%s", j.RepositoryOwner, j.RepositoryName, j.Name)
}
