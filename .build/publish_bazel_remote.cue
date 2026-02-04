jobs: publish_bazel_remote: {
	command: "run"
	targets: ["//containers/bazel-remote:push"]
	platforms: ["@rules_go//go/toolchain:linux_amd64"]
	args: ["--insecure"]
	cpu_limit: "2000m"
	event: ["manual"]
}
