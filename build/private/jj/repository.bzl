load("//build/private/jj:assets.bzl", "JJ_ASSETS")
load("//build/private/assets:assets.bzl", "multi_platform_download_and_extract")

def _jj_binary_impl(ctx):
    if not ctx.attr.version in JJ_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download_and_extract(ctx, JJ_ASSETS[ctx.attr.version], Label("//build/private/jj:BUILD.jj.bazel"))

jj_binary = repository_rule(
    implementation = _jj_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
