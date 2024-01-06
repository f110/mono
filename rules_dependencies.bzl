load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("//build/rules/bazel:def.bzl", "rule_on_github")

rules = {
    "io_bazel_rules_go": rule_on_github("rules_go", "bazelbuild/rules_go", "v0.44.2", "7c76d6236b28ff695aa28cf35f95de317a9472fd1fb14ac797c9bf684f09b37c", archive = "zip"),
    "bazel_gazelle": rule_on_github("bazel-gazelle", "bazelbuild/bazel-gazelle", "v0.34.0", "b7387f72efb59f876e4daae42f1d3912d0d45563eac7cb23d1de0b094ab588cf"),
    "rules_oci": rule_on_github("rules_oci", "bazel-contrib/rules_oci", "v1.4.3", "d41d0ba7855f029ad0e5ee35025f882cbe45b0d5d570842c52704f7a47ba8668", strip_prefix = "rules_oci-1.4.3"),
    "rules_pkg": rule_on_github("rules_pkg", "bazelbuild/rules_pkg", "0.9.1", "8f9ee2dc10c1ae514ee599a8b42ed99fa262b757058f65ad3c384289ff70c4b8"),
    "rules_python": rule_on_github("rules_python", "bazelbuild/rules_python", "0.26.0", "9d04041ac92a0985e344235f5d946f71ac543f1b1565f2cdbc9a2aaee8adf55b", strip_prefix = "rules_python-0.26.0", type = "tag"),
    "rules_foreign_cc": rule_on_github("rules_foreign_cc", "bazelbuild/rules_foreign_cc", "0.5.1", "33a5690733c5cc2ede39cb62ebf89e751f2448e27f20c8b2fbbc7d136b166804", strip_prefix = "rules_foreign_cc-0.5.1", type = "tag"),
    "com_google_protobuf": rule_on_github("com_google_protobuf", "protocolbuffers/protobuf", "v3.21.1", "a295dd3b9551d3e2749a9969583dea110c6cdcc39d02088f7c7bb1100077e081", strip_prefix = "protobuf-3.21.1"),
}

def rules_dependencies():
    for k, v in rules.items():
        http_archive(
            name = k,
            sha256 = v["sha256"],
            urls = v["urls"],
            strip_prefix = v["strip_prefix"],
        )
