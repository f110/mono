package config

import (
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"

	"go.f110.dev/xerrors"
	"go.starlark.net/starlark"
)

type Config struct {
	Jobs            []*Job
	RepositoryOwner string
	RepositoryName  string
}

func (c *Config) Job(event string) []*Job {
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

type Job struct {
	// Name is a job name
	Name  string   `attr:"name"`
	Event []string `attr:"event"`
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
	// Do not allow parallelized build in this job
	Exclusive bool `attr:"exclusive,allowempty"`
	// The name of config
	ConfigName string `attr:"config_name,allowempty"`
	// Job schedule
	Schedule string `attr:"schedule,allowempty"`

	RepositoryOwner string
	RepositoryName  string
}

func (j *Job) Identification() string {
	return fmt.Sprintf("%s-%s-%s", j.RepositoryOwner, j.RepositoryName, j.Name)
}

func Read(r io.Reader, owner, repo string) (*Config, error) {
	config := &Config{RepositoryOwner: owner, RepositoryName: repo}
	predeclared := starlark.StringDict{
		"job": starlark.NewBuiltin("job", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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

			requiredField := make(map[string]struct{})
			typ := reflect.TypeOf(job).Elem()
			for i := 0; i < typ.NumField(); i++ {
				ft := typ.Field(i)
				if v := ft.Tag.Get("attr"); v != "" {
					t := strings.Split(v, ",")
					if len(t) == 2 && t[1] == "allowempty" {
						continue
					}
					requiredField[t[0]] = struct{}{}
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
				return nil, xerrors.Newf("all mandatory fields are not set: %s", strings.Join(k, ", "))
			}

			config.Jobs = append(config.Jobs, job)
			return starlark.String(""), nil
		}),
	}

	thread := &starlark.Thread{
		Name:  "example",
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	_, err := starlark.ExecFile(thread, "", r, predeclared)
	if err != nil {
		return nil, err
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

		iter := v.Iterate()
		var item starlark.Value
		for iter.Next(&item) {
			newValue := reflect.New(ft.Elem()).Elem()
			if err := setValue(ft.Elem(), newValue, item); err != nil {
				return err
			}
			fv.Set(reflect.Append(fv, newValue))
		}
		return nil
	}

	return xerrors.Newf("unsupported field type: %s", ft.Kind())
}
