load("//build/rules/minio:assets.bzl", "MINIO_ASSETS")
load("//build/private/assets:assets.bzl", "multi_platform_download")

def _minio_binary_impl(ctx):
    if not ctx.attr.version in MINIO_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download(ctx, MINIO_ASSETS[ctx.attr.version], Label("//build/rules/minio:BUILD.minio.bazel"))

minio_binary = repository_rule(
    implementation = _minio_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
