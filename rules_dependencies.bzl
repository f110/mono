load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("//build/rules/bazel:def.bzl", "rule_on_github")

rules = {
    "io_bazel_rules_go": rule_on_github("rules_go", "bazelbuild/rules_go", "v0.49.0", "d93ef02f1e72c82d8bb3d5169519b36167b33cf68c252525e3b9d3d5dd143de7", archive = "zip"),
    "bazel_gazelle": rule_on_github("bazel-gazelle", "bazelbuild/bazel-gazelle", "v0.37.0", "d76bf7a60fd8b050444090dfa2837a4eaf9829e1165618ee35dceca5cbdf58d5"),
    "rules_proto": rule_on_github("rules_proto", "bazelbuild/rules_proto", "6.0.0", "303e86e722a520f6f326a50b41cfc16b98fe6d1955ce46642a5b7a67c11c0f5d", strip_prefix = "rules_proto-6.0.0"),
    "rules_oci": rule_on_github("rules_oci", "bazel-contrib/rules_oci", "v1.7.2", "cf6b8be82cde30daef18a09519d75269650317e40d917c8633cf8e3ab5645ea5", strip_prefix = "rules_oci-1.7.2"),
    "rules_pkg": rule_on_github("rules_pkg", "bazelbuild/rules_pkg", "0.9.1", "8f9ee2dc10c1ae514ee599a8b42ed99fa262b757058f65ad3c384289ff70c4b8"),
    "rules_python": rule_on_github("rules_python", "bazelbuild/rules_python", "0.26.0", "9d04041ac92a0985e344235f5d946f71ac543f1b1565f2cdbc9a2aaee8adf55b", strip_prefix = "rules_python-0.26.0", type = "tag"),
    "rules_foreign_cc": rule_on_github("rules_foreign_cc", "bazelbuild/rules_foreign_cc", "0.5.1", "33a5690733c5cc2ede39cb62ebf89e751f2448e27f20c8b2fbbc7d136b166804", strip_prefix = "rules_foreign_cc-0.5.1", type = "tag"),
    "com_google_protobuf": rule_on_github("com_google_protobuf", "protocolbuffers/protobuf", "v3.21.1", "a295dd3b9551d3e2749a9969583dea110c6cdcc39d02088f7c7bb1100077e081", strip_prefix = "protobuf-3.21.1", type = "tag"),
}

def rules_dependencies():
    for k, v in rules.items():
        http_archive(
            name = k,
            sha256 = v["sha256"],
            urls = v["urls"],
            strip_prefix = v["strip_prefix"],
        )
