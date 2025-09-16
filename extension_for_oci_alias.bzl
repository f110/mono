load("@rules_oci//oci/private:pull.bzl", "oci_alias")
# -- load statements -- #

def _extension_for_oci_alias_impl(ctx):
  oci_alias(
    name = "com_google_distroless_base",
    scheme = "https",
    registry = "gcr.io",
    repository = "distroless/base",
    identifier = "sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
    platform = "//external:com_google_distroless_base_single",
    target_name = "com_google_distroless_base",
    reproducible = True,
  )
# -- repo definitions -- #

extension_for_oci_alias = module_extension(implementation = _extension_for_oci_alias_impl)
