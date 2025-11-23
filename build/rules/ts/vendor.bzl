load("@aspect_rules_js//js:providers.bzl", "JsInfo")
load("@bazel_skylib//lib:shell.bzl", "shell")

def _vendor_ts_impl(ctx):
  generated = ctx.attr.src[JsInfo].types.to_list()
  generated.extend(ctx.attr.src[JsInfo].sources.to_list())
  files = [v.path for v in generated]

  out = ctx.actions.declare_file(ctx.label.name + ".sh")
  substitutions = {
    "@@FROM@@": shell.array_literal(files),
    "@@TO@@": shell.quote(ctx.attr.dir),
  }
  ctx.actions.expand_template(
    template = ctx.file._template,
    output = out,
    substitutions = substitutions,
    is_executable = True,
  )
  runfiles = ctx.runfiles(files = generated)
  return [
    DefaultInfo(
      runfiles = runfiles,
      executable = out
    )
  ]

_vendor_ts = rule(
    implementation = _vendor_ts_impl,
    executable = True,
    attrs = {
        "dir": attr.string(),
        "src": attr.label(),
        "_template": attr.label(
            default = "//build/rules/ts:move-into-workspace.bash",
            allow_single_file = True,
        ),
    }
)

def vendor_ts(name, **kwargs):
    if not "dir" in kwargs:
        dir = native.package_name()
        kwargs["dir"] = dir

    _vendor_ts(name = name, **kwargs)
