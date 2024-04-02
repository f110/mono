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
