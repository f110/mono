load("@rules_oci//oci/private:pull.bzl", "oci_pull")
# -- load statements -- #

def _extension_for_oci_pull_impl(ctx):
  oci_pull(
    name = "com_google_distroless_base_single",
    scheme = "https",
    registry = "gcr.io",
    repository = "distroless/base",
    identifier = "sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
    target_name = "com_google_distroless_base_single",
    bazel_tags = [  ],
  )
# -- repo definitions -- #

extension_for_oci_pull = module_extension(implementation = _extension_for_oci_pull_impl)
