load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "685052b498b6ddfe562ca7a97736741d87916fe536623afb7da2824c0211c369",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.33.0/rules_go-v0.33.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.33.0/rules_go-v0.33.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "5982e5463f171da99e3bdaeff8c0f48283a7a5f396ec5282910b9e8a49c0dd7e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
    ],
)

http_archive(
    name = "rules_python",
    sha256 = "5fa3c738d33acca3b97622a13a741129f67ef43f5fdfcec63b29374cc0574c29",
    strip_prefix = "rules_python-0.9.0",
    url = "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.9.0.tar.gz",
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "59d5b42ac315e7eadffa944e86e90c2990110a1c8075f1cd145f487e999d22b3",
    strip_prefix = "rules_docker-0.17.0",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.17.0/rules_docker-v0.17.0.tar.gz"],
)

http_archive(
    name = "rules_pkg",
    sha256 = "8a298e832762eda1830597d64fe7db58178aa84cd5926d76d5b744d6558941c2",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.7.0/rules_pkg-0.7.0.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.7.0/rules_pkg-0.7.0.tar.gz",
    ],
)

http_archive(
    name = "rules_foreign_cc",
    sha256 = "33a5690733c5cc2ede39cb62ebf89e751f2448e27f20c8b2fbbc7d136b166804",
    strip_prefix = "rules_foreign_cc-0.5.1",
    url = "https://github.com/bazelbuild/rules_foreign_cc/archive/0.5.1.tar.gz",
)

http_archive(
    name = "com_google_protobuf",
    sha256 = "b07772d38ab07e55eca4d50f4b53da2d998bb221575c60a4f81100242d4b4889",
    strip_prefix = "protobuf-3.20.0",
    urls = [
        "https://mirror.bazel.build/github.com/protocolbuffers/protobuf/archive/v3.20.0.tar.gz",
        "https://github.com/protocolbuffers/protobuf/archive/v3.20.0.tar.gz",
    ],
)

http_archive(
    name = "dev_f110_rules_k8s_controller",
    sha256 = "ddc05d5941371c08ee9145b2984c77b6b28c3ca7ed2d80ef1be1f61986405a3e",
    strip_prefix = "rules_k8s_controller-0.14.0",
    urls = [
        "https://github.com/f110/rules_k8s_controller/archive/v0.14.0.tar.gz",
    ],
)

#git_repository(
#    name = "dev_f110_rules_k8s_controller",
#    commit = "5c3933b6f1509d4e86b3dd916ee7fb848048b199",
#    remote = "https://github.com/f110/rules_k8s_controller",
#)

git_repository(
    name = "dev_f110_rules_extras",
    commit = "dd9d0fc35009dd3d3c852e94432e64ec4a2c27b1",
    remote = "https://github.com/f110/rules_extras",
)

git_repository(
    name = "dev_f110_protoc_ddl",
    commit = "f15651f509bf64e43a6493f5b11214af9b921e9b",
    remote = "https://github.com/f110/protoc-ddl",
)

git_repository(
    name = "dev_f110_kubeproto",
    commit = "96094605874d1aedbb1b70621fda6fec914084bc",
    remote = "https://github.com/f110/kubeproto",
)

# This is workaround for dependency problem.
# Ref: https://github.com/bazelbuild/bazel-gazelle/issues/1217

load("@bazel_gazelle//:deps.bzl", "go_repository")

go_repository(
    name = "org_golang_x_mod",
    build_external = "external",
    importpath = "golang.org/x/mod",
    sum = "h1:kQgndtyPBW/JIYERgdxfwMYh3AVStj88WQTlNDi2a+o=",
    version = "v0.6.0-dev.0.20220106191415-9b9b3d81d5e3",
)

go_repository(
    name = "org_golang_x_net",
    generator_function = "gazelle_dependencies",
    generator_name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:20cMwl2fHAzkJMEA+8J4JgqBQcQGzbisXo31MIeenXI=",
    version = "v0.0.0-20210805182204-aaa1db679c0d",
)

go_repository(
    name = "org_golang_x_text",
    generator_function = "gazelle_dependencies",
    generator_name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:aRYxNxv6iGQlyVaZmk6ZgYEDa+Jg18DxebPSrd6bg1M=",
    version = "v0.3.6",
)

go_repository(
    name = "org_golang_google_grpc",
    build_external = "external",
    build_file_proto_mode = "disable",
    importpath = "google.golang.org/grpc",
    sum = "h1:oCjezcn6g6A75TGoKYBPgKmVBLexhYLM6MebdrPApP8=",
    version = "v1.46.0",
)

# End of workaround

load("@dev_f110_rules_extras//go:deps.bzl", "go_extras_dependencies")

go_extras_dependencies()

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.18.3")

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

load("@io_bazel_rules_docker//repositories:repositories.bzl", container_repositories = "repositories")
load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")
load("@io_bazel_rules_docker//container:container.bzl", "container_pull")

container_deps()

container_repositories()

load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")

rules_foreign_cc_dependencies()

container_pull(
    name = "com_google_distroless_base",
    digest = "sha256:7fa7445dfbebae4f4b7ab0e6ef99276e96075ae42584af6286ba080750d6dfe5",
    registry = "gcr.io",
    repository = "distroless/base",
)

container_pull(
    name = "com_google_distroless_base_debug",
    digest = "sha256:e12ba6be36761fd29e7c3beae66fc4e3a4a28652d0076bb9964274569f8e8a26",
    registry = "gcr.io",
    repository = "distroless/base",
)

container_pull(
    name = "com_google_distroless_base_arm64",
    digest = "sha256:c60be29941a0be6f748c8cf2e42832f95e9b73276042d3c44212af7cf4a152c9",
    registry = "gcr.io",
    repository = "distroless/base",
)

http_archive(
    name = "com_github_migrate_migrate",
    build_file_content = "filegroup(name = \"bin\", srcs = [\"migrate.linux-amd64\"], visibility = [\"//visibility:public\"])",
    sha256 = "9b39a0fe0e4dd1d6d3f0705f938a89c9d98c31152e0f097bb2e1556f9030387c",
    urls = ["https://github.com/golang-migrate/migrate/releases/download/v4.11.0/migrate.linux-amd64.tar.gz"],
)

load("//build/rules/kustomize:def.bzl", "kustomize_binary")

kustomize_binary(
    name = "kustomize",
    version = "v4.5.4",
)

load("//build/rules/kind:def.bzl", "kind_binary")

kind_binary(
    name = "kind",
    version = "0.14.0",
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

http_file(
    name = "argocd_vault_plugin",
    sha256 = "8888551f80efae9a4c95120c241b729b7bf8926570e64339840adc2852d9e185",
    urls = ["https://github.com/IBM/argocd-vault-plugin/releases/download/v0.7.0/argocd-vault-plugin_0.7.0_linux_amd64"],
)

golang_tarball_build_file = """
filegroup(
    name = "srcs",
    srcs = glob(["go/src/**", "go/bin/**", "go/pkg/**"]),
    visibility = ["//visibility:public"],
)
"""

http_archive(
    name = "golang_1.17",
    build_file_content = golang_tarball_build_file,
    sha256 = "6bf89fc4f5ad763871cf7eac80a2d594492de7a818303283f1366a7f6a30372d",
    urls = ["https://golang.org/dl/go1.17.linux-amd64.tar.gz"],
)

load("@rules_python//python:pip.bzl", "pip_install")

pip_install(
    name = "py_deps",
    requirements = "//:requirements.txt",
)
