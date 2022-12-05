def register_development_environments():
    native.sh_binary(
        name = "env_build",
        srcs = ["//go/cmd/monodev"],
        args = [
            "env",
            "build",
            "--etcd=$(location @etcd//:bin)",
            "--minio=$(location @minio//:file)",
        ],
        data = [
            "@etcd//:bin",
            "@minio//:file",
        ],
    )
