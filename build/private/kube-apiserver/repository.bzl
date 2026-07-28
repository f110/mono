load("//build/private/assets:assets.bzl", "multi_platform_download_and_extract")
load("//build/private/kube-apiserver:assets.bzl", "KUBE_APISERVER_ASSETS")

def _kube_apiserver_binary_impl(ctx):
    if not ctx.attr.version in KUBE_APISERVER_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download_and_extract(ctx, KUBE_APISERVER_ASSETS[ctx.attr.version], Label("//build/private/kube-apiserver:BUILD.kube-apiserver.bazel"))

kube_apiserver_binary = repository_rule(
    implementation = _kube_apiserver_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
