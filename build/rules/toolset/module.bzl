load("//build/private/kustomize:repository.bzl", "kustomize_binary")
load("//build/private/kind:repository.bzl", "kind_binary")
load("//build/private/etcd:repository.bzl", "etcd_binary")
load("//build/private/minio:repository.bzl", "minio_binary")
load("//build/private/vault:repository.bzl", "vault_binary")

def _toolset_impl(module_ctx):
    for mod in module_ctx.modules:
        for tools in mod.tags.k8s:
            if tools.kustomize:
                kustomize_binary(name = "kustomize", version = tools.kustomize)
            if tools.kind:
                kind_binary(name = "kind", version = tools.kind)
            if tools.etcd:
                etcd_binary(name = "etcd", version = tools.etcd)
            if tools.minio:
                minio_binary(name = "minio", version = tools.minio)

        for tools in mod.tags.test_tool:
            if tools.vault:
                vault_binary(name = "vault", version = tools.vault)

toolset_extension = module_extension(
    tag_classes = {
        "k8s": tag_class(attrs = {
            "kustomize": attr.string(),
            "kind": attr.string(),
            "etcd": attr.string(),
            "minio": attr.string(),
        }),
        "test_tool": tag_class(attrs = {
            "vault": attr.string(),
        }),
    },
    implementation = _toolset_impl,
)
