load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "6f111c57fd50baf5b8ee9d63024874dd2a014b069426156c55adbf6d3d22cb7b",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.25.0/rules_go-v0.25.0.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.25.0/rules_go-v0.25.0.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "b85f48fa105c4403326e9525ad2b2cc437babaa6e15a3fc0b1dbab0ab064bc7c",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.22.2/bazel-gazelle-v0.22.2.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.22.2/bazel-gazelle-v0.22.2.tar.gz",
    ],
)

http_archive(
    name = "rules_python",
    sha256 = "b6d46438523a3ec0f3cead544190ee13223a52f6a6765a29eae7b7cc24cc83a0",
    url = "https://github.com/bazelbuild/rules_python/releases/download/0.1.0/rules_python-0.1.0.tar.gz",
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "1698624e878b0607052ae6131aa216d45ebb63871ec497f26c67455b34119c80",
    strip_prefix = "rules_docker-0.15.0",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.15.0/rules_docker-v0.15.0.tar.gz"],
)

http_archive(
    name = "rules_pkg",
    sha256 = "6b5969a7acd7b60c02f816773b06fcf32fbe8ba0c7919ccdc2df4f8fb923804a",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.3.0/rules_pkg-0.3.0.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.3.0/rules_pkg-0.3.0.tar.gz",
    ],
)

http_archive(
    name = "com_google_protobuf",
    sha256 = "9748c0d90e54ea09e5e75fb7fac16edce15d2028d4356f32211cfa3c0e956564",
    strip_prefix = "protobuf-3.11.4",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.11.4.zip"],
)

#http_archive(
#    name = "dev_f110_rules_k8s_controller",
#    sha256 = "7fc332ccc1c23f356f6ac43720c00349419fbf641a06e589703cdd12358136db",
#    strip_prefix = "rules_k8s_controller-0.6.0",
#    urls = [
#        "https://github.com/f110/rules_k8s_controller/archive/v0.6.0.tar.gz",
#    ],
#)

git_repository(
    name = "dev_f110_rules_k8s_controller",
    commit = "3077786aaa108f22ee02298f959e1220fd6813c8",
    remote = "https://github.com/f110/rules_k8s_controller",
)

git_repository(
    name = "dev_f110_rules_extras",
    commit = "186d8d33b74f1ad888298e34510eeac4f86d0f9d",
    remote = "https://github.com/f110/rules_extras",
)

git_repository(
    name = "dev_f110_protoc_ddl",
    commit = "127db5500265a684ee17aaea6b85240fcf469fd3",
    remote = "https://github.com/f110/protoc-ddl",
)

load("@dev_f110_rules_extras//go:deps.bzl", "go_extras_dependencies")

go_extras_dependencies()

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.15.6")

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

container_pull(
    name = "com_google_distroless_base",
    digest = "sha256:7fa7445dfbebae4f4b7ab0e6ef99276e96075ae42584af6286ba080750d6dfe5",
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

load("@dev_f110_rules_k8s_controller//k8s/kustomize:def.bzl", "kustomize_binary")

kustomize_binary(
    name = "kustomize",
    version = "3.9.1",
)

load("//build/rules/kind:def.bzl", "kind_binary")

kind_binary(
    name = "kind",
    version = "0.9.0",
)