jobs: {
	test_all: {
		command: "test"
		targets: [
			"//...",
			"-//containers/zoekt-indexer/...",
			"-//containers/zoekt-webserver/...",
			"-//py/...",
		]
		platforms: ["@rules_go//go/toolchain:linux_amd64"]
		all_revision:  true
		github_status: true
		cpu_limit:     "2000m"
		memory_limit:  "8192Mi"
		event: ["push"]
	}
}
