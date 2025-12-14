job(
    name = "test_all",
    command = "test",
    all_revision = True,
    github_status = True,
    targets = [
        "//...",
        "-//containers/zoekt-indexer/...",
        "-//containers/zoekt-webserver/...",
        "-//py/...",
    ],
    platforms = [
        "@rules_go//go/toolchain:linux_amd64",
    ],
    cpu_limit = "2000m",
    memory_limit = "8192Mi",
    event = ["push"],
)

job(
    name = "publish_zoekt_indexer",
    command = "run",
    container = "repo.center.x.f110.dev/codesearch/zoekt-indexer-builder:latest",
    targets = ["//containers/zoekt-indexer:push"],
    platforms = [
        "@rules_go//go/toolchain:linux_amd64",
    ],
    args = ["--insecure"],
    cpu_limit = "2000m",
    event = ["manual"],
)

job(
    name = "publish_build",
    command = "run",
    targets = ["//containers/build:push_build"],
    platforms = [
        "@rules_go//go/toolchain:linux_amd64",
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
        "@rules_go//go/toolchain:linux_amd64",
    ],
    args = [
        "--insecure",  # To run internally, accessing to the registry is used http. So We have to pass --insecure flag.
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)

job(
    name = "publish_build_frontend",
    command = "run",
    targets = ["//containers/build:push_frontend"],
    platforms = [
        "@rules_go//go/toolchain:linux_amd64",
    ],
    args = [
        "--insecure",  # To run internally, accessing to the registry is used http. So We have to pass --insecure flag.
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)

job(
    name = "publish_build_frontend_dev",
    command = "run",
    targets = ["//containers/build:push_frontend_dev"],
    platforms = [
        "@rules_go//go/toolchain:linux_amd64",
    ],
    args = [
        "--insecure",  # To run internally, accessing to the registry is used http. So We have to pass --insecure flag.
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)

job(
    name = "publish_controller_manager",
    command = "run",
    targets = ["//containers/controller-manager:push"],
    platforms = [
        "@rules_go//go/toolchain:linux_amd64",
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
        "@rules_go//go/toolchain:linux_amd64",
    ],
    secrets = [
        secret(mount_path = "/var/vault/globemaster/storage/token", vault_mount = "globemaster", vault_path = "storage/mirror-bazel/token", vault_key = "secretkey"),
    ],
    args = [
        "--rules-macro-file=$(WORKSPACE)/rules_dependencies.bzl",
        "--bucket=mirror",
        "--endpoint=http://incluster.storage.svc.cluster.local:9000",
        "--region=US",
        "--access-key=I4e91N6IGSeJfxsq",
        "--secret-access-key-file=/var/vault/globemaster/storage/token/secretkey",
        "--bazel",
    ],
    cpu_limit = "1000m",
    event = ["manual"],
)

job(
    name = "publish_bazel_remote",
    command = "run",
    targets = ["//containers/bazel-remote:push"],
    platforms = [
        "@rules_go//go/toolchain:linux_amd64",
    ],
    args = [
        "--insecure",  # To run internally, accessing to the registry is used http. So We have to pass --insecure flag.
    ],
    cpu_limit = "2000m",
    event = ["manual"],
)
