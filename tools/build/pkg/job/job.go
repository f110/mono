package job

import (
	"go.uber.org/zap"
)

const (
	TypeCommit  = "commit"
	TypeRelease = "release"
)

type Job struct {
	Package      string `attr:"package"`
	Name         string `attr:"name"`
	Target       string `attr:"target"`
	Targets      string `attr:"targets"`
	Command      string `attr:"command"`
	AllRevision  bool   `attr:"all_revision"`
	GithubStatus bool   `attr:"github_status"`
	CPULimit     string `attr:"cpu_limit"`
	MemoryLimit  string `attr:"memory_limit"`
	Exclusive    bool   `attr:"exclusive"`
	ConfigName   string `attr:"config_name"`
	Type         string `attr:"type"`
	Schedule     string `attr:"schedule"`
}

func (j *Job) ZapFields() []zap.Field {
	return []zap.Field{
		zap.String("package", j.Package),
		zap.String("name", j.Name),
		zap.String("command", j.Command),
		zap.String("target", j.Target),
		zap.Bool("all_revision", j.AllRevision),
		zap.Bool("github_status", j.GithubStatus),
		zap.String("schedule", j.Schedule),
	}
}
