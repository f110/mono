load("//build/private/assets:assets.bzl", "multi_platform_download")
load("//build/private/kwok:assets.bzl", "KWOK_ASSETS")

def _kwok_binary_impl(ctx):
    if not ctx.attr.version in KWOK_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download(ctx, KWOK_ASSETS[ctx.attr.version], Label("//build/private/kwok:BUILD.kwok.bazel"))

kwok_binary = repository_rule(
    implementation = _kwok_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
