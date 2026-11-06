jobs: publish_etcdbackup: {
	command: "run"
	targets: ["//containers/etcdbackup:push"]
	platforms: ["@rules_go//go/toolchain:linux_arm64"]
	args: ["--insecure"]
	cpu_limit: "2000m"
	event: ["manual"]
}
