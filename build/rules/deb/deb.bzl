BUILD_TMPL = """\
filegroup(
    name = "control",
    srcs = glob(["control.tar.*"]),
    visibility = ["//visibility:public"]
)
filegroup(
    name = "data",
    srcs = glob(["data.tar.*"]),
    visibility = ["//visibility:public"]
)
"""

def _deb_pkg_impl(ctx):
    ctx.download_and_extract(
        url = ctx.attr.urls,
        sha256 = ctx.attr.sha256,
        type = "deb",
    )
    ctx.file(
        "BUILD.bazel",
        content = BUILD_TMPL,
    )

deb_pkg = repository_rule(
    implementation = _deb_pkg_impl,
    attrs = {
        "package_name": attr.string(mandatory = True),
        "urls": attr.string_list(mandatory = True),
        "sha256": attr.string(mandatory = True),
    },
)
