jobs: publish_simple_http_server: {
	command: "run"
	targets: ["//containers/simple-http-server:push"]
	platforms: ["@rules_go//go/toolchain:linux_amd64"]
	args: ["--insecure"]
	cpu_limit: "2000m"
	event: ["manual"]
}
