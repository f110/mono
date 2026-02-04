_job_common: {
	command: "run"
	platforms: ["@rules_go//go/toolchain:linux_amd64"]
	args: ["--insecure"]
	cpu_limit: "2000m"
	event: ["manual"]
}

jobs: {
	publish_build: _job_common & {
		targets: ["//containers/build:push_build"]
	}
	publish_build_sidecar: _job_common & {
		targets: ["//containers/build:push_sidecar"]
	}
	publish_build_frontend: _job_common & {
		targets: ["//containers/build:push_frontend"]
	}
	publish_build_frontend_dev: _job_common & {
		targets: ["//containers/build:push_frontend_dev"]
	}
}
