workspace(name = "mono")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "dd926a88a564a9246713a9c00b35315f54cbd46b31a26d5d8fb264c07045f05d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.38.1/rules_go-v0.38.1.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.38.1/rules_go-v0.38.1.zip",
        "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_go/releases/download/v0.38.1/rules_go-v0.38.1.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "501deb3d5695ab658e82f6f6f549ba681ea3ca2a5fb7911154b5aa45596183fa",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.26.0/bazel-gazelle-v0.26.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.26.0/bazel-gazelle-v0.26.0.tar.gz",
        "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/bazel-gazelle/releases/download/v0.26.0/bazel-gazelle-v0.26.0.tar.gz",
    ],
)

http_archive(
    name = "rules_python",
    sha256 = "5fa3c738d33acca3b97622a13a741129f67ef43f5fdfcec63b29374cc0574c29",
    strip_prefix = "rules_python-0.9.0",
    urls = [
        "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.9.0.tar.gz",
        "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_python/archive/refs/tags/0.9.0.tar.gz",
    ],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "59d5b42ac315e7eadffa944e86e90c2990110a1c8075f1cd145f487e999d22b3",
    strip_prefix = "rules_docker-0.17.0",
    urls = [
        "https://github.com/bazelbuild/rules_docker/releases/download/v0.17.0/rules_docker-v0.17.0.tar.gz",
        "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_docker/releases/download/v0.17.0/rules_docker-v0.17.0.tar.gz",
    ],
)

http_archive(
    name = "rules_pkg",
    sha256 = "8a298e832762eda1830597d64fe7db58178aa84cd5926d76d5b744d6558941c2",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.7.0/rules_pkg-0.7.0.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.7.0/rules_pkg-0.7.0.tar.gz",
        "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_pkg/releases/download/0.7.0/rules_pkg-0.7.0.tar.gz",
    ],
)

http_archive(
    name = "rules_foreign_cc",
    sha256 = "33a5690733c5cc2ede39cb62ebf89e751f2448e27f20c8b2fbbc7d136b166804",
    strip_prefix = "rules_foreign_cc-0.5.1",
    urls = [
        "https://github.com/bazelbuild/rules_foreign_cc/archive/0.5.1.tar.gz",
        "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_foreign_cc/archive/refs/tags/0.5.1.tar.gz",
    ],
)

http_archive(
    name = "com_google_protobuf",
    sha256 = "a295dd3b9551d3e2749a9969583dea110c6cdcc39d02088f7c7bb1100077e081",
    strip_prefix = "protobuf-3.21.1",
    urls = [
        "https://mirror.bazel.build/github.com/protocolbuffers/protobuf/archive/v3.21.1.tar.gz",
        "https://github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz",
        "https://mirror.bucket.x.f110.dev/github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz",
    ],
)

git_repository(
    name = "dev_f110_rules_extras",
    commit = "dd9d0fc35009dd3d3c852e94432e64ec4a2c27b1",
    remote = "https://github.com/f110/rules_extras",
)

git_repository(
    name = "dev_f110_protoc_ddl",
    commit = "740a8ba3e227ba252c2034c3b955fd1471105eb6",
    remote = "https://github.com/f110/protoc-ddl",
)

git_repository(
    name = "dev_f110_kubeproto",
    commit = "90d00e364ad040d388c54b32c9ac3d85604bc6ec",
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

go_register_toolchains(version = "1.20.1")

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
    digest = "sha256:e8f299757c8f8f2ebbebc4fd1826720a0a7a45fce0a4f9e7d210c5cc09d624a3",
    registry = "gcr.io",
    repository = "distroless/base",
)

container_pull(
    name = "com_google_distroless_base_debug",
    digest = "sha256:c532b9983712e1d9fadec8449908a9ac329909f37a47d491f2ad06ee6040fa4c",
    registry = "gcr.io",
    repository = "distroless/base",
)

container_pull(
    name = "com_google_distroless_base_arm64",
    digest = "sha256:bf4d6dc160bab223a0d377df083ad6b4ebacf5db2a313d8d7f3f07c9da967093",
    registry = "gcr.io",
    repository = "distroless/base",
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

load("//build/rules/etcd:def.bzl", "etcd_binary")

etcd_binary(
    name = "etcd",
    version = "3.5.6",
)

load("//build/rules/minio:def.bzl", "minio_binary")

minio_binary(
    name = "minio",
    version = "RELEASE.2022-12-02T19-19-22Z",
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

http_file(
    name = "argocd_vault_plugin",
    sha256 = "957001f4bcd5db9aca468fbea9afa19d5088c06708fbcf97b07ba8e369447932",
    urls = ["https://github.com/argoproj-labs/argocd-vault-plugin/releases/download/v1.13.1/argocd-vault-plugin_1.13.1_linux_amd64"],
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

load("@bazel_skylib//lib:unittest.bzl", "register_unittest_toolchains")

register_unittest_toolchains()

load("//build/rules/vault:def.bzl", "vault_binary")

vault_binary(
    name = "vault",
    version = "1.11.4",
)
