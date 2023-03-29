load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("//build/rules/vault:def.bzl", "vault_binary")
load("//build/rules/minio:def.bzl", "minio_binary")
load("//build/rules/etcd:def.bzl", "etcd_binary")
load("//build/rules/kind:def.bzl", "kind_binary")
load("//build/rules/kustomize:def.bzl", "kustomize_binary")
load("@rules_oci//oci:pull.bzl", "oci_pull")
load("@io_bazel_rules_docker//container:container.bzl", "container_pull")

versions = {
    "kustomize": "v4.5.4",
    "kind": "0.14.0",
    "etcd": "3.5.6",
    "minio": "RELEASE.2022-12-02T19-19-22Z",
    "vault": "1.11.4",
}

containers = {
    "com_google_distroless_base": "gcr.io/distroless/base@sha256:e8f299757c8f8f2ebbebc4fd1826720a0a7a45fce0a4f9e7d210c5cc09d624a3",
    "com_google_distroless_base_debug": "gcr.io/distroless/base@sha256:c532b9983712e1d9fadec8449908a9ac329909f37a47d491f2ad06ee6040fa4c",
    "com_google_distroless_base_arm64": "gcr.io/distroless/base@sha256:bf4d6dc160bab223a0d377df083ad6b4ebacf5db2a313d8d7f3f07c9da967093",
}

golang_tarball_build_file = """
filegroup(
    name = "srcs",
    srcs = glob(["go/src/**", "go/bin/**", "go/pkg/**"]),
    visibility = ["//visibility:public"],
)
"""

def repository_dependencies():
    kustomize_binary(
        name = "kustomize",
        version = versions["kustomize"],
    )

    kind_binary(
        name = "kind",
        version = versions["kind"],
    )

    etcd_binary(
        name = "etcd",
        version = versions["etcd"],
    )

    minio_binary(
        name = "minio",
        version = versions["minio"],
    )

    vault_binary(
        name = "vault",
        version = versions["vault"],
    )

    http_file(
        name = "argocd_vault_plugin",
        sha256 = "957001f4bcd5db9aca468fbea9afa19d5088c06708fbcf97b07ba8e369447932",
        urls = ["https://github.com/argoproj-labs/argocd-vault-plugin/releases/download/v1.13.1/argocd-vault-plugin_1.13.1_linux_amd64"],
    )

    http_archive(
        name = "golang_1.17",
        build_file_content = golang_tarball_build_file,
        sha256 = "6bf89fc4f5ad763871cf7eac80a2d594492de7a818303283f1366a7f6a30372d",
        urls = ["https://golang.org/dl/go1.17.linux-amd64.tar.gz"],
    )

    http_file(
        name = "bazel_remote",
        sha256 = "5e4b248262a56e389e9ee4212ffd0498746347fb5bf155785c9410ba2abc7b07",
        urls = ["https://github.com/buchgr/bazel-remote/releases/download/v2.4.1/bazel-remote-2.4.1-linux-x86_64"],
    )

def container_dependencies():
    for k, v in containers.items():
        image, digest = v.split("@", 1)
        registry, repository = image.split("/", 1)

        oci_pull(
            name = "oci_%s" % k,
            digest = digest,
            image = image,
        )

        container_pull(
            name = k,
            digest = digest,
            registry = registry,
            repository = repository,
        )
