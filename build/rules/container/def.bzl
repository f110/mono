load("@rules_oci//oci:defs.bzl", "oci_image", "oci_load")

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

    oci_load(
        name = "%s.tar" % name,
        image = ":%s" % name,
        repo_tags = ["%s:image" % name],
        visibility = ["//visibility:public"],
    )
