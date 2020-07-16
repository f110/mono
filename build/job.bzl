def _job_impl(ctx):
    pass

job = rule(
    implementation = _job_impl,
    attrs = {
        "target": attr.label(
            doc = "target is the label of target for job. This value should be the full path not relative path.",
        ),
        "targets": attr.string(doc = "(e.g. //...)"),
        "command": attr.string(default = "run"),
        "all_revision": attr.bool(doc = "If true, build at each revision."),
        "args": attr.string_list(),
        "github_status": attr.bool(),
        "cpu_limit": attr.string(doc = "Limit of cpu"),
        "memory_limit": attr.string(doc = "Limit of memory"),
    },
)
