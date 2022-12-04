load("//build/rules/kind:assets.bzl", "KIND_ASSETS")
load("//build/private/assets:assets.bzl", "multi_platform_download")

def _kind_binary_impl(ctx):
    if not ctx.attr.version in KIND_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download(ctx, KIND_ASSETS[ctx.attr.version], Label("//build/rules/kind:BUILD.kind.bazel"))

kind_binary = repository_rule(
    implementation = _kind_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
