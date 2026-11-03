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
    built = out
    if ctx.attr.images:
        built = ctx.actions.declare_file("kustomize.%s.without_digest.yaml" % ctx.label.name)
    args.add("--output=%s" % built.path)
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
        outputs = [built],
        arguments = [args],
    )

    if ctx.attr.images:
        # Refer to the images by the digests so that the manifest uses the images which are built from the same
        # revision. The job which uses the image is named with the digest because the template of the job can not be
        # updated.
        yq = ctx.toolchains["@aspect_bazel_lib//lib:yq_toolchain_type"].yqinfo.bin
        digests = []
        exports = []
        expressions = []
        for i, (target, repository) in enumerate(ctx.attr.images.items()):
            digest = target.files.to_list()[0]
            digests.append(digest)
            exports.append("export REPOSITORY_%d=%s DIGEST_%d=$(cat %s)" % (i, repository, i, digest.path))
            expressions.append(
                "(.. | select(tag == \"!!map\" and .image == strenv(REPOSITORY_{i})) | .image) = strenv(REPOSITORY_{i}) + \"@\" + strenv(DIGEST_{i})".format(i = i),
            )
            expressions.append(
                "(select(.kind == \"Job\" and ([.spec.template.spec.containers[].image] | any_c(. == strenv(REPOSITORY_{i}) + \"@\" + strenv(DIGEST_{i})))) | .metadata.name) += \"-\" + (strenv(DIGEST_{i}) | sub(\"^sha256:([0-9a-f]{{6}}).*$\", \"${{1}}\"))".format(i = i),
            )
        ctx.actions.run_shell(
            inputs = [built] + digests,
            outputs = [out],
            tools = [yq],
            command = "%s && %s '%s' %s > %s" % (" && ".join(exports), yq.path, " | ".join(expressions), built.path, out.path),
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
        # images maps the digest file of the image (e.g. "//containers/foo:image.digest") to the repository of the
        # image. "image: <repository>" in the manifest is replaced with "image: <repository>@<digest>", and the job which
        # uses the image is named with the suffix of the first 6 characters of the digest.
        "images": attr.label_keyed_string_dict(allow_files = True),
        "_kustomize": attr.label(
            default = "@kustomize//:toolchain",
        ),
    },
    toolchains = ["@aspect_bazel_lib//lib:yq_toolchain_type"],
)
