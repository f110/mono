load("@bazel_skylib//lib:shell.bzl", "shell")

def _cluster_create_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".sh")
    args = [
        "create",
        "--k8s-version=%s" % ctx.attr.version,
        "--worker-num=%s" % ctx.attr.worker_num,
    ]
    if ctx.attr.cluster_name:
        args.append("--name=%s" % ctx.attr.cluster_name)
    if ctx.file.manifest:
        args.append("--manifest=%s" % ctx.file.manifest.short_path)

    substitutions = {
        "@@BIN@@": shell.quote(ctx.executable._bin.short_path),
        "@@KIND@@": shell.quote(ctx.file._kind.path),
        "@@ARGS@@": shell.array_literal(args),
    }
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = out,
        substitutions = substitutions,
    )
    files = [ctx.file._kind, ctx.executable._bin]
    if ctx.file.manifest:
        files.append(ctx.file.manifest)

    return [
        DefaultInfo(
            executable = out,
            runfiles = ctx.runfiles(files = files),
        ),
    ]

cluster_create = rule(
    implementation = _cluster_create_impl,
    executable = True,
    attrs = {
        "cluster_name": attr.string(),
        "version": attr.string(default = "v1.23.4"),
        "worker_num": attr.int(default = 1),
        "manifest": attr.label(
            allow_single_file = True,
        ),
        "_template": attr.label(
            default = "//build/rules/kind:cluster.bash",
            allow_single_file = True,
        ),
        "_kind": attr.label(
            default = "@kind//:file",
            allow_single_file = True,
        ),
        "_bin": attr.label(
            default = "//go/cmd/kindcluster",
            executable = True,
            cfg = "host",
        ),
        "_go_context_data": attr.label(
            default = "@rules_go//:go_context_data",
        ),
    },
    toolchains = ["@rules_go//go:toolchain"],
)

def _cluster_delete_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".sh")
    args = ["delete"]
    if ctx.attr.cluster_name:
        args.append("--name=%s" % ctx.attr.cluster_name)
    substitutions = {
        "@@BIN@@": shell.quote(ctx.executable._bin.short_path),
        "@@KIND@@": shell.quote(ctx.file._kind.path),
        "@@ARGS@@": shell.array_literal(args),
    }
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = out,
        substitutions = substitutions,
    )
    files = [ctx.file._kind, ctx.executable._bin]

    return [
        DefaultInfo(
            executable = out,
            runfiles = ctx.runfiles(files = files),
        ),
    ]

cluster_delete = rule(
    implementation = _cluster_delete_impl,
    executable = True,
    attrs = {
        "cluster_name": attr.string(),
        "_template": attr.label(
            default = "//build/rules/kind:cluster.bash",
            allow_single_file = True,
        ),
        "_kind": attr.label(
            default = "@kind//:file",
            allow_single_file = True,
        ),
        "_bin": attr.label(
            default = "//go/cmd/kindcluster",
            executable = True,
            cfg = "host",
        ),
        "_go_context_data": attr.label(
            default = "@rules_go//:go_context_data",
        ),
    },
    toolchains = ["@rules_go//go:toolchain"],
)

def cluster(name, **kwargs):
    kwargs["cluster_name"] = name
    cluster_create(name = name + ".create", **kwargs)
    cluster_delete(name = name + ".delete", cluster_name = name)
