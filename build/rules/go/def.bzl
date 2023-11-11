golang_tarball_build_file = """
filegroup(
    name = "srcs",
    srcs = glob(["src/**", "bin/**", "pkg/**"]),
    visibility = ["//visibility:public"],
)
"""

def _go_download_tarball_impl(ctx):
    ctx.download(
        url = ctx.attr.urls,
        sha256 = ctx.attr.sha256,
        output = "go_sdk.tar.gz",
    )
    res = ctx.execute(["tar", "-xf", "go_sdk.tar.gz", "--strip-components=1"])
    ctx.delete("go_sdk.tar.gz")
    ctx.file(
        "BUILD.bazel",
        golang_tarball_build_file,
        executable = False,
    )

go_download_tarball = repository_rule(
    implementation = _go_download_tarball_impl,
    attrs = {
        "urls": attr.string_list(),
        "sha256": attr.string(),
    },
)
