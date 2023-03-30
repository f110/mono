load("@rules_oci//oci:defs.bzl", "oci_image")

def container_image(name, annotations = {}, architecture = "", base = None, cmd = [], entrypoint = [], env = {}, labels = {}, os = "", tars = [], user = "", variant = "", workdir = ""):
    oci_image(
        name = name,
        annotations = annotations,
        architecture = architecture,
        base = base,
        cmd = cmd,
        entrypoint = entrypoint,
        env = env,
        labels = labels,
        os = os,
        tars = tars,
        user = user,
        variant = variant,
        workdir = workdir,
        visibility = ["//visibility:public"],
    )

    native.genrule(
        name = "%s.gen_digest" % name,
        srcs = [":%s" % name],
        outs = ["%s.digest" % name],
        cmd = "$(JQ_BIN) -r '.manifests[0].digest' $(location :%s)/index.json > $@" % name,
        toolchains = ["@jq_toolchains//:resolved_toolchain"],
    )
