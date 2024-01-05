load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("//build/rules/vault:def.bzl", "vault_binary")
load("//build/rules/minio:def.bzl", "minio_binary")
load("//build/rules/etcd:def.bzl", "etcd_binary")
load("//build/rules/kind:def.bzl", "kind_binary")
load("//build/rules/kustomize:def.bzl", "kustomize_binary")
load("//build/rules/go:def.bzl", "go_download_tarball")
load("@rules_oci//oci:pull.bzl", "oci_pull")

versions = {
    "kustomize": "v4.5.4",
    "kind": "0.14.0",
    "etcd": "3.5.6",
    "minio": "RELEASE.2022-12-02T19-19-22Z",
    "vault": "1.11.4",
}

containers = {
    "com_google_distroless_base": "gcr.io/distroless/base@sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
    "com_google_distroless_base_debug": "gcr.io/distroless/base@sha256:c59a1e5509d1b2586e28b899667774e599b79d7289a6bb893766a0cbbce7384b",
    "com_google_distroless_base_arm64": "gcr.io/distroless/base@sha256:f19b05270bbd5c38e12c5610f23c1dfe4441858d959102a83074cf17ec074b50",
    "com_google_distroless_base_debian12": "gcr.io/distroless/base-debian12@sha256:9d6c97c160bff0f78a443b583811dd0c8dde5c5086fe8fd2aaf2c23ee7e9590a",
    "com_google_distroless_base_debian12_arm64": "gcr.io/distroless/base-debian12@sha256:b251ebd844116427f92523668ca5e9f8d803e479eef44705b62090176d5e8cc7",
    "nix_amd64": "docker.io/nixos/nix@sha256:52498160e7a93d9b6b881200690b7e5c446baa314dbf06c0a8015389afd0e58f",  # 2.15.2-amd64
}

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

    http_file(
        name = "bazel_remote",
        sha256 = "5e4b248262a56e389e9ee4212ffd0498746347fb5bf155785c9410ba2abc7b07",
        urls = ["https://github.com/buchgr/bazel-remote/releases/download/v2.4.1/bazel-remote-2.4.1-linux-x86_64"],
    )

    go_download_tarball(
        name = "golang_1.21",
        sha256 = "e2bc0b3e4b64111ec117295c088bde5f00eeed1567999ff77bc859d7df70078e",
        urls = ["https://golang.org/dl/go1.21.5.linux-amd64.tar.gz"],
    )

def container_dependencies():
    for k, v in containers.items():
        image, digest = v.split("@", 1)
        registry, repository = image.split("/", 1)

        oci_pull(
            name = k,
            digest = digest,
            image = image,
        )
