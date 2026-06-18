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

#EventType: "push" | "manual" | "pull_request" | "release" | "external_release"

#Command: "test" | "run" | "build"

#ExternalReleaseSource: {
	provider:            "github"
	repo!:               =~"^[^/]+/[^/]+$"
	kind:                "release" | "tag" | *"release"
	tag_pattern?:        string
	include_prerelease?: bool | *false
}

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
	cache_test_results?: bool
	env?: {
		[string]: string
	}
	external_source?: #ExternalReleaseSource
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
		// cache_test_results is only meaningful for the test command.
		cache_test_results?: _|_
	}

	if list.Contains(event, "external_release") {
		external_source: #ExternalReleaseSource
	}
}

jobs: {
    [string]: #Job
}

jobs: [Name=_]: {
    name: Name
}

output: [for _name, job in jobs {job}]
