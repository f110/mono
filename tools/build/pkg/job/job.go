package job

type Job struct {
	Package      string `attr:"package"`
	Target       string `attr:"target"`
	Targets      string `attr:"targets"`
	Command      string `attr:"command"`
	AllRevision  bool   `attr:"all_revision"`
	GithubStatus bool   `attr:"github_status"`
}
