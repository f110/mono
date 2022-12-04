def register_development_environments():
    native.sh_binary(
        name = "dev_env_build",
        srcs = ["//go/cmd/monodev"],
        args = [
            "dev-env",
            "build",
            "--etcd=$(location @etcd//:bin)",
        ],
        data = [
            "@etcd//:bin",
        ],
    )
