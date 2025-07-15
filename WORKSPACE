workspace(name = "mono")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")
load("//:rules_dependencies.bzl", "rules_dependencies")

rules_dependencies()

git_repository(
    name = "dev_f110_rules_extras",
    commit = "dd9d0fc35009dd3d3c852e94432e64ec4a2c27b1",
    remote = "https://github.com/f110/rules_extras",
)

git_repository(
    name = "dev_f110_protoc_ddl",
    commit = "1cb0fefe60f4aeecc458a2f48abbc4a4e59f637f",
    remote = "https://github.com/f110/protoc-ddl",
)

git_repository(
    name = "dev_f110_kubeproto",
    commit = "90d00e364ad040d388c54b32c9ac3d85604bc6ec",
    remote = "https://github.com/f110/kubeproto",
)

load("@dev_f110_rules_extras//go:deps.bzl", "go_extras_dependencies")

go_extras_dependencies()

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.24.5")

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

load("@rules_oci//oci:dependencies.bzl", "rules_oci_dependencies")

rules_oci_dependencies()

load("@rules_oci//oci:repositories.bzl", "oci_register_toolchains")

oci_register_toolchains(name = "oci")

load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")

rules_foreign_cc_dependencies()

load("@rules_python//python:repositories.bzl", "py_repositories")

py_repositories()

load("@rules_python//python:pip.bzl", "pip_parse")

pip_parse(
    name = "pip_deps",
    requirements_lock = "//:requirements.lock",
)

load("@pip_deps//:requirements.bzl", "install_deps")

install_deps()

load("@bazel_skylib//lib:unittest.bzl", "register_unittest_toolchains")

register_unittest_toolchains()

load("//:dependencies.bzl", "container_dependencies", "repository_dependencies")

repository_dependencies()

container_dependencies()

load("//:deb_packages.bzl", debian_package_dependencies = "debian_packages")

debian_package_dependencies()

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies")

rules_proto_dependencies()

load("@rules_proto//proto:toolchains.bzl", "rules_proto_toolchains")

rules_proto_toolchains()

load("@bazel_features//:deps.bzl", "bazel_features_deps")

bazel_features_deps()

http_archive(
    name = "com_github_golang-migrate_migrate_amd64",
    build_file_content = """exports_files(["migrate"])""",
    sha256 = "2a08137b4720aa457bc760540723e313783f1fab27473463bdcc5fc2e9252959",
    urls = ["https://github.com/golang-migrate/migrate/releases/download/v4.18.1/migrate.linux-amd64.tar.gz"],
)
