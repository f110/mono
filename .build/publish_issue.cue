_job_common: {
	command: "run"
	platforms: ["@rules_go//go/toolchain:linux_amd64"]
	args: ["--insecure"]
	cpu_limit: "2000m"
	event: ["manual"]
}

jobs: {
	publish_issue_server: _job_common & {
		targets: ["//containers/issue:push_server"]
	}
	publish_issue_frontend: _job_common & {
		targets: ["//containers/issue:push_frontend"]
	}
}
