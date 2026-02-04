jobs: mirror_bazel: {
	command: "run"
	targets: ["//cmd/rotarypress"]
	args: [
		"--rules-macro-file=$(WORKSPACE)/rules_dependencies.bzl",
		"--bucket=mirror",
		"--endpoint=http://incluster.storage.svc.cluster.local:9000",
		"--region=US",
		"--access-key=I4e91N6IGSeJfxsq",
		"--secret-access-key-file=/var/vault/globemaster/storage/token/secretkey",
		"--bazel",
	]
	platforms: ["@rules_go//go/toolchain:linux_amd64"]
	secrets: [
		{
			mount_path:  "/var/vault/globemaster/storage/token"
			vault_mount: "globemaster"
			vault_path:  "storage/mirror-bazel/token"
			vault_key:   "secretkey"
		},
	]
	cpu_limit: "1000m"
	event: ["manual"]
}
