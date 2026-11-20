def _exec_filegroup_impl(ctx):
    return [
        DefaultInfo(files = depset(transitive = [src[DefaultInfo].files for src in ctx.attr.srcs])),
    ]

exec_filegroup = rule(
    implementation = _exec_filegroup_impl,
    doc = """Builds srcs in the exec configuration and forwards their files.

Use this for the outputs of the tools that must run on the exec platform with the dependencies resolved in the same configuration
(e.g. vite which loads the native modules of rollup and esbuild selected by the target platform).""",
    attrs = {
        "srcs": attr.label_list(cfg = "exec", allow_files = True),
    },
)
