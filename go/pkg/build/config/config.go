package config

import (
	"fmt"
	"io"
	"log"
	"reflect"

	"go.f110.dev/xerrors"
	"go.starlark.net/starlark"
)

type Config struct {
	Jobs            []*Job
	RepositoryOwner string
	RepositoryName  string
}

type Job struct {
	// Name is a job name
	Name string `attr:"name"`
	// If true, build at each revision
	AllRevision bool   `attr:"all_revision"`
	Command     string `attr:"command"`
	// Limit of CPU
	CPULimit string `attr:"cpu_limit"`
	// Limit of memory
	MemoryLimit  string   `attr:"memory_limit"`
	GitHubStatus bool     `attr:"github_status"`
	Platforms    []string `attr:"platforms"`
	Targets      []string `attr:"targets"`
	// Do not allow parallelized build in this job
	Exclusive bool `attr:"exclusive"`
	// The name of config
	ConfigName string `attr:"config_name"`
	// Job schedule
	Schedule string `attr:"schedule"`

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

			for _, v := range kwargs {
				name := v.Index(0)
				value := v.Index(1)
				switch name.Type() {
				case "string":
					s := name.(starlark.String)

					typ := reflect.TypeOf(job).Elem()
					val := reflect.ValueOf(job).Elem()
					for i := 0; i < typ.NumField(); i++ {
						ft := typ.Field(i)
						if ft.Tag.Get("attr") == s.GoString() {
							fv := val.Field(i)
							if err := setValue(ft.Type, fv, value); err != nil {
								log.Println(err)
							}
							break
						}
					}
				}
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
