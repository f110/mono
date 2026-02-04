package config

import "list"

#Secret: {
	vault_mount!: string
	vault_path!: string
	vault_key!: string
} & (
	{
		mount_path: string
		host?: _|_
	} |
	{
		host: string
		mount_path?: _|_
	})

#EventType: "push" | "manual" | "pull_request" | "release"

#Command: "test" | "run" | "build"

#Job: {
	name?:   string
	command!: #Command
	targets: [...string]
	platforms: [...string]
	args?: [...string]
	container?: string
	cpu_limit?:    string
	memory_limit?: string
	event: [...#EventType]
	secrets?: [...#Secret]
	config_name?: string
	schedule?: string
	all_revision?:  bool
	github_status?: bool
	exclusive?: bool
	env?: {
		[string]: string
	}
}

#Job: {
	command: *"test" | "run"
	targets: list.MinItems(1)
	event: list.MinItems(1)
	platforms: list.MinItems(1)
	targets: list.MinItems(1)

	if command == "test" {
		args: list.MaxItems(0)
	}

	if command == "run" {
		targets: list.MaxItems(1)
	}
}

jobs: {
    [string]: #Job
}

jobs: [Name=_]: {
    name: Name
}

output: [for _name, job in jobs {job}]
