load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "7c10271940c6bce577d51a075ae77728964db285dac0a46614a7934dc34303e6",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.26.0/rules_go-v0.26.0.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.26.0/rules_go-v0.26.0.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
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
#    sha256 = "ec185cf615f01d69ce25ad0ca60c7dbb22b5f570af6f6ce199ca6c1812a86ae6",
#    strip_prefix = "rules_k8s_controller-0.10.0",
#    urls = [
#        "https://github.com/f110/rules_k8s_controller/archive/v0.10.0.tar.gz",
#    ],
#)

git_repository(
    name = "dev_f110_rules_k8s_controller",
    commit = "f90eae06f614aba0c065e6fd1d5c09b5dd476851",
    remote = "https://github.com/f110/rules_k8s_controller",
)

git_repository(
    name = "dev_f110_rules_extras",
    commit = "23175122db6205204ff30291dc9a2d62752a1862",
    remote = "https://github.com/f110/rules_extras",
)

git_repository(
    name = "dev_f110_protoc_ddl",
    commit = "127db5500265a684ee17aaea6b85240fcf469fd3",
    remote = "https://github.com/f110/protoc-ddl",
)

load("@bazel_gazelle//:deps.bzl", "go_repository")

go_repository(
    name = "org_golang_google_grpc",
    build_file_proto_mode = "disable",
    importpath = "google.golang.org/grpc",
    sum = "h1:EC2SB8S04d2r73uptxphDSUG+kTKVgjRPF+N3xpxRB4=",
    version = "v1.29.1",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:oWX7TPOiFAMXLq8o0ikBYfCJVlRHBcsciT5bXOrH628=",
    version = "v0.0.0-20190311183353-d8887717615a",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:g61tztE5qeGQ89tm6NTjjM9VPIm088od1l6aSorWRWg=",
    version = "v0.3.0",
)

load("@dev_f110_rules_extras//go:deps.bzl", "go_extras_dependencies")

go_extras_dependencies()

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(version = "1.16.2")

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

load("@dev_f110_rules_k8s_controller//k8s/kustomize:def.bzl", "kustomize_binary")

kustomize_binary(
    name = "kustomize",
    version = "v4.0.5",
)

load("//build/rules/kind:def.bzl", "kind_binary")

kind_binary(
    name = "kind",
    version = "0.9.0",
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

http_file(
    name = "argocd_vault_plugin",
    sha256 = "8888551f80efae9a4c95120c241b729b7bf8926570e64339840adc2852d9e185",
    urls = ["https://github.com/IBM/argocd-vault-plugin/releases/download/v0.7.0/argocd-vault-plugin_0.7.0_linux_amd64"],
)
