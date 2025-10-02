load("//build/private/etcd:assets.bzl", "ETCD_ASSETS")
load("//build/private/assets:assets.bzl", "multi_platform_download_and_extract")

def _etcd_binary_impl(ctx):
    if not ctx.attr.version in ETCD_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download_and_extract(ctx, ETCD_ASSETS[ctx.attr.version], Label("//build/private/etcd:BUILD.etcd.bazel"))

etcd_binary = repository_rule(
    implementation = _etcd_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
