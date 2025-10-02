load("//build/private/vault:assets.bzl", "VAULT_ASSETS")
load("//build/private/assets:assets.bzl", "multi_platform_download_and_extract")

def _vault_binary_impl(ctx):
    if not ctx.attr.version in VAULT_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)
    multi_platform_download_and_extract(ctx, VAULT_ASSETS[ctx.attr.version], Label("//build/private/vault:BUILD.vault.bazel"))
    os = ""
    if ctx.os.name == "linux":
        os = "linux"
    elif ctx.os.name == "mac os x":
        os = "darwin"
    else:
        fail("%s is not supported" % ctx.os.name)
    arch = ctx.execute(["uname", "-m"]).stdout.strip()

    # On Linux, uname returns x86_64 as CPU architecture.
    if arch == "x86_64":
        arch = "amd64"

    if not ctx.attr.version in VAULT_ASSETS:
        fail("%s is not supported version" % ctx.attr.version)

    url, checksum = VAULT_ASSETS[ctx.attr.version][os][arch]
    ctx.download_and_extract(
        url = url,
        sha256 = checksum,
        type = "zip",
    )

    ctx.file("BUILD.bazel", "sh_binary(name = \"bin\", srcs = [\"vault\"], visibility = [\"//visibility:public\"])")

vault_binary = repository_rule(
    implementation = _vault_binary_impl,
    attrs = {
        "version": attr.string(),
    },
)
