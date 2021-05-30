load("@bazel_skylib//lib:shell.bzl", "shell")

def _dns_config_generator_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".sh")

    substitutions = {
        "@@BIN@@": shell.quote(ctx.executable._bin.short_path),
        "@@SRC@@": shell.quote(ctx.file.src.path),
        "@@DIR@@": shell.quote(ctx.attr.dir),
        "@@OUTFILE@@": shell.quote(ctx.attr.outfile),
    }
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = out,
        substitutions = substitutions,
        is_executable = True,
    )

    runfiles = ctx.runfiles(files = [out, ctx.file.src, ctx.executable._bin])
    return [
        DefaultInfo(
            runfiles = runfiles,
            executable = out,
        ),
    ]

_dns_config_generator = rule(
    implementation = _dns_config_generator_impl,
    executable = True,
    attrs = {
        "src": attr.label(allow_single_file = True),
        "outfile": attr.string(default = "dnsconfig.js"),
        "dir": attr.string(),
        "_template": attr.label(
            default = "//build/rules/dns-config-generator:run.bash",
            allow_single_file = True,
        ),
        "_bin": attr.label(
            default = "//go/cmd/dns-config-generator:dns-config-generator",
            executable = True,
            cfg = "host",
        ),
    },
)

def dns_config_generator(name, **kwargs):
    kwargs["dir"] = native.package_name()
    _dns_config_generator(name = name, **kwargs)
