load("//build/private/vault:assets.bzl", "VAULT_ASSETS")
load("//build/private/assets:assets.bzl", "multi_platform_download_and_extract")

def _vault_binary_impl(ctx):
    if not ctx.attr.version in VAULT_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download_and_extract(ctx, VAULT_ASSETS[ctx.attr.version], Label("//build/private/vault:BUILD.vault.bazel"))

vault_binary = repository_rule(
    implementation = _vault_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
