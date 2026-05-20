package job

const (
	TypeCommit  = "commit"
	TypeRelease = "release"
)

type Job struct {
	Package      string   `attr:"package"`
	Name         string   `attr:"name"`
	Target       string   `attr:"target"`
	Targets      []string `attr:"targets"`
	Platforms    []string `attr:"platforms"`
	Command      string   `attr:"command"`
	AllRevision  bool     `attr:"all_revision"`
	GithubStatus bool     `attr:"github_status"`
	CPULimit     string   `attr:"cpu_limit"`
	MemoryLimit  string   `attr:"memory_limit"`
	Exclusive    bool     `attr:"exclusive"`
	ConfigName   string   `attr:"config_name"`
	Type         string   `attr:"type"`
	Schedule     string   `attr:"schedule"`
}
