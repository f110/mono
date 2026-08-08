"""cd_artifact marks deliverable build outputs so the Build CI collect sidecar can
find and store them.

Building a cd_artifact target materializes every referenced artifact plus a JSON
marker (`<name>.cd_artifact.json`) into the output tree. The sidecar walks the
shared Bazel output base, discovers the markers, computes each artifact's hash and
uploads new ones to the builder. See docs/working/artifact-collection.md.
"""

CdArtifactInfo = provider(
    doc = "Describes a set of deliverable artifacts wrapped by cd_artifact.",
    fields = {
        "marker": "File: the JSON marker describing the artifacts.",
        "artifacts": "depset[File]: the artifact files/directories to be collected.",
    },
)

def _cd_artifact_impl(ctx):
    repo = ctx.attr.repository
    tag = ctx.attr.tag
    entries = []
    inputs = []

    # oci_image targets produce a single OCI layout directory as their default
    # output. The sidecar reads it to derive the image manifest digest.
    for t in ctx.attr.images:
        files = t[DefaultInfo].files.to_list()
        if len(files) != 1:
            fail("cd_artifact: image %s must produce exactly one output (an OCI layout), got %d" % (t.label, len(files)))
        d = files[0]
        entries.append({"type": "container_image", "path": d.path, "repository": repo, "tag": tag, "format": "oci_layout"})
        inputs.append(d)
    for f in ctx.files.files:
        entries.append({"type": "binary", "path": f.path, "repository": repo, "tag": tag})
        inputs.append(f)
    for f in ctx.files.manifests:
        entries.append({"type": "k8s_manifest", "path": f.path, "repository": repo, "tag": tag})
        inputs.append(f)

    # Paths are exec-root relative (File.path). The sidecar resolves them against
    # the exec root it derives from the marker's own location.
    marker = ctx.actions.declare_file(ctx.label.name + ".cd_artifact.json")
    ctx.actions.write(
        output = marker,
        content = json.encode({"target": str(ctx.label), "artifacts": entries}),
    )

    artifacts = depset(inputs)
    return [
        DefaultInfo(files = depset([marker], transitive = [artifacts])),
        CdArtifactInfo(marker = marker, artifacts = artifacts),
    ]

cd_artifact = rule(
    implementation = _cd_artifact_impl,
    doc = "Wraps deliverable targets/files so the Build CI collects them into the OCI registry.",
    attrs = {
        "repository": attr.string(
            mandatory = True,
            doc = "Destination repository path in the OCI registry (zot).",
        ),
        "tag": attr.string(
            doc = "Optional reference tag. When empty, the content digest is used as the reference.",
        ),
        "images": attr.label_list(
            doc = "oci_image targets. Stored as container images (hash = image manifest digest).",
        ),
        "files": attr.label_list(
            allow_files = True,
            doc = "Binary files. Stored as OCI artifacts (hash = sha256 of the file).",
        ),
        "manifests": attr.label_list(
            allow_files = True,
            doc = "Kubernetes manifest files. Stored as OCI artifacts (hash = sha256 of the file).",
        ),
    },
)
