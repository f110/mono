package config

import (
	"bytes"
	_ "embed"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"reflect"
	"strings"

	"go.f110.dev/xerrors"
	"go.starlark.net/starlark"
)

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
	Jobs            []*Job
	RepositoryOwner string
	RepositoryName  string
}

func (c *Config) Job(event EventType) []*Job {
	var jobs []*Job
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
	EnvVarName string `attr:"env_name,allowempty"`
	MountPath  string `attr:"mount_path,allowempty"`
	VaultPath  string `attr:"vault_path"`
	VaultKey   string `attr:"vault_key"`
}

var _ starlark.Value = &Secret{}

func (s *Secret) String() string {
	return s.EnvVarName
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

type Job struct {
	// Name is a job name
	Name  string      `attr:"name"`
	Event []EventType `attr:"event"`
	// If true, build at each revision
	AllRevision bool   `attr:"all_revision,allowempty"`
	Command     string `attr:"command"`
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
	Schedule string         `attr:"schedule,allowempty"`
	Secrets  []*Secret      `attr:"secrets,allowempty"`
	Env      map[string]any `attr:"env,allowempty"`

	RepositoryOwner string
	RepositoryName  string
}

func (j *Job) Identification() string {
	return fmt.Sprintf("%s-%s-%s", j.RepositoryOwner, j.RepositoryName, j.Name)
}

func (j *Job) IsValid() error {
	if j.Name == "" {
		return xerrors.New("name is required")
	}

	var keys []string
	requiredField := make(map[string]struct{})
	typ := reflect.TypeOf(j).Elem()
	val := reflect.ValueOf(j).Elem()
	for i := 0; i < typ.NumField(); i++ {
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
		return xerrors.Newf("all mandatory fields are not set at %s: %s", j.Name, strings.Join(k, ", "))
	}

	switch j.Command {
	case "test":
	case "run":
		if len(j.Targets) != 1 {
			return xerrors.New("can't specify multiple targets if the command is run")
		}
	default:
		return xerrors.Newf("%s is not supported command", j.Command)
	}

	if j.Args != nil && j.Command != "run" {
		return xerrors.Newf("specifying argument is not allowed in %s command", j.Command)
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
				for i := 0; i < typ.NumField(); i++ {
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

		config.Jobs = append(config.Jobs, job)
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
			return xerrors.Newf("expect starlark.String field: %T", val)
		}
		fv.SetString(v.GoString())
		return nil
	case reflect.Bool:
		v, ok := val.(starlark.Bool)
		if !ok {
			return xerrors.Newf("expect starlark.Bool field: %T", val)
		}
		fv.SetBool(bool(v))
		return nil
	case reflect.Slice:
		v, ok := val.(*starlark.List)
		if !ok {
			return xerrors.Newf("expect *starlark.List field: %T", val)
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
			return xerrors.Newf("expect *starlark.Dict field: %T", val)
		}
		m := make(map[string]any)
		for _, t := range v.Items() {
			k, ok := t.Index(0).(starlark.String)
			if !ok {
				return xerrors.Newf("the type of the key is not string: %T", t.Index(0))
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
			m[t.Index(0).(starlark.String).GoString()] = t.Index(1).String()
		}
		fv.Set(reflect.ValueOf(m))
		return nil
	}

	return xerrors.Newf("unsupported field type: %s", ft.Kind())
}

func argPairs(obj any) []any {
	var pairs []any
	st := reflect.TypeOf(obj).Elem()
	sv := reflect.ValueOf(obj).Elem()
	for i := 0; i < st.NumField(); i++ {
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
