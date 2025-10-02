load("@bazel_skylib//lib:paths.bzl", "paths")
load("//build/private/util:semver.bzl", "semver")
load("//build/rules/kustomize:toolchain.bzl", "KustomizeToolchain")

Kustomization = provider()

def _kustomization_impl(ctx):
    toolchain = ctx.attr._kustomize[KustomizeToolchain]

    out = ctx.actions.declare_file("kustomize.%s.yaml" % ctx.label.name)
    args = ctx.actions.args()
    args.add("build")
    args.add(paths.dirname(ctx.file.src.path))
    args.add("--output=%s" % out.path)
    v = semver.parse(toolchain.version)
    if semver.gte(v, semver.parse("v4.0.1")):
        args.add("--load-restrictor=LoadRestrictionsNone")
    else:
        args.add("--load_restrictor=none")

    srcs = []
    for x in ctx.attr.resources:
        if Kustomization in x:
            srcs.extend(x[Kustomization].srcs)
            continue
        srcs.extend(x.files.to_list())

    ctx.actions.run(
        executable = toolchain.bin,
        inputs = depset(direct = [ctx.file.src], transitive = [depset(srcs)]),
        outputs = [out],
        arguments = [args],
    )

    data_runfiles = ctx.runfiles(files = [out])
    return [
        DefaultInfo(
            files = depset([out]),
            data_runfiles = data_runfiles,
        ),
        Kustomization(
            name = ctx.label.name,
            generated_manifest = out,
            srcs = [ctx.file.src] + srcs,
        ),
    ]

kustomization = rule(
    implementation = _kustomization_impl,
    attrs = {
        "src": attr.label(allow_single_file = True),
        "resources": attr.label_list(allow_files = True),
        "_kustomize": attr.label(
            default = "@kustomize//:toolchain",
        ),
    },
)
