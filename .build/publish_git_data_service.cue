jobs: publish_git_data_service: {
	command: "run"
	targets: ["//containers/git-data-service:push"]
	platforms: ["@rules_go//go/toolchain:linux_amd64"]
	args: ["--insecure"]
	cpu_limit: "2000m"
	event: ["manual"]
}
