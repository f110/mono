job(
    name = "test_all",
    command = "test",
    all_revision = True,
    github_status = True,
    targets = [
        "//...",
        "-//vendor/github.com/JuulLabs-OSS/cbgo:cbgo",
        "-//third_party/universal-ctags/ctags:ctags",
        "-//containers/zoekt-indexer/...",
        "-//vendor/github.com/go-enry/go-oniguruma/...",
    ],
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    cpu_limit = "2000m",
    memory_limit = "8192Mi",
    event = ["push"],
)

job(
    name = "publish_zoekt_indexer",
    command = "run",
    container = "registry.f110.dev/tools/zoekt-indexer-builder:latest",
    targets = ["//containers/zoekt-indexer:push"],
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    secrets = [
        registry_secret(host = "registry.f110.dev", vault_mount = "globemaster", vault_path = "registry.f110.dev/build", vault_key = "robot$build"),
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)

job(
    name = "publish_build",
    command = "run",
    targets = ["//containers/build:push_build"],
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    args = [
        "--insecure",  # To run internally, accessing to the registry is used http. So We have to pass --insecure flag.
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)

job(
    name = "publish_build_sidecar",
    command = "run",
    targets = ["//containers/build:push_sidecar"],
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    args = [
        "--insecure",  # To run internally, accessing to the registry is used http. So We have to pass --insecure flag.
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)

job(
    name = "mirror_bazel",
    command = "run",
    targets = ["//cmd/rotarypress"],
    platforms = [
        "@io_bazel_rules_go//go/toolchain:linux_amd64",
    ],
    secrets = [
        secret(mount_path = "/var/vault/globemaster/storage/token", vault_mount = "globemaster", vault_path = "storage/token", vault_key = "secretkey"),
    ],
    args = [
        "--rules-macro-file=$(WORKSPACE)/rules_dependencies.bzl",
        "--bucket=mirror",
        "--endpoint=http://incluster-hl-svc.storage.svc.cluster.local:9000",
        "--region=US",
        "--access-key=NogubAm7w1PC",
        "--secret-access-key-file=/var/vault/globemaster/storage/token/secretkey",
        "--bazel",
    ],
    cpu_limit = "1000m",
    event = ["manual"],
)
