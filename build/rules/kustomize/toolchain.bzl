KustomizeToolchain = provider(
    fields = {
        "version": "The version string of kustomize",
        "bin": "",
    },
)

def _kustomize_toolchain(ctx):
    return [KustomizeToolchain(
        version = ctx.attr.version,
        bin = ctx.executable.bin,
    )]

kustomize_toolchain = rule(
    implementation = _kustomize_toolchain,
    attrs = {
        "version": attr.string(
            mandatory = True,
        ),
        "bin": attr.label(
            executable = True,
            cfg = "host",
        ),
    },
)
