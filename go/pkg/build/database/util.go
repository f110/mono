package database

import (
	"strings"

	"go.f110.dev/mono/go/pkg/build/job"
)

func (e *Job) Into(j *job.Job) {
	e.Name = j.Name
	e.Command = j.Command
	e.Target = j.Target
	e.Targets = strings.Join(j.Targets, "\n")
	e.AllRevision = j.AllRevision
	e.GithubStatus = j.GithubStatus
	e.CpuLimit = j.CPULimit
	e.MemoryLimit = j.MemoryLimit
	e.Exclusive = j.Exclusive
	e.ConfigName = j.ConfigName
	e.Sync = true
	e.JobType = j.Type
	e.Schedule = j.Schedule
}
