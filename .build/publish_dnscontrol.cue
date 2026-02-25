jobs: publish_dnscontrol: {
	command: "run"
	targets: ["//containers/dnscontrol:push"]
	platforms: ["@rules_go//go/toolchain:linux_amd64"]
	args: ["--insecure"]
	cpu_limit: "2000m"
	event: ["manual"]
}
