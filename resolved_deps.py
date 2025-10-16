resolved = [
     {
          "original_rule_class": "local_repository",
          "original_attributes": {
               "name": "bazel_tools",
               "path": "/var/tmp/_bazel_dexter/install/2b043e6253d7abff533067974426e4a8/embedded_tools"
          },
          "native": "local_repository(name = \"bazel_tools\", path = __embedded_dir__ + \"/\" + \"embedded_tools\")"
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository io_bazel_rules_go instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "io_bazel_rules_go",
               "generator_name": "io_bazel_rules_go",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip",
                    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip"
               ],
               "sha256": "d93ef02f1e72c82d8bb3d5169519b36167b33cf68c252525e3b9d3d5dd143de7",
               "strip_prefix": ""
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip",
                              "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip"
                         ],
                         "sha256": "d93ef02f1e72c82d8bb3d5169519b36167b33cf68c252525e3b9d3d5dd143de7",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "io_bazel_rules_go"
                    },
                    "output_tree_hash": "6bb67c31356a0c5c1a2458936a999aebd51bcdaa3c89f25dd2e8ca4563a9a7dc"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository com_google_protobuf instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "com_google_protobuf",
               "generator_name": "com_google_protobuf",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz",
                    "https://mirror.bucket.x.f110.dev/github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz"
               ],
               "sha256": "a295dd3b9551d3e2749a9969583dea110c6cdcc39d02088f7c7bb1100077e081",
               "strip_prefix": "protobuf-3.21.1"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz",
                              "https://mirror.bucket.x.f110.dev/github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz"
                         ],
                         "sha256": "a295dd3b9551d3e2749a9969583dea110c6cdcc39d02088f7c7bb1100077e081",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "protobuf-3.21.1",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "com_google_protobuf"
                    },
                    "output_tree_hash": "240e1ad4e92c3ae22844295ef478cb459d49aa461feae4d8d45c78e3afcf389d"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository bazel_gazelle instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle",
               "generator_name": "bazel_gazelle",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz",
                    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz"
               ],
               "sha256": "8ad77552825b078a10ad960bec6ef77d2ff8ec70faef2fd038db713f410f5d87",
               "strip_prefix": ""
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz",
                              "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz"
                         ],
                         "sha256": "8ad77552825b078a10ad960bec6ef77d2ff8ec70faef2fd038db713f410f5d87",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "bazel_gazelle"
                    },
                    "output_tree_hash": "185876bbe45b3d7823c133cc441d9411d047b3880afdafd72e341347bd49d4e8"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_pkg instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_pkg",
               "generator_name": "rules_pkg",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz",
                    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz"
               ],
               "sha256": "d20c951960ed77cb7b341c2a59488534e494d5ad1d30c4818c736d57772a9fef",
               "strip_prefix": ""
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz",
                              "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz"
                         ],
                         "sha256": "d20c951960ed77cb7b341c2a59488534e494d5ad1d30c4818c736d57772a9fef",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_pkg"
                    },
                    "output_tree_hash": "b6b3fbe6552389be93f802344544406e8fd9cb2fdb4551563412a97d74c2ecd1"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_oci instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_oci",
               "generator_name": "rules_oci",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazel-contrib/rules_oci/releases/download/v2.2.1/rules_oci-v2.2.1.tar.gz",
                    "https://mirror.bucket.x.f110.dev/github.com/bazel-contrib/rules_oci/releases/download/v2.2.1/rules_oci-v2.2.1.tar.gz"
               ],
               "sha256": "cfea16076ebbec1faea494882ab97d94b1a62d6bcd5aceabad8f95ea0d0a1361",
               "strip_prefix": "rules_oci-2.2.1"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazel-contrib/rules_oci/releases/download/v2.2.1/rules_oci-v2.2.1.tar.gz",
                              "https://mirror.bucket.x.f110.dev/github.com/bazel-contrib/rules_oci/releases/download/v2.2.1/rules_oci-v2.2.1.tar.gz"
                         ],
                         "sha256": "cfea16076ebbec1faea494882ab97d94b1a62d6bcd5aceabad8f95ea0d0a1361",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_oci-2.2.1",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_oci"
                    },
                    "output_tree_hash": "705803ac474d4cc63dd7a4bf9912d855b350b3a0d0d8e8bab41e538397fd3644"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository bazel_features instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:41:23: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/dependencies.bzl:30:17: in rules_oci_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/dependencies.bzl:11:10: in http_archive\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_features",
               "generator_name": "bazel_features",
               "generator_function": "rules_oci_dependencies",
               "generator_location": None,
               "url": "https://github.com/bazel-contrib/bazel_features/releases/download/v1.10.0/bazel_features-v1.10.0.tar.gz",
               "sha256": "95fb3cfd11466b4cad6565e3647a76f89886d875556a4b827c021525cb2482bb",
               "strip_prefix": "bazel_features-1.10.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "https://github.com/bazel-contrib/bazel_features/releases/download/v1.10.0/bazel_features-v1.10.0.tar.gz",
                         "urls": [],
                         "sha256": "95fb3cfd11466b4cad6565e3647a76f89886d875556a4b827c021525cb2482bb",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "bazel_features-1.10.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "bazel_features"
                    },
                    "output_tree_hash": "876df672835d3decdd084e2b8a40cdf7bd547193ad2f63a7122b1651be4adebc"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository aspect_bazel_lib instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:41:23: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/dependencies.bzl:23:17: in rules_oci_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/dependencies.bzl:11:10: in http_archive\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "aspect_bazel_lib",
               "generator_name": "aspect_bazel_lib",
               "generator_function": "rules_oci_dependencies",
               "generator_location": None,
               "url": "https://github.com/aspect-build/bazel-lib/releases/download/v2.7.2/bazel-lib-v2.7.2.tar.gz",
               "sha256": "a8a92645e7298bbf538aa880131c6adb4cf6239bbd27230f077a00414d58e4ce",
               "strip_prefix": "bazel-lib-2.7.2"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "https://github.com/aspect-build/bazel-lib/releases/download/v2.7.2/bazel-lib-v2.7.2.tar.gz",
                         "urls": [],
                         "sha256": "a8a92645e7298bbf538aa880131c6adb4cf6239bbd27230f077a00414d58e4ce",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "bazel-lib-2.7.2",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "aspect_bazel_lib"
                    },
                    "output_tree_hash": "036f893c548de21d883e3bbe6dc6002bde5d775f71e0f729c109a667d88e22ae"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_foreign_cc instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_foreign_cc",
               "generator_name": "rules_foreign_cc",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_foreign_cc/archive/refs/tags/0.5.1.tar.gz",
                    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_foreign_cc/archive/refs/tags/0.5.1.tar.gz"
               ],
               "sha256": "33a5690733c5cc2ede39cb62ebf89e751f2448e27f20c8b2fbbc7d136b166804",
               "strip_prefix": "rules_foreign_cc-0.5.1"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_foreign_cc/archive/refs/tags/0.5.1.tar.gz",
                              "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_foreign_cc/archive/refs/tags/0.5.1.tar.gz"
                         ],
                         "sha256": "33a5690733c5cc2ede39cb62ebf89e751f2448e27f20c8b2fbbc7d136b166804",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_foreign_cc-0.5.1",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_foreign_cc"
                    },
                    "output_tree_hash": "8e48c4e562ca55fb6316409b5143100f764b94f56f780804548ffdeb545a54f3"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_python instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_python",
               "generator_name": "rules_python",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.26.0.tar.gz",
                    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_python/archive/refs/tags/0.26.0.tar.gz"
               ],
               "sha256": "9d04041ac92a0985e344235f5d946f71ac543f1b1565f2cdbc9a2aaee8adf55b",
               "strip_prefix": "rules_python-0.26.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.26.0.tar.gz",
                              "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_python/archive/refs/tags/0.26.0.tar.gz"
                         ],
                         "sha256": "9d04041ac92a0985e344235f5d946f71ac543f1b1565f2cdbc9a2aaee8adf55b",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_python-0.26.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_python"
                    },
                    "output_tree_hash": "1cd0bdb7a0b481ee2eff2ee20651c34414b9bf8b29378b194128390fd6bbdcd4"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_python//python/private:internal_config_repo.bzl%internal_config_repo",
          "definition_information": "Repository rules_python_internal instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:53:16: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_python/python/repositories.bzl:50:10: in py_repositories\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule internal_config_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_python/python/private/internal_config_repo.bzl:93:39: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_python_internal",
               "generator_name": "rules_python_internal",
               "generator_function": "py_repositories",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@rules_python//python/private:internal_config_repo.bzl%internal_config_repo",
                    "attributes": {
                         "name": "rules_python_internal",
                         "generator_name": "rules_python_internal",
                         "generator_function": "py_repositories",
                         "generator_location": None
                    },
                    "output_tree_hash": "936d6aa32e0a9970a711db1624564d78691e1a806c7f465031b4d04200b90f85"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository bazel_skylib instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:51:12: in go_rules_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_skylib",
               "generator_name": "bazel_skylib",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz",
                    "https://github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz"
               ],
               "sha256": "9f38886a40548c6e96c106b752f242130ee11aaa068a56ba7e56f4511f33e4f2",
               "strip_prefix": ""
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz",
                              "https://github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz"
                         ],
                         "sha256": "9f38886a40548c6e96c106b752f242130ee11aaa068a56ba7e56f4511f33e4f2",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "bazel_skylib"
                    },
                    "output_tree_hash": "812c4109d01140c82f941132d6a7d0e7587683481d7083d01b12a6bd21373f1d"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_python//python/pip_install:pip_repository.bzl%pip_repository",
          "definition_information": "Repository pip_deps instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:57:10: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_python/python/pip.bzl:157:19: in pip_parse\nRepository rule pip_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_python/python/pip_install/pip_repository.bzl:530:33: in <toplevel>\n",
          "original_attributes": {
               "name": "pip_deps",
               "generator_name": "pip_deps",
               "generator_function": "pip_parse",
               "generator_location": None,
               "requirements_lock": "//:requirements.lock"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_python//python/pip_install:pip_repository.bzl%pip_repository",
                    "attributes": {
                         "name": "pip_deps",
                         "generator_name": "pip_deps",
                         "generator_function": "pip_parse",
                         "generator_location": None,
                         "requirements_lock": "//:requirements.lock"
                    },
                    "output_tree_hash": "79c293597ffbd8c0881ff350c1de1d50c9b9f791410a2b8f4498456bb05b5145"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_proto instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_proto",
               "generator_name": "rules_proto",
               "generator_function": "rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_proto/releases/download/6.0.0/rules_proto-6.0.0.tar.gz",
                    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_proto/releases/download/6.0.0/rules_proto-6.0.0.tar.gz"
               ],
               "sha256": "303e86e722a520f6f326a50b41cfc16b98fe6d1955ce46642a5b7a67c11c0f5d",
               "strip_prefix": "rules_proto-6.0.0"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_proto/releases/download/6.0.0/rules_proto-6.0.0.tar.gz",
                              "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_proto/releases/download/6.0.0/rules_proto-6.0.0.tar.gz"
                         ],
                         "sha256": "303e86e722a520f6f326a50b41cfc16b98fe6d1955ce46642a5b7a67c11c0f5d",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_proto-6.0.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_proto"
                    },
                    "output_tree_hash": "c411e2d6cbc3e20544c9bec8b96e7752ebe087c01265d118da3a27959925f899"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
          "definition_information": "Repository rules_java_builtin instantiated at:\n  /DEFAULT.WORKSPACE:12:36: in <toplevel>\nRepository rule local_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/local.bzl:64:35: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_java_builtin",
               "path": "/var/tmp/_bazel_dexter/install/2b043e6253d7abff533067974426e4a8/rules_java"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
                    "attributes": {
                         "name": "rules_java_builtin",
                         "path": "/var/tmp/_bazel_dexter/install/2b043e6253d7abff533067974426e4a8/rules_java"
                    },
                    "output_tree_hash": "23156af102e8441d4b3e5358092fc1dce333786289d48b1df6503ecb8c735cf3"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
          "definition_information": "Repository internal_platforms_do_not_use instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:153:6: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule local_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/local.bzl:64:35: in <toplevel>\n",
          "original_attributes": {
               "name": "internal_platforms_do_not_use",
               "generator_name": "internal_platforms_do_not_use",
               "generator_function": "maybe",
               "generator_location": None,
               "path": "/var/tmp/_bazel_dexter/install/2b043e6253d7abff533067974426e4a8/platforms"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:local.bzl%local_repository",
                    "attributes": {
                         "name": "internal_platforms_do_not_use",
                         "generator_name": "internal_platforms_do_not_use",
                         "generator_function": "maybe",
                         "generator_location": None,
                         "path": "/var/tmp/_bazel_dexter/install/2b043e6253d7abff533067974426e4a8/platforms"
                    },
                    "output_tree_hash": "db797f5ddb49595460e727f2c71af1b3adfed4d65132bbe31bd9d3a06bd95dba"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_features//private:version_repo.bzl%version_repo",
          "definition_information": "Repository bazel_features_version instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:110:24: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_features/deps.bzl:8:25: in bazel_features_deps\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_features/private/repos.bzl:9:10: in bazel_features_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule version_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_features/private/version_repo.bzl:20:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_features_version",
               "generator_name": "bazel_features_version",
               "generator_function": "oci_register_toolchains",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_features//private:version_repo.bzl%version_repo",
                    "attributes": {
                         "name": "bazel_features_version",
                         "generator_name": "bazel_features_version",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None
                    },
                    "output_tree_hash": "3decd24bdcdc47289a24d50fde7e86e866fe7cac84c50b8b551e4ab637034ffa"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_features//private:globals_repo.bzl%globals_repo",
          "definition_information": "Repository bazel_features_globals instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:110:24: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_features/deps.bzl:8:25: in bazel_features_deps\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_features/private/repos.bzl:13:10: in bazel_features_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule globals_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_features/private/globals_repo.bzl:36:31: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_features_globals",
               "generator_name": "bazel_features_globals",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "globals": {
                    "RunEnvironmentInfo": "5.3.0",
                    "DefaultInfo": "0.0.1",
                    "__TestingOnly_NeverAvailable": "1000000000.0.0"
               }
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_features//private:globals_repo.bzl%globals_repo",
                    "attributes": {
                         "name": "bazel_features_globals",
                         "generator_name": "bazel_features_globals",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "globals": {
                              "RunEnvironmentInfo": "5.3.0",
                              "DefaultInfo": "0.0.1",
                              "__TestingOnly_NeverAvailable": "1000000000.0.0"
                         }
                    },
                    "output_tree_hash": "41268d749251f28d1d7d332ee107336be74362383e0eb4677f445966421e7628"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_go//go/private:nogo.bzl%go_register_nogo",
          "definition_information": "Repository io_bazel_rules_nogo instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:292:12: in go_rules_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe\nRepository rule go_register_nogo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/nogo.bzl:54:35: in <toplevel>\n",
          "original_attributes": {
               "name": "io_bazel_rules_nogo",
               "generator_name": "io_bazel_rules_nogo",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "nogo": "@rules_go//:default_nogo"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_go//go/private:nogo.bzl%go_register_nogo",
                    "attributes": {
                         "name": "io_bazel_rules_nogo",
                         "generator_name": "io_bazel_rules_nogo",
                         "generator_function": "go_rules_dependencies",
                         "generator_location": None,
                         "nogo": "@rules_go//:default_nogo"
                    },
                    "output_tree_hash": "b414d771116ea38248c456980d0530fe9f067be3b2788ab130fe8d828a181379"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_go//go/private:polyfill_bazel_features.bzl%polyfill_bazel_features",
          "definition_information": "Repository io_bazel_rules_go_bazel_features instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:298:11: in go_rules_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe\nRepository rule polyfill_bazel_features defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/polyfill_bazel_features.bzl:42:42: in <toplevel>\n",
          "original_attributes": {
               "name": "io_bazel_rules_go_bazel_features",
               "generator_name": "io_bazel_rules_go_bazel_features",
               "generator_function": "go_rules_dependencies",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@rules_go//go/private:polyfill_bazel_features.bzl%polyfill_bazel_features",
                    "attributes": {
                         "name": "io_bazel_rules_go_bazel_features",
                         "generator_name": "io_bazel_rules_go_bazel_features",
                         "generator_function": "go_rules_dependencies",
                         "generator_location": None
                    },
                    "output_tree_hash": "6155013dadfc43721a75bda2302470359ed5f14d26007b34ff6749ed298cb521"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:git.bzl%git_repository",
          "definition_information": "Repository dev_f110_protoc_ddl instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:9:15: in <toplevel>\nRepository rule git_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/git.bzl:189:33: in <toplevel>\n",
          "original_attributes": {
               "name": "dev_f110_protoc_ddl",
               "remote": "https://github.com/f110/protoc-ddl",
               "commit": "1cb0fefe60f4aeecc458a2f48abbc4a4e59f637f"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:git.bzl%git_repository",
                    "attributes": {
                         "remote": "https://github.com/f110/protoc-ddl",
                         "commit": "1cb0fefe60f4aeecc458a2f48abbc4a4e59f637f",
                         "shallow_since": "",
                         "init_submodules": False,
                         "recursive_init_submodules": False,
                         "verbose": False,
                         "strip_prefix": "",
                         "patches": [],
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "dev_f110_protoc_ddl"
                    },
                    "output_tree_hash": "3a29454ad828cc80802e149f35322d086497006af849248a153395d1c5ded9dd"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:git.bzl%git_repository",
          "definition_information": "Repository dev_f110_kubeproto instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:15:15: in <toplevel>\nRepository rule git_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/git.bzl:189:33: in <toplevel>\n",
          "original_attributes": {
               "name": "dev_f110_kubeproto",
               "remote": "https://github.com/f110/kubeproto",
               "commit": "90d00e364ad040d388c54b32c9ac3d85604bc6ec"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:git.bzl%git_repository",
                    "attributes": {
                         "remote": "https://github.com/f110/kubeproto",
                         "commit": "90d00e364ad040d388c54b32c9ac3d85604bc6ec",
                         "shallow_since": "",
                         "init_submodules": False,
                         "recursive_init_submodules": False,
                         "verbose": False,
                         "strip_prefix": "",
                         "patches": [],
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "dev_f110_kubeproto"
                    },
                    "output_tree_hash": "a1575766d7b6351f778c0ed11d67f7c68fe7b80a83b2eabd45af46e800c29535"
               }
          ]
     },
     {
          "original_rule_class": "@@internal_platforms_do_not_use//host:extension.bzl%host_platform_repo",
          "definition_information": "Repository host_platform instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:165:6: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule host_platform_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/internal_platforms_do_not_use/host/extension.bzl:51:37: in <toplevel>\n",
          "original_attributes": {
               "name": "host_platform",
               "generator_name": "host_platform",
               "generator_function": "maybe",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@internal_platforms_do_not_use//host:extension.bzl%host_platform_repo",
                    "attributes": {
                         "name": "host_platform",
                         "generator_name": "host_platform",
                         "generator_function": "maybe",
                         "generator_location": None
                    },
                    "output_tree_hash": "dcbfb61be394f2b4fd27f49c2c538d0d87564c5c8baad14a0079063212442538"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository platforms instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:261:12: in go_rules_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "platforms",
               "generator_name": "platforms",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/platforms/releases/download/0.0.10/platforms-0.0.10.tar.gz",
                    "https://github.com/bazelbuild/platforms/releases/download/0.0.10/platforms-0.0.10.tar.gz"
               ],
               "sha256": "218efe8ee736d26a3572663b374a253c012b716d8af0c07e842e82f238a0a7ee"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/platforms/releases/download/0.0.10/platforms-0.0.10.tar.gz",
                              "https://github.com/bazelbuild/platforms/releases/download/0.0.10/platforms-0.0.10.tar.gz"
                         ],
                         "sha256": "218efe8ee736d26a3572663b374a253c012b716d8af0c07e842e82f238a0a7ee",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "platforms"
                    },
                    "output_tree_hash": "ad99d2c6fedcf39b61c65aa76de4b7f5f28d7460bcf26f99ac64a4be9d3616e2"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:tar_toolchain.bzl%tar_toolchains_repo",
          "definition_information": "Repository bsd_tar_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:112:28: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:103:24: in register_tar_toolchains\nRepository rule tar_toolchains_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/tar_toolchain.bzl:190:38: in <toplevel>\n",
          "original_attributes": {
               "name": "bsd_tar_toolchains",
               "generator_name": "bsd_tar_toolchains",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "user_repository_name": "bsd_tar"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:tar_toolchain.bzl%tar_toolchains_repo",
                    "attributes": {
                         "name": "bsd_tar_toolchains",
                         "generator_name": "bsd_tar_toolchains",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "user_repository_name": "bsd_tar"
                    },
                    "output_tree_hash": "529814415e09be0642ac42d978ddb338932daf5b02c54ff5022b34e00bcc2b5f"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:jq_toolchain.bzl%jq_toolchains_repo",
          "definition_information": "Repository jq_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:111:27: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:52:23: in register_jq_toolchains\nRepository rule jq_toolchains_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/jq_toolchain.bzl:158:37: in <toplevel>\n",
          "original_attributes": {
               "name": "jq_toolchains",
               "generator_name": "jq_toolchains",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "user_repository_name": "jq"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:jq_toolchain.bzl%jq_toolchains_repo",
                    "attributes": {
                         "name": "jq_toolchains",
                         "generator_name": "jq_toolchains",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "user_repository_name": "jq"
                    },
                    "output_tree_hash": "372ff19b7076ae4fd0acf82cdf5ef3fc4d769f6f3692094bd3d7c212557e3347"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_oci//oci/private:toolchains_repo.bzl%toolchains_repo",
          "definition_information": "Repository oci_crane_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:116:30: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:132:20: in register_crane_toolchains\nRepository rule toolchains_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/private/toolchains_repo.bzl:139:34: in <toplevel>\n",
          "original_attributes": {
               "name": "oci_crane_toolchains",
               "generator_name": "oci_crane_toolchains",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "toolchain": "@oci_crane_{platform}//:crane_toolchain",
               "toolchain_type": "@rules_oci//oci:crane_toolchain_type"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_oci//oci/private:toolchains_repo.bzl%toolchains_repo",
                    "attributes": {
                         "name": "oci_crane_toolchains",
                         "generator_name": "oci_crane_toolchains",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "toolchain": "@oci_crane_{platform}//:crane_toolchain",
                         "toolchain_type": "@rules_oci//oci:crane_toolchain_type"
                    },
                    "output_tree_hash": "1e7a05161eff2f5a204f6c9c600609b186b40fd2e3b44be624e4d17fc89f502f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_foreign_cc//foreign_cc/private/framework:toolchain.bzl%framework_toolchain_repository",
          "definition_information": "Repository rules_foreign_cc_framework_toolchain_linux_commands instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:49:30: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/repositories.bzl:46:34: in rules_foreign_cc_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/private/framework/toolchain.bzl:97:39: in register_framework_toolchains\nRepository rule framework_toolchain_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/private/framework/toolchain.bzl:71:49: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_foreign_cc_framework_toolchain_linux_commands",
               "generator_name": "rules_foreign_cc_framework_toolchain_linux_commands",
               "generator_function": "rules_foreign_cc_dependencies",
               "generator_location": None,
               "commands_src": "@@rules_foreign_cc//foreign_cc/private/framework/toolchains:linux_commands.bzl",
               "exec_compatible_with": [
                    "@platforms//os:linux"
               ],
               "target_compatible_with": []
          },
          "repositories": [
               {
                    "rule_class": "@@rules_foreign_cc//foreign_cc/private/framework:toolchain.bzl%framework_toolchain_repository",
                    "attributes": {
                         "name": "rules_foreign_cc_framework_toolchain_linux_commands",
                         "generator_name": "rules_foreign_cc_framework_toolchain_linux_commands",
                         "generator_function": "rules_foreign_cc_dependencies",
                         "generator_location": None,
                         "commands_src": "@@rules_foreign_cc//foreign_cc/private/framework/toolchains:linux_commands.bzl",
                         "exec_compatible_with": [
                              "@platforms//os:linux"
                         ],
                         "target_compatible_with": []
                    },
                    "output_tree_hash": "14701a54e90ec3423333b79483281b150e22099d5379f1edcde0adba899fdd23"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:coreutils_toolchain.bzl%coreutils_toolchains_repo",
          "definition_information": "Repository coreutils_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:113:34: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:227:30: in register_coreutils_toolchains\nRepository rule coreutils_toolchains_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/coreutils_toolchain.bzl:166:44: in <toplevel>\n",
          "original_attributes": {
               "name": "coreutils_toolchains",
               "generator_name": "coreutils_toolchains",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "user_repository_name": "coreutils"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:coreutils_toolchain.bzl%coreutils_toolchains_repo",
                    "attributes": {
                         "name": "coreutils_toolchains",
                         "generator_name": "coreutils_toolchains",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "user_repository_name": "coreutils"
                    },
                    "output_tree_hash": "3d7d9da7c7d0de15f4f85a8b597350c9d2c68f86181925f1c827a516288f4fb3"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_go//go/private:sdk.bzl%go_multiple_toolchains",
          "definition_information": "Repository go_sdk_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:25:23: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/sdk.bzl:725:28: in go_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/sdk.bzl:319:19: in go_download_sdk\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/sdk.bzl:307:27: in _go_toolchains\nRepository rule go_multiple_toolchains defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/sdk.bzl:294:41: in <toplevel>\n",
          "original_attributes": {
               "name": "go_sdk_toolchains",
               "generator_name": "go_sdk_toolchains",
               "generator_function": "go_register_toolchains",
               "generator_location": None,
               "prefixes": [
                    ""
               ],
               "sdk_repos": [
                    "go_sdk"
               ],
               "sdk_types": [
                    "remote"
               ],
               "sdk_versions": [
                    "1.24.5"
               ],
               "geese": [
                    ""
               ],
               "goarchs": [
                    ""
               ]
          },
          "repositories": [
               {
                    "rule_class": "@@rules_go//go/private:sdk.bzl%go_multiple_toolchains",
                    "attributes": {
                         "name": "go_sdk_toolchains",
                         "generator_name": "go_sdk_toolchains",
                         "generator_function": "go_register_toolchains",
                         "generator_location": None,
                         "prefixes": [
                              ""
                         ],
                         "sdk_repos": [
                              "go_sdk"
                         ],
                         "sdk_types": [
                              "remote"
                         ],
                         "sdk_versions": [
                              "1.24.5"
                         ],
                         "geese": [
                              ""
                         ],
                         "goarchs": [
                              ""
                         ]
                    },
                    "output_tree_hash": "8e34f54d52d68d815d3a287e43576f3e892597b36ee46a73b1978e91914e43ec"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_oci//oci/private:toolchains_repo.bzl%toolchains_repo",
          "definition_information": "Repository oci_regctl_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:117:31: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:151:20: in register_regctl_toolchains\nRepository rule toolchains_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/private/toolchains_repo.bzl:139:34: in <toplevel>\n",
          "original_attributes": {
               "name": "oci_regctl_toolchains",
               "generator_name": "oci_regctl_toolchains",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "toolchain": "@oci_regctl_{platform}//:regctl_toolchain",
               "toolchain_type": "@rules_oci//oci:regctl_toolchain_type"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_oci//oci/private:toolchains_repo.bzl%toolchains_repo",
                    "attributes": {
                         "name": "oci_regctl_toolchains",
                         "generator_name": "oci_regctl_toolchains",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "toolchain": "@oci_regctl_{platform}//:regctl_toolchain",
                         "toolchain_type": "@rules_oci//oci:regctl_toolchain_type"
                    },
                    "output_tree_hash": "8a09c06bf763ba6925164dd14bf09af0a53eb646a2d944d50d6f09f5a92a9a25"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:copy_to_directory_toolchain.bzl%copy_to_directory_toolchains_repo",
          "definition_information": "Repository copy_to_directory_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:114:42: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:297:38: in register_copy_to_directory_toolchains\nRepository rule copy_to_directory_toolchains_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/copy_to_directory_toolchain.bzl:146:52: in <toplevel>\n",
          "original_attributes": {
               "name": "copy_to_directory_toolchains",
               "generator_name": "copy_to_directory_toolchains",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "user_repository_name": "copy_to_directory"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:copy_to_directory_toolchain.bzl%copy_to_directory_toolchains_repo",
                    "attributes": {
                         "name": "copy_to_directory_toolchains",
                         "generator_name": "copy_to_directory_toolchains",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "user_repository_name": "copy_to_directory"
                    },
                    "output_tree_hash": "bbe208edab7e408878068af72042aef50aa472366a194f0817af359f1d572110"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:zstd_toolchain.bzl%zstd_toolchains_repo",
          "definition_information": "Repository zstd_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:115:29: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:126:25: in register_zstd_toolchains\nRepository rule zstd_toolchains_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/zstd_toolchain.bzl:171:39: in <toplevel>\n",
          "original_attributes": {
               "name": "zstd_toolchains",
               "generator_name": "zstd_toolchains",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "user_repository_name": "zstd"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:zstd_toolchain.bzl%zstd_toolchains_repo",
                    "attributes": {
                         "name": "zstd_toolchains",
                         "generator_name": "zstd_toolchains",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "user_repository_name": "zstd"
                    },
                    "output_tree_hash": "a3cbead29d08c4ca763d6fb73b5d432a67813a0d21982df9812f0500ca3c52b7"
               }
          ]
     },
     {
          "original_rule_class": "local_config_platform",
          "original_attributes": {
               "name": "local_config_platform"
          },
          "native": "local_config_platform(name = 'local_config_platform')"
     },
     {
          "original_rule_class": "@@bazel_tools//tools/sh:sh_configure.bzl%sh_config",
          "definition_information": "Repository local_config_sh instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:187:13: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/sh/sh_configure.bzl:83:14: in sh_configure\nRepository rule sh_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/sh/sh_configure.bzl:72:28: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_sh",
               "generator_name": "local_config_sh",
               "generator_function": "sh_configure",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/sh:sh_configure.bzl%sh_config",
                    "attributes": {
                         "name": "local_config_sh",
                         "generator_name": "local_config_sh",
                         "generator_function": "sh_configure",
                         "generator_location": None
                    },
                    "output_tree_hash": "bf05b8c3fcee06daafdb55df31026139d17b735d270ec2298ea58ab65cb6f18d"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_foreign_cc//foreign_cc/private/framework:toolchain.bzl%framework_toolchain_repository",
          "definition_information": "Repository rules_foreign_cc_framework_toolchain_windows_commands instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:49:30: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/repositories.bzl:46:34: in rules_foreign_cc_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/private/framework/toolchain.bzl:97:39: in register_framework_toolchains\nRepository rule framework_toolchain_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/private/framework/toolchain.bzl:71:49: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_foreign_cc_framework_toolchain_windows_commands",
               "generator_name": "rules_foreign_cc_framework_toolchain_windows_commands",
               "generator_function": "rules_foreign_cc_dependencies",
               "generator_location": None,
               "commands_src": "@@rules_foreign_cc//foreign_cc/private/framework/toolchains:windows_commands.bzl",
               "exec_compatible_with": [
                    "@platforms//os:windows"
               ],
               "target_compatible_with": []
          },
          "repositories": [
               {
                    "rule_class": "@@rules_foreign_cc//foreign_cc/private/framework:toolchain.bzl%framework_toolchain_repository",
                    "attributes": {
                         "name": "rules_foreign_cc_framework_toolchain_windows_commands",
                         "generator_name": "rules_foreign_cc_framework_toolchain_windows_commands",
                         "generator_function": "rules_foreign_cc_dependencies",
                         "generator_location": None,
                         "commands_src": "@@rules_foreign_cc//foreign_cc/private/framework/toolchains:windows_commands.bzl",
                         "exec_compatible_with": [
                              "@platforms//os:windows"
                         ],
                         "target_compatible_with": []
                    },
                    "output_tree_hash": "18718fc7d62ff8baee61fc4bab32ab53d779f8bba7d766eb2934545c517f16d8"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_foreign_cc//foreign_cc/private/framework:toolchain.bzl%framework_toolchain_repository",
          "definition_information": "Repository rules_foreign_cc_framework_toolchain_macos_commands instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:49:30: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/repositories.bzl:46:34: in rules_foreign_cc_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/private/framework/toolchain.bzl:97:39: in register_framework_toolchains\nRepository rule framework_toolchain_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/private/framework/toolchain.bzl:71:49: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_foreign_cc_framework_toolchain_macos_commands",
               "generator_name": "rules_foreign_cc_framework_toolchain_macos_commands",
               "generator_function": "rules_foreign_cc_dependencies",
               "generator_location": None,
               "commands_src": "@@rules_foreign_cc//foreign_cc/private/framework/toolchains:macos_commands.bzl",
               "exec_compatible_with": [
                    "@platforms//os:macos"
               ],
               "target_compatible_with": []
          },
          "repositories": [
               {
                    "rule_class": "@@rules_foreign_cc//foreign_cc/private/framework:toolchain.bzl%framework_toolchain_repository",
                    "attributes": {
                         "name": "rules_foreign_cc_framework_toolchain_macos_commands",
                         "generator_name": "rules_foreign_cc_framework_toolchain_macos_commands",
                         "generator_function": "rules_foreign_cc_dependencies",
                         "generator_location": None,
                         "commands_src": "@@rules_foreign_cc//foreign_cc/private/framework/toolchains:macos_commands.bzl",
                         "exec_compatible_with": [
                              "@platforms//os:macos"
                         ],
                         "target_compatible_with": []
                    },
                    "output_tree_hash": "7a59b3ce5d74393d8ce6f9643dcbfd42b48c91fc798a98a60caa8cc2156c5324"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
               "generator_name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remote_jdk8_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "c9c795851cffbf2a808bfc7cccea597c3b3fef46cfefa084f7e9de7e90b65447"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_foreign_cc//toolchains:prebuilt_toolchains_repository.bzl%prebuilt_toolchains_repository",
          "definition_information": "Repository ninja_1.10.2_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:49:30: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/repositories.bzl:51:28: in rules_foreign_cc_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/toolchains/prebuilt_toolchains.bzl:64:22: in prebuilt_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/toolchains/prebuilt_toolchains.bzl:2765:14: in _ninja_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule prebuilt_toolchains_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/toolchains/prebuilt_toolchains_repository.bzl:58:49: in <toplevel>\n",
          "original_attributes": {
               "name": "ninja_1.10.2_toolchains",
               "generator_name": "ninja_1.10.2_toolchains",
               "generator_function": "rules_foreign_cc_dependencies",
               "generator_location": None,
               "repos": {
                    "ninja_1.10.2_linux": [
                         "@platforms//cpu:x86_64",
                         "@platforms//os:linux"
                    ],
                    "ninja_1.10.2_mac": [
                         "@platforms//cpu:x86_64",
                         "@platforms//os:macos"
                    ],
                    "ninja_1.10.2_win": [
                         "@platforms//cpu:x86_64",
                         "@platforms//os:windows"
                    ]
               },
               "tool": "ninja"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_foreign_cc//toolchains:prebuilt_toolchains_repository.bzl%prebuilt_toolchains_repository",
                    "attributes": {
                         "name": "ninja_1.10.2_toolchains",
                         "generator_name": "ninja_1.10.2_toolchains",
                         "generator_function": "rules_foreign_cc_dependencies",
                         "generator_location": None,
                         "repos": {
                              "ninja_1.10.2_linux": [
                                   "@platforms//cpu:x86_64",
                                   "@platforms//os:linux"
                              ],
                              "ninja_1.10.2_mac": [
                                   "@platforms//cpu:x86_64",
                                   "@platforms//os:macos"
                              ],
                              "ninja_1.10.2_win": [
                                   "@platforms//cpu:x86_64",
                                   "@platforms//os:windows"
                              ]
                         },
                         "tool": "ninja"
                    },
                    "output_tree_hash": "dbe0afeffb0086ed64cd9b81223e5b0b6c3ac86f2f0039f305d65be439a54061"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_foreign_cc//toolchains:prebuilt_toolchains_repository.bzl%prebuilt_toolchains_repository",
          "definition_information": "Repository cmake_3.21.0_toolchains instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:49:30: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/foreign_cc/repositories.bzl:51:28: in rules_foreign_cc_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/toolchains/prebuilt_toolchains.bzl:63:22: in prebuilt_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/toolchains/prebuilt_toolchains.bzl:135:14: in _cmake_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule prebuilt_toolchains_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_foreign_cc/toolchains/prebuilt_toolchains_repository.bzl:58:49: in <toplevel>\n",
          "original_attributes": {
               "name": "cmake_3.21.0_toolchains",
               "generator_name": "cmake_3.21.0_toolchains",
               "generator_function": "rules_foreign_cc_dependencies",
               "generator_location": None,
               "repos": {
                    "cmake-3.21.0-linux-aarch64": [
                         "@platforms//cpu:aarch64",
                         "@platforms//os:linux"
                    ],
                    "cmake-3.21.0-linux-x86_64": [
                         "@platforms//cpu:x86_64",
                         "@platforms//os:linux"
                    ],
                    "cmake-3.21.0-macos-universal": [
                         "@platforms//os:macos"
                    ],
                    "cmake-3.21.0-windows-i386": [
                         "@platforms//cpu:x86_32",
                         "@platforms//os:windows"
                    ],
                    "cmake-3.21.0-windows-x86_64": [
                         "@platforms//cpu:x86_64",
                         "@platforms//os:windows"
                    ]
               },
               "tool": "cmake"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_foreign_cc//toolchains:prebuilt_toolchains_repository.bzl%prebuilt_toolchains_repository",
                    "attributes": {
                         "name": "cmake_3.21.0_toolchains",
                         "generator_name": "cmake_3.21.0_toolchains",
                         "generator_function": "rules_foreign_cc_dependencies",
                         "generator_location": None,
                         "repos": {
                              "cmake-3.21.0-linux-aarch64": [
                                   "@platforms//cpu:aarch64",
                                   "@platforms//os:linux"
                              ],
                              "cmake-3.21.0-linux-x86_64": [
                                   "@platforms//cpu:x86_64",
                                   "@platforms//os:linux"
                              ],
                              "cmake-3.21.0-macos-universal": [
                                   "@platforms//os:macos"
                              ],
                              "cmake-3.21.0-windows-i386": [
                                   "@platforms//cpu:x86_32",
                                   "@platforms//os:windows"
                              ],
                              "cmake-3.21.0-windows-x86_64": [
                                   "@platforms//cpu:x86_64",
                                   "@platforms//os:windows"
                              ]
                         },
                         "tool": "cmake"
                    },
                    "output_tree_hash": "534035e16c99ac2b445a9758ed43306a4a42b2f2d636188cd26385f8aaf9c1f3"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_linux_toolchain_config_repo",
               "generator_name": "remote_jdk8_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_linux_toolchain_config_repo",
                         "generator_name": "remote_jdk8_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "b6a178fc0ca08a4473490f1c5d0f9f633db0ca0f2834c69dd08ce8290cf9ca86"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf_toolchains",
          "definition_information": "Repository local_config_cc_toolchains instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:181:13: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/cpp/cc_configure.bzl:148:27: in cc_configure\nRepository rule cc_autoconf_toolchains defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/cpp/cc_configure.bzl:47:41: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_cc_toolchains",
               "generator_name": "local_config_cc_toolchains",
               "generator_function": "cc_configure",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf_toolchains",
                    "attributes": {
                         "name": "local_config_cc_toolchains",
                         "generator_name": "local_config_cc_toolchains",
                         "generator_function": "cc_configure",
                         "generator_location": None
                    },
                    "output_tree_hash": "2c6c2998e70208a29847dd5420b3aff0b1e2f5ac956a0911addd090e92b83969"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
               "generator_name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remote_jdk8_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "4d721d8b0731cfb50f963f8b55c7bef9f572de0e2f251f07a12c722ef1acbb2f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_macos_toolchain_config_repo",
               "generator_name": "remote_jdk8_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_macos_toolchain_config_repo",
                         "generator_name": "remote_jdk8_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "e0d82dc2dbe8ec49d859811afe4973ec36226875a39ac7fc8419e91e7e9c89fb"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_linux_s390x_toolchain_config_repo",
               "generator_name": "remote_jdk8_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_linux_s390x_toolchain_config_repo",
                         "generator_name": "remote_jdk8_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "f1e3f0b4884e21863a7c19a3a12a8995ed4162e02bd07cbb61b42799fc2d7359"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remote_jdk8_windows_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:370:22: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:349:34: in remote_jdk8_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remote_jdk8_windows_toolchain_config_repo",
               "generator_name": "remote_jdk8_windows_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remote_jdk8_windows_toolchain_config_repo",
                         "generator_name": "remote_jdk8_windows_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_8\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"8\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remote_jdk8_windows//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "8d0b08c18f215c185d64efe72054a5ffef36325906c34ebf1d3c710d4ba5c685"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "0a170bf4f31e6c4621aeb4d4ce4b75b808be2f3a63cb55dc8172c27707d299ab"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "bef508c068dd47d605f62c53ab0628f1f7f5101fdcc8ada09b2067b36c47931f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_macos_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk11_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk11_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "ca1d067909669aa58188026a7da06d43bdec74a3ba5c122af8a4c3660acd8d8f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_macos_toolchain_config_repo",
               "generator_name": "remotejdk11_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_macos_toolchain_config_repo",
                         "generator_name": "remotejdk11_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "45b3b36d22d3e614745e7a5e838351c32fe0eabb09a4a197bac0f4d416a950ce"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_ppc64le_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_ppc64le_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_ppc64le//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "3272b586976beea589d09ea8029fd5d714da40127c8850e3480991c2440c5825"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_win_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_win_toolchain_config_repo",
               "generator_name": "remotejdk11_win_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_win_toolchain_config_repo",
                         "generator_name": "remotejdk11_win_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "d0587a4ecc9323d5cf65314b2d284b520ffb5ee1d3231cc6601efa13dadcc0f4"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_linux_s390x_toolchain_config_repo",
               "generator_name": "remotejdk11_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_linux_s390x_toolchain_config_repo",
                         "generator_name": "remotejdk11_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "244e11245106a8495ac4744a90023b87008e3e553766ba11d47a9fe5b4bb408d"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "b169b01ac1a169d7eb5e3525454c3e408e9127993ac0f578dc2c5ad183fd4e3e"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk11_win_arm64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:371:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:353:34: in remote_jdk11_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk11_win_arm64_toolchain_config_repo",
               "generator_name": "remotejdk11_win_arm64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk11_win_arm64_toolchain_config_repo",
                         "generator_name": "remotejdk11_win_arm64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_11\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"11\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk11_win_arm64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "c237bd9668de9b6437c452c020ea5bc717ff80b1a5ffd581adfdc7d4a6c5fe03"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_macos_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk17_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk17_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "0eb17f6d969bc665a21e55d29eb51e88a067159ee62cf5094b17658a07d3accb"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "f0f07fe0f645f2dc7b8c9953c7962627e1c7425cc52f543729dbff16cd20e461"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_win_arm64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_win_arm64_toolchain_config_repo",
               "generator_name": "remotejdk21_win_arm64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_win_arm64_toolchain_config_repo",
                         "generator_name": "remotejdk21_win_arm64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win_arm64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "9bbdbb329eeba27bc482582360abc6e3351d9a9a07ee11cba3a0026c90223e85"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_win_arm64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_win_arm64_toolchain_config_repo",
               "generator_name": "remotejdk17_win_arm64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_win_arm64_toolchain_config_repo",
                         "generator_name": "remotejdk17_win_arm64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:arm64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win_arm64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "86b129d9c464a9b08f97eca7d8bc5bdb3676b581f8aac044451dbdfaa49e69d3"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_macos_toolchain_config_repo",
               "generator_name": "remotejdk17_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_macos_toolchain_config_repo",
                         "generator_name": "remotejdk17_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "41aa7b3317f8d9001746e908454760bf544ffaa058abe22f40711246608022ba"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_win_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_win_toolchain_config_repo",
               "generator_name": "remotejdk17_win_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_win_toolchain_config_repo",
                         "generator_name": "remotejdk17_win_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_win//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "170c3c9a35e502555dc9f04b345e064880acbf7df935f673154011356f4aad34"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_macos_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_macos_toolchain_config_repo",
               "generator_name": "remotejdk21_macos_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_macos_toolchain_config_repo",
                         "generator_name": "remotejdk21_macos_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "434446eddb7f6a3dcc7a2a5330ed9ab26579c5142c19866b197475a695fbb32f"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_s390x_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_s390x_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "6ba1870e09fccfdcd423f4169b966a73f8e9deaff859ec6fb3b626ed61ebd8b5"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_s390x_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_s390x_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_s390x_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_s390x_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_s390x_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:s390x\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_s390x//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "30b78e0951c37c2d7ae1318f83045ff42ef261dbb93c5b4fd3ba963e12cf68d6"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "bb33021f243382d2fb849ec204c5c8be5083c37e081df71d34a84324687cf001"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "ee548ad054c9b75286ff3cd19792e433a2d1236378d3a0d8076fca0bb1a88e05"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk17_linux_ppc64le_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:372:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:357:34: in remote_jdk17_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
               "generator_name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
                         "generator_name": "remotejdk17_linux_ppc64le_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_17\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"17\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk17_linux_ppc64le//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "fdc8ae00f2436bfc46b2f54c84f2bd84122787ede232a4d61ffc284bfe6f61ec"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_win_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_win_toolchain_config_repo",
               "generator_name": "remotejdk21_win_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_win_toolchain_config_repo",
                         "generator_name": "remotejdk21_win_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:windows\", \"@platforms//cpu:x86_64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_win//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "87012328b07a779503deec0ef47132a0de50efd69afe7df87619bcc07b1dc4ed"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_linux_ppc64le_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
               "generator_name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
                         "generator_name": "remotejdk21_linux_ppc64le_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:linux\", \"@platforms//cpu:ppc\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_linux_ppc64le//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "7886e497d586c3f3c8225685281b0940e9aa699af208dc98de3db8839e197be3"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
          "definition_information": "Repository remotejdk21_macos_aarch64_toolchain_config_repo instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:93:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:373:23: in rules_java_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:361:34: in remote_jdk21_repos\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/java/repositories.bzl:333:14: in _remote_jdk_repos_for_version\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:57:22: in remote_java_repository\nRepository rule _toolchain_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/remote_java_repository.bzl:27:36: in <toplevel>\n",
          "original_attributes": {
               "name": "remotejdk21_macos_aarch64_toolchain_config_repo",
               "generator_name": "remotejdk21_macos_aarch64_toolchain_config_repo",
               "generator_function": "rules_java_dependencies",
               "generator_location": None,
               "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\n"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:remote_java_repository.bzl%_toolchain_config",
                    "attributes": {
                         "name": "remotejdk21_macos_aarch64_toolchain_config_repo",
                         "generator_name": "remotejdk21_macos_aarch64_toolchain_config_repo",
                         "generator_function": "rules_java_dependencies",
                         "generator_location": None,
                         "build_file": "\nconfig_setting(\n    name = \"prefix_version_setting\",\n    values = {\"java_runtime_version\": \"remotejdk_21\"},\n    visibility = [\"//visibility:private\"],\n)\nconfig_setting(\n    name = \"version_setting\",\n    values = {\"java_runtime_version\": \"21\"},\n    visibility = [\"//visibility:private\"],\n)\nalias(\n    name = \"version_or_prefix_version_setting\",\n    actual = select({\n        \":version_setting\": \":version_setting\",\n        \"//conditions:default\": \":prefix_version_setting\",\n    }),\n    visibility = [\"//visibility:private\"],\n)\ntoolchain(\n    name = \"toolchain\",\n    target_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\ntoolchain(\n    name = \"bootstrap_runtime_toolchain\",\n    # These constraints are not required for correctness, but prevent fetches of remote JDK for\n    # different architectures. As every Java compilation toolchain depends on a bootstrap runtime in\n    # the same configuration, this constraint will not result in toolchain resolution failures.\n    exec_compatible_with = [\"@platforms//os:macos\", \"@platforms//cpu:aarch64\"],\n    target_settings = [\":version_or_prefix_version_setting\"],\n    toolchain_type = \"@bazel_tools//tools/jdk:bootstrap_runtime_toolchain_type\",\n    toolchain = \"@remotejdk21_macos_aarch64//:jdk\",\n)\n"
                    },
                    "output_tree_hash": "706d910cc6809ea7f77fa4f938a4f019dd90d9dad927fb804a14b04321300a36"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:is_bazel_module.bzl%is_bazel_module",
          "definition_information": "Repository bazel_gazelle_is_bazel_module instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:33:21: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:92:20: in gazelle_dependencies\nRepository rule is_bazel_module defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/internal/is_bazel_module.bzl:30:34: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle_is_bazel_module",
               "generator_name": "bazel_gazelle_is_bazel_module",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "is_bazel_module": False
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:is_bazel_module.bzl%is_bazel_module",
                    "attributes": {
                         "name": "bazel_gazelle_is_bazel_module",
                         "generator_name": "bazel_gazelle_is_bazel_module",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "is_bazel_module": False
                    },
                    "output_tree_hash": "dda7e5d5aa9d766c3d9c95f78a31564192edad9cc7e5448bcaeb039f33b87c8f"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_java instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:29:14: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/com_google_protobuf/protobuf_deps.bzl:65:24: in protobuf_deps\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/com_google_protobuf/protobuf_deps.bzl:19:17: in _github_archive\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_java",
               "generator_name": "rules_java",
               "generator_function": "protobuf_deps",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_java/archive/981f06c3d2bd10225e85209904090eb7b5fb26bd.zip"
               ],
               "sha256": "7979ece89e82546b0dcd1dff7538c34b5a6ebc9148971106f0e3705444f00665",
               "strip_prefix": "rules_java-981f06c3d2bd10225e85209904090eb7b5fb26bd"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_java/archive/981f06c3d2bd10225e85209904090eb7b5fb26bd.zip"
                         ],
                         "sha256": "7979ece89e82546b0dcd1dff7538c34b5a6ebc9148971106f0e3705444f00665",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_java-981f06c3d2bd10225e85209904090eb7b5fb26bd",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_java"
                    },
                    "output_tree_hash": "da2ec8d78f7273154d4b7eeb3fe6aeeeddd71330cdc8c5593e3e3d060e0fa465"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_cc instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:29:14: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/com_google_protobuf/protobuf_deps.bzl:57:24: in protobuf_deps\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/com_google_protobuf/protobuf_deps.bzl:19:17: in _github_archive\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_cc",
               "generator_name": "rules_cc",
               "generator_function": "protobuf_deps",
               "generator_location": None,
               "urls": [
                    "https://github.com/bazelbuild/rules_cc/archive/818289e5613731ae410efb54218a4077fb9dbb03.zip"
               ],
               "sha256": "0adbd6f567291ad526e82c765e15aed33cea5e256eeba129f1501142c2c56610",
               "strip_prefix": "rules_cc-818289e5613731ae410efb54218a4077fb9dbb03"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/bazelbuild/rules_cc/archive/818289e5613731ae410efb54218a4077fb9dbb03.zip"
                         ],
                         "sha256": "0adbd6f567291ad526e82c765e15aed33cea5e256eeba129f1501142c2c56610",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "rules_cc-818289e5613731ae410efb54218a4077fb9dbb03",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_cc"
                    },
                    "output_tree_hash": "e2fdc13582a894a6b9cd962971904036e35e61ba22702447a7bb7cae91dd97e9"
               }
          ]
     },
     {
          "original_rule_class": "//build/rules/kind:def.bzl%kind_binary",
          "definition_information": "Repository kind instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:72:24: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:34:16: in repository_dependencies\nRepository rule kind_binary defined at:\n  /Users/dexter/dev/src/github.com/f110/mono/build/rules/kind/def.bzl:9:30: in <toplevel>\n",
          "original_attributes": {
               "name": "kind",
               "generator_name": "kind",
               "generator_function": "repository_dependencies",
               "generator_location": None,
               "version": "0.22.0"
          },
          "repositories": [
               {
                    "rule_class": "//build/rules/kind:def.bzl%kind_binary",
                    "attributes": {
                         "name": "kind",
                         "generator_name": "kind",
                         "generator_function": "repository_dependencies",
                         "generator_location": None,
                         "version": "0.22.0"
                    },
                    "output_tree_hash": "837db7ac52e99e01130d5a1256f2219a4a100f454865b8a8b6418f38ab38e8b2"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_java_builtin//toolchains:local_java_repository.bzl%_local_java_repository_rule",
          "definition_information": "Repository local_jdk instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:85:6: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/local_java_repository.bzl:335:32: in local_java_repository\nRepository rule _local_java_repository_rule defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_java_builtin/toolchains/local_java_repository.bzl:290:46: in <toplevel>\n",
          "original_attributes": {
               "name": "local_jdk",
               "generator_name": "local_jdk",
               "generator_function": "maybe",
               "generator_location": None,
               "build_file_content": "load(\"@rules_java//java:defs.bzl\", \"java_runtime\")\n\npackage(default_visibility = [\"//visibility:public\"])\n\nexports_files([\"WORKSPACE\", \"BUILD.bazel\"])\n\nfilegroup(\n    name = \"jre\",\n    srcs = glob(\n        [\n            \"jre/bin/**\",\n            \"jre/lib/**\",\n        ],\n        allow_empty = True,\n        # In some configurations, Java browser plugin is considered harmful and\n        # common antivirus software blocks access to npjp2.dll interfering with Bazel,\n        # so do not include it in JRE on Windows.\n        exclude = [\"jre/bin/plugin2/**\"],\n    ),\n)\n\nfilegroup(\n    name = \"jdk-bin\",\n    srcs = glob(\n        [\"bin/**\"],\n        # The JDK on Windows sometimes contains a directory called\n        # \"%systemroot%\", which is not a valid label.\n        exclude = [\"**/*%*/**\"],\n    ),\n)\n\n# This folder holds security policies.\nfilegroup(\n    name = \"jdk-conf\",\n    srcs = glob(\n        [\"conf/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-include\",\n    srcs = glob(\n        [\"include/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-lib\",\n    srcs = glob(\n        [\"lib/**\", \"release\"],\n        allow_empty = True,\n        exclude = [\n            \"lib/missioncontrol/**\",\n            \"lib/visualvm/**\",\n        ],\n    ),\n)\n\njava_runtime(\n    name = \"jdk\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jre\",\n    ],\n    # Provide the 'java` binary explicitly so that the correct path is used by\n    # Bazel even when the host platform differs from the execution platform.\n    # Exactly one of the two globs will be empty depending on the host platform.\n    # When --incompatible_disallow_empty_glob is enabled, each individual empty\n    # glob will fail without allow_empty = True, even if the overall result is\n    # non-empty.\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n\nfilegroup(\n    name = \"jdk-jmods\",\n    srcs = glob(\n        [\"jmods/**\"],\n        allow_empty = True,\n    ),\n)\n\njava_runtime(\n    name = \"jdk-with-jmods\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jdk-jmods\",\n        \":jre\",\n    ],\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n",
               "java_home": "",
               "version": ""
          },
          "repositories": [
               {
                    "rule_class": "@@rules_java_builtin//toolchains:local_java_repository.bzl%_local_java_repository_rule",
                    "attributes": {
                         "name": "local_jdk",
                         "generator_name": "local_jdk",
                         "generator_function": "maybe",
                         "generator_location": None,
                         "build_file_content": "load(\"@rules_java//java:defs.bzl\", \"java_runtime\")\n\npackage(default_visibility = [\"//visibility:public\"])\n\nexports_files([\"WORKSPACE\", \"BUILD.bazel\"])\n\nfilegroup(\n    name = \"jre\",\n    srcs = glob(\n        [\n            \"jre/bin/**\",\n            \"jre/lib/**\",\n        ],\n        allow_empty = True,\n        # In some configurations, Java browser plugin is considered harmful and\n        # common antivirus software blocks access to npjp2.dll interfering with Bazel,\n        # so do not include it in JRE on Windows.\n        exclude = [\"jre/bin/plugin2/**\"],\n    ),\n)\n\nfilegroup(\n    name = \"jdk-bin\",\n    srcs = glob(\n        [\"bin/**\"],\n        # The JDK on Windows sometimes contains a directory called\n        # \"%systemroot%\", which is not a valid label.\n        exclude = [\"**/*%*/**\"],\n    ),\n)\n\n# This folder holds security policies.\nfilegroup(\n    name = \"jdk-conf\",\n    srcs = glob(\n        [\"conf/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-include\",\n    srcs = glob(\n        [\"include/**\"],\n        allow_empty = True,\n    ),\n)\n\nfilegroup(\n    name = \"jdk-lib\",\n    srcs = glob(\n        [\"lib/**\", \"release\"],\n        allow_empty = True,\n        exclude = [\n            \"lib/missioncontrol/**\",\n            \"lib/visualvm/**\",\n        ],\n    ),\n)\n\njava_runtime(\n    name = \"jdk\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jre\",\n    ],\n    # Provide the 'java` binary explicitly so that the correct path is used by\n    # Bazel even when the host platform differs from the execution platform.\n    # Exactly one of the two globs will be empty depending on the host platform.\n    # When --incompatible_disallow_empty_glob is enabled, each individual empty\n    # glob will fail without allow_empty = True, even if the overall result is\n    # non-empty.\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n\nfilegroup(\n    name = \"jdk-jmods\",\n    srcs = glob(\n        [\"jmods/**\"],\n        allow_empty = True,\n    ),\n)\n\njava_runtime(\n    name = \"jdk-with-jmods\",\n    srcs = [\n        \":jdk-bin\",\n        \":jdk-conf\",\n        \":jdk-include\",\n        \":jdk-lib\",\n        \":jdk-jmods\",\n        \":jre\",\n    ],\n    java = glob([\"bin/java.exe\", \"bin/java\"], allow_empty = True)[0],\n    version = {RUNTIME_VERSION},\n)\n",
                         "java_home": "",
                         "version": ""
                    },
                    "output_tree_hash": "cf4c308c246a6b0fd021e6fcc8b8740e456db91ed2fed54e9349dc302596d109"
               }
          ]
     },
     {
          "original_rule_class": "//build/rules/kustomize:def.bzl%kustomize_binary",
          "definition_information": "Repository kustomize instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:72:24: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:29:21: in repository_dependencies\nRepository rule kustomize_binary defined at:\n  /Users/dexter/dev/src/github.com/f110/mono/build/rules/kustomize/def.bzl:13:35: in <toplevel>\n",
          "original_attributes": {
               "name": "kustomize",
               "generator_name": "kustomize",
               "generator_function": "repository_dependencies",
               "generator_location": None,
               "version": "v4.5.4"
          },
          "repositories": [
               {
                    "rule_class": "//build/rules/kustomize:def.bzl%kustomize_binary",
                    "attributes": {
                         "name": "kustomize",
                         "generator_name": "kustomize",
                         "generator_function": "repository_dependencies",
                         "generator_location": None,
                         "version": "v4.5.4"
                    },
                    "output_tree_hash": "d178cb212e98c5e131a3d68af11f5ec1cc8c68b4dd06bb3958a5152d798b6298"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:tar_toolchain.bzl%bsdtar_binary_repo",
          "definition_information": "Repository bsd_tar_darwin_arm64 instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:112:28: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:96:27: in register_tar_toolchains\nRepository rule bsdtar_binary_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/tar_toolchain.bzl:88:37: in <toplevel>\n",
          "original_attributes": {
               "name": "bsd_tar_darwin_arm64",
               "generator_name": "bsd_tar_darwin_arm64",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "platform": "darwin_arm64"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:tar_toolchain.bzl%bsdtar_binary_repo",
                    "attributes": {
                         "name": "bsd_tar_darwin_arm64",
                         "generator_name": "bsd_tar_darwin_arm64",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "platform": "darwin_arm64"
                    },
                    "output_tree_hash": "5dd4406324b387d699b683181cb41dcddebd32831e09b98c2cf049709b6724c1"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:coreutils_toolchain.bzl%coreutils_platform_repo",
          "definition_information": "Repository coreutils_darwin_arm64 instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:113:34: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:219:32: in register_coreutils_toolchains\nRepository rule coreutils_platform_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/coreutils_toolchain.bzl:197:42: in <toplevel>\n",
          "original_attributes": {
               "name": "coreutils_darwin_arm64",
               "generator_name": "coreutils_darwin_arm64",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "version": "0.0.23",
               "platform": "darwin_arm64"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:coreutils_toolchain.bzl%coreutils_platform_repo",
                    "attributes": {
                         "name": "coreutils_darwin_arm64",
                         "generator_name": "coreutils_darwin_arm64",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "version": "0.0.23",
                         "platform": "darwin_arm64"
                    },
                    "output_tree_hash": "fecbedf135a0cd0e5d1bf30c7eb7875a61dbfaa8b47d75a5c66e1136ed0fe26a"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository com_github_golang_protobuf instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:200:12: in go_rules_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "com_github_golang_protobuf",
               "generator_name": "com_github_golang_protobuf",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://mirror.bazel.build/github.com/golang/protobuf/archive/refs/tags/v1.5.4.zip",
                    "https://github.com/golang/protobuf/archive/refs/tags/v1.5.4.zip"
               ],
               "sha256": "9efeb4561ed4fbb9cefe97da407bb7b6247d4ed3dee4bfc2c24fc03dd4b5596d",
               "strip_prefix": "protobuf-1.5.4",
               "patches": [
                    "@@rules_go//third_party:com_github_golang_protobuf-gazelle.patch"
               ],
               "patch_args": [
                    "-p1"
               ]
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/golang/protobuf/archive/refs/tags/v1.5.4.zip",
                              "https://github.com/golang/protobuf/archive/refs/tags/v1.5.4.zip"
                         ],
                         "sha256": "9efeb4561ed4fbb9cefe97da407bb7b6247d4ed3dee4bfc2c24fc03dd4b5596d",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "protobuf-1.5.4",
                         "add_prefix": "",
                         "type": "",
                         "patches": [
                              "@@rules_go//third_party:com_github_golang_protobuf-gazelle.patch"
                         ],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p1"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "com_github_golang_protobuf"
                    },
                    "output_tree_hash": "330bd5f7d6f6094c6cf83a0466818adca3ae8c5547bc410d2022b0e26844ca02"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:jq_toolchain.bzl%jq_platform_repo",
          "definition_information": "Repository jq_darwin_arm64 instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:111:27: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:42:25: in register_jq_toolchains\nRepository rule jq_platform_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/jq_toolchain.bzl:193:35: in <toplevel>\n",
          "original_attributes": {
               "name": "jq_darwin_arm64",
               "generator_name": "jq_darwin_arm64",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "version": "1.7",
               "platform": "darwin_arm64"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:jq_toolchain.bzl%jq_platform_repo",
                    "attributes": {
                         "name": "jq_darwin_arm64",
                         "generator_name": "jq_darwin_arm64",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "version": "1.7",
                         "platform": "darwin_arm64"
                    },
                    "output_tree_hash": "6d5145084762d56df85861752675a18a8fd27ff2578f96643fa5cde9aef4cc2a"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:zstd_toolchain.bzl%zstd_binary_repo",
          "definition_information": "Repository zstd_darwin_arm64 instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:115:29: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:119:25: in register_zstd_toolchains\nRepository rule zstd_binary_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/zstd_toolchain.bzl:69:35: in <toplevel>\n",
          "original_attributes": {
               "name": "zstd_darwin_arm64",
               "generator_name": "zstd_darwin_arm64",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "platform": "darwin_arm64"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:zstd_toolchain.bzl%zstd_binary_repo",
                    "attributes": {
                         "name": "zstd_darwin_arm64",
                         "generator_name": "zstd_darwin_arm64",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "platform": "darwin_arm64"
                    },
                    "output_tree_hash": "f803c1abdf03c1c197a1caf14f0d94d051b63ef27543095e474f494d9d9a5559"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_oci//oci:repositories.bzl%regctl_repositories",
          "definition_information": "Repository oci_regctl_darwin_arm64 instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:117:31: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:143:28: in register_regctl_toolchains\nRepository rule regctl_repositories defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:88:38: in <toplevel>\n",
          "original_attributes": {
               "name": "oci_regctl_darwin_arm64",
               "generator_name": "oci_regctl_darwin_arm64",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "platform": "darwin_arm64"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_oci//oci:repositories.bzl%regctl_repositories",
                    "attributes": {
                         "name": "oci_regctl_darwin_arm64",
                         "generator_name": "oci_regctl_darwin_arm64",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "platform": "darwin_arm64"
                    },
                    "output_tree_hash": "5712203c6bc30cedea66a6ae4be6e5d129970d084578156ff689e488411160c5"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository org_golang_google_protobuf instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:160:12: in go_rules_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "org_golang_google_protobuf",
               "generator_name": "org_golang_google_protobuf",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://mirror.bazel.build/github.com/protocolbuffers/protobuf-go/archive/refs/tags/v1.33.0.zip",
                    "https://github.com/protocolbuffers/protobuf-go/archive/refs/tags/v1.33.0.zip"
               ],
               "sha256": "39a8bbfadaa3e71f9d7741d67ee60d69db40422dc531708a777259e594d923e3",
               "strip_prefix": "protobuf-go-1.33.0",
               "patches": [
                    "@@rules_go//third_party:org_golang_google_protobuf-gazelle.patch"
               ],
               "patch_args": [
                    "-p1"
               ]
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/protocolbuffers/protobuf-go/archive/refs/tags/v1.33.0.zip",
                              "https://github.com/protocolbuffers/protobuf-go/archive/refs/tags/v1.33.0.zip"
                         ],
                         "sha256": "39a8bbfadaa3e71f9d7741d67ee60d69db40422dc531708a777259e594d923e3",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "protobuf-go-1.33.0",
                         "add_prefix": "",
                         "type": "",
                         "patches": [
                              "@@rules_go//third_party:org_golang_google_protobuf-gazelle.patch"
                         ],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p1"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "org_golang_google_protobuf"
                    },
                    "output_tree_hash": "812fe919a0a8e38b916aee09866b8e33d9f116b84aac74d8c9eb623d9cf39590"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository rules_license instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:37:23: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_pkg/pkg/deps.bzl:49:17: in rules_pkg_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_pkg/pkg/deps.bzl:21:10: in http_archive\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "rules_license",
               "generator_name": "rules_license",
               "generator_function": "rules_pkg_dependencies",
               "generator_location": None,
               "urls": [
                    "https://mirror.bazel.build/github.com/bazelbuild/rules_license/releases/download/0.0.7/rules_license-0.0.7.tar.gz",
                    "https://github.com/bazelbuild/rules_license/releases/download/0.0.7/rules_license-0.0.7.tar.gz"
               ],
               "sha256": "4531deccb913639c30e5c7512a054d5d875698daeb75d8cf90f284375fe7c360"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/bazelbuild/rules_license/releases/download/0.0.7/rules_license-0.0.7.tar.gz",
                              "https://github.com/bazelbuild/rules_license/releases/download/0.0.7/rules_license-0.0.7.tar.gz"
                         ],
                         "sha256": "4531deccb913639c30e5c7512a054d5d875698daeb75d8cf90f284375fe7c360",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "rules_license"
                    },
                    "output_tree_hash": "387b52ddb659ca14fa5c4c415efd5125747238863bac4d972eeb411da9ec5582"
               }
          ]
     },
     {
          "original_rule_class": "//build/rules/vault:def.bzl%vault_binary",
          "definition_information": "Repository vault instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:72:24: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:49:17: in repository_dependencies\nRepository rule vault_binary defined at:\n  /Users/dexter/dev/src/github.com/f110/mono/build/rules/vault/def.bzl:154:31: in <toplevel>\n",
          "original_attributes": {
               "name": "vault",
               "generator_name": "vault",
               "generator_function": "repository_dependencies",
               "generator_location": None,
               "version": "1.11.4"
          },
          "repositories": [
               {
                    "rule_class": "//build/rules/vault:def.bzl%vault_binary",
                    "attributes": {
                         "name": "vault",
                         "generator_name": "vault",
                         "generator_function": "repository_dependencies",
                         "generator_location": None,
                         "version": "1.11.4"
                    },
                    "output_tree_hash": "f1e1430672f2df265a4e9bfed62a863d7d4c04cefa7b6bdf0a4edc7813bc31f0"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf",
          "definition_information": "Repository local_config_cc instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:181:13: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/cpp/cc_configure.bzl:149:16: in cc_configure\nRepository rule cc_autoconf defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/cpp/cc_configure.bzl:109:30: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_cc",
               "generator_name": "local_config_cc",
               "generator_function": "cc_configure",
               "generator_location": None
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/cpp:cc_configure.bzl%cc_autoconf",
                    "attributes": {
                         "name": "local_config_cc",
                         "generator_name": "local_config_cc",
                         "generator_function": "cc_configure",
                         "generator_location": None
                    },
                    "output_tree_hash": "5ea4a056fbe5adb2089a72db04e840c1f0b74779b76841994265e0914458cdb7"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/osx:xcode_configure.bzl%xcode_autoconf",
          "definition_information": "Repository local_config_xcode instantiated at:\n  /DEFAULT.WORKSPACE.SUFFIX:184:16: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/osx/xcode_configure.bzl:312:19: in xcode_configure\nRepository rule xcode_autoconf defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/osx/xcode_configure.bzl:297:33: in <toplevel>\n",
          "original_attributes": {
               "name": "local_config_xcode",
               "generator_name": "local_config_xcode",
               "generator_function": "xcode_configure",
               "generator_location": None,
               "xcode_locator": "@bazel_tools//tools/osx:xcode_locator.m"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/osx:xcode_configure.bzl%xcode_autoconf",
                    "attributes": {
                         "name": "local_config_xcode",
                         "generator_name": "local_config_xcode",
                         "generator_function": "xcode_configure",
                         "generator_location": None,
                         "xcode_locator": "@bazel_tools//tools/osx:xcode_locator.m"
                    },
                    "output_tree_hash": "3ebe2ff4e790ba515ffd37a73307f1069bed10d77b28ec6d87d5f91c52dbab3b"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository zlib instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:29:14: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/com_google_protobuf/protobuf_deps.bzl:48:21: in protobuf_deps\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "zlib",
               "generator_name": "zlib",
               "generator_function": "protobuf_deps",
               "generator_location": None,
               "urls": [
                    "https://github.com/madler/zlib/archive/v1.2.11.tar.gz"
               ],
               "sha256": "629380c90a77b964d896ed37163f5c3a34f6e6d897311f1df2a7016355c45eff",
               "strip_prefix": "zlib-1.2.11",
               "build_file": "@@com_google_protobuf//:third_party/zlib.BUILD"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://github.com/madler/zlib/archive/v1.2.11.tar.gz"
                         ],
                         "sha256": "629380c90a77b964d896ed37163f5c3a34f6e6d897311f1df2a7016355c45eff",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "zlib-1.2.11",
                         "add_prefix": "",
                         "type": "",
                         "patches": [],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p0"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file": "@@com_google_protobuf//:third_party/zlib.BUILD",
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "zlib"
                    },
                    "output_tree_hash": "1ddc0eb78288f2df46d473fefeb86920180c37ed8ce9ca3da0286ad6068fa253"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_go//go/private:sdk.bzl%go_download_sdk_rule",
          "definition_information": "Repository go_sdk instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:25:23: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/sdk.bzl:725:28: in go_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/sdk.bzl:318:25: in go_download_sdk\nRepository rule go_download_sdk_rule defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/sdk.bzl:136:39: in <toplevel>\n",
          "original_attributes": {
               "name": "go_sdk",
               "generator_name": "go_sdk",
               "generator_function": "go_register_toolchains",
               "generator_location": None,
               "version": "1.24.5"
          },
          "repositories": [
               {
                    "rule_class": "@@rules_go//go/private:sdk.bzl%go_download_sdk_rule",
                    "attributes": {
                         "name": "go_sdk",
                         "generator_name": "go_sdk",
                         "generator_function": "go_register_toolchains",
                         "generator_location": None,
                         "version": "1.24.5"
                    },
                    "output_tree_hash": "9a4ed598cd491020b343f9c9ad65877fc3e8f24b528131b8be556bd885b9b1b2"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository_cache.bzl%go_repository_cache",
          "definition_information": "Repository bazel_gazelle_go_repository_cache instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:33:21: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:76:28: in gazelle_dependencies\nRepository rule go_repository_cache defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/internal/go_repository_cache.bzl:77:38: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle_go_repository_cache",
               "generator_name": "bazel_gazelle_go_repository_cache",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "go_sdk_info": {
                    "go_sdk": "host"
               },
               "go_env": {}
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository_cache.bzl%go_repository_cache",
                    "attributes": {
                         "name": "bazel_gazelle_go_repository_cache",
                         "generator_name": "bazel_gazelle_go_repository_cache",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "go_sdk_info": {
                              "go_sdk": "host"
                         },
                         "go_env": {}
                    },
                    "output_tree_hash": "4b97b724d2ef1773ce97f6e26b01b60270a7fa1ca8232919bbe665f6e57dee2c"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository_tools.bzl%go_repository_tools",
          "definition_information": "Repository bazel_gazelle_go_repository_tools instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:33:21: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:82:24: in gazelle_dependencies\nRepository rule go_repository_tools defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/internal/go_repository_tools.bzl:117:38: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle_go_repository_tools",
               "generator_name": "bazel_gazelle_go_repository_tools",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "go_cache": "@@bazel_gazelle_go_repository_cache//:go.env"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository_tools.bzl%go_repository_tools",
                    "attributes": {
                         "name": "bazel_gazelle_go_repository_tools",
                         "generator_name": "bazel_gazelle_go_repository_tools",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "go_cache": "@@bazel_gazelle_go_repository_cache//:go.env"
                    },
                    "output_tree_hash": "4b0361d658a11acecf8bc08aabccde1e68eb40111ccb68017cbb57f7db4b2a55"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository_config.bzl%go_repository_config",
          "definition_information": "Repository bazel_gazelle_go_repository_config instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:33:21: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:87:25: in gazelle_dependencies\nRepository rule go_repository_config defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/internal/go_repository_config.bzl:66:39: in <toplevel>\n",
          "original_attributes": {
               "name": "bazel_gazelle_go_repository_config",
               "generator_name": "bazel_gazelle_go_repository_config",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "config": "//:WORKSPACE"
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository_config.bzl%go_repository_config",
                    "attributes": {
                         "name": "bazel_gazelle_go_repository_config",
                         "generator_name": "bazel_gazelle_go_repository_config",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "config": "//:WORKSPACE"
                    },
                    "output_tree_hash": "183a8ea0d278fdc906b28f2593b0b48a09e6baf47d6689ab33d9166d8d5d0dac"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository org_golang_x_net instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:33:21: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:313:11: in gazelle_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:372:18: in _maybe\nRepository rule go_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/internal/go_repository.bzl:363:32: in <toplevel>\n",
          "original_attributes": {
               "name": "org_golang_x_net",
               "generator_name": "org_golang_x_net",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "importpath": "golang.org/x/net",
               "version": "v0.18.0",
               "sum": "h1:mIYleuAkSbHh0tCv7RvjL3F6ZVbLjq4+R7zbOn3Kokg="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "org_golang_x_net",
                         "generator_name": "org_golang_x_net",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "importpath": "golang.org/x/net",
                         "version": "v0.18.0",
                         "sum": "h1:mIYleuAkSbHh0tCv7RvjL3F6ZVbLjq4+R7zbOn3Kokg="
                    },
                    "output_tree_hash": "f2af833ffae4dcd6b8f79506567b4a0d1a176dea0ef6daa04a41d550831a1e55"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository org_golang_google_grpc instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:33:21: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:264:11: in gazelle_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:372:18: in _maybe\nRepository rule go_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/internal/go_repository.bzl:363:32: in <toplevel>\n",
          "original_attributes": {
               "name": "org_golang_google_grpc",
               "generator_name": "org_golang_google_grpc",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "importpath": "google.golang.org/grpc",
               "version": "v1.40.1",
               "sum": "h1:pnP7OclFFFgFi4VHQDQDaoXUVauOFyktqTsqqgzFKbc="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "org_golang_google_grpc",
                         "generator_name": "org_golang_google_grpc",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "importpath": "google.golang.org/grpc",
                         "version": "v1.40.1",
                         "sum": "h1:pnP7OclFFFgFi4VHQDQDaoXUVauOFyktqTsqqgzFKbc="
                    },
                    "output_tree_hash": "9680010be68468df416ce0613f83e7eabf8f1e31855f8ce9423978892fa70c06"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
          "definition_information": "Repository org_golang_google_genproto instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:244:12: in go_rules_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe\nRepository rule http_archive defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>\n",
          "original_attributes": {
               "name": "org_golang_google_genproto",
               "generator_name": "org_golang_google_genproto",
               "generator_function": "go_rules_dependencies",
               "generator_location": None,
               "urls": [
                    "https://mirror.bazel.build/github.com/googleapis/go-genproto/archive/dc85e6b867a5ebdfeaa293ddb423f00255ec921e.zip",
                    "https://github.com/googleapis/go-genproto/archive/dc85e6b867a5ebdfeaa293ddb423f00255ec921e.zip"
               ],
               "sha256": "ef3c82a1e6951a7931107d00ad4fe034366903290feae82bb1a19211c86d9d2f",
               "strip_prefix": "go-genproto-dc85e6b867a5ebdfeaa293ddb423f00255ec921e",
               "patches": [
                    "@@rules_go//third_party:org_golang_google_genproto-gazelle.patch"
               ],
               "patch_args": [
                    "-p1"
               ]
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_tools//tools/build_defs/repo:http.bzl%http_archive",
                    "attributes": {
                         "url": "",
                         "urls": [
                              "https://mirror.bazel.build/github.com/googleapis/go-genproto/archive/dc85e6b867a5ebdfeaa293ddb423f00255ec921e.zip",
                              "https://github.com/googleapis/go-genproto/archive/dc85e6b867a5ebdfeaa293ddb423f00255ec921e.zip"
                         ],
                         "sha256": "ef3c82a1e6951a7931107d00ad4fe034366903290feae82bb1a19211c86d9d2f",
                         "integrity": "",
                         "netrc": "",
                         "auth_patterns": {},
                         "canonical_id": "",
                         "strip_prefix": "go-genproto-dc85e6b867a5ebdfeaa293ddb423f00255ec921e",
                         "add_prefix": "",
                         "type": "",
                         "patches": [
                              "@@rules_go//third_party:org_golang_google_genproto-gazelle.patch"
                         ],
                         "remote_file_urls": {},
                         "remote_file_integrity": {},
                         "remote_patches": {},
                         "remote_patch_strip": 0,
                         "patch_tool": "",
                         "patch_args": [
                              "-p1"
                         ],
                         "patch_cmds": [],
                         "patch_cmds_win": [],
                         "build_file_content": "",
                         "workspace_file_content": "",
                         "name": "org_golang_google_genproto"
                    },
                    "output_tree_hash": "aeb8f44c4690c564aadc19661cb373508eb77f7a9624b46861c9d4e5d01837df"
               }
          ]
     },
     {
          "original_rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
          "definition_information": "Repository org_golang_x_text instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:33:21: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:341:11: in gazelle_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/deps.bzl:372:18: in _maybe\nRepository rule go_repository defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_gazelle/internal/go_repository.bzl:363:32: in <toplevel>\n",
          "original_attributes": {
               "name": "org_golang_x_text",
               "generator_name": "org_golang_x_text",
               "generator_function": "gazelle_dependencies",
               "generator_location": None,
               "importpath": "golang.org/x/text",
               "version": "v0.14.0",
               "sum": "h1:ScX5w1eTa3QqT8oi6+ziP7dTV1S2+ALU0bI+0zXKWiQ="
          },
          "repositories": [
               {
                    "rule_class": "@@bazel_gazelle//internal:go_repository.bzl%go_repository",
                    "attributes": {
                         "name": "org_golang_x_text",
                         "generator_name": "org_golang_x_text",
                         "generator_function": "gazelle_dependencies",
                         "generator_location": None,
                         "importpath": "golang.org/x/text",
                         "version": "v0.14.0",
                         "sum": "h1:ScX5w1eTa3QqT8oi6+ziP7dTV1S2+ALU0bI+0zXKWiQ="
                    },
                    "output_tree_hash": "44f3e47f7057a6c845ae532d6b2e6d627b80f9793da70dad03c5f50100ffa6cf"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_oci//oci/private:pull.bzl%oci_alias",
          "definition_information": "Repository com_google_distroless_base instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:74:23: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:77:17: in container_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/pull.bzl:251:14: in oci_pull\nRepository rule oci_alias defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/private/pull.bzl:417:28: in <toplevel>\n",
          "original_attributes": {
               "name": "com_google_distroless_base",
               "generator_name": "com_google_distroless_base",
               "generator_function": "container_dependencies",
               "generator_location": None,
               "scheme": "https",
               "registry": "gcr.io",
               "repository": "distroless/base",
               "identifier": "sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
               "platform": "//external:com_google_distroless_base_single",
               "target_name": "com_google_distroless_base",
               "reproducible": True
          },
          "repositories": [
               {
                    "rule_class": "@@rules_oci//oci/private:pull.bzl%oci_alias",
                    "attributes": {
                         "name": "com_google_distroless_base",
                         "generator_name": "com_google_distroless_base",
                         "generator_function": "container_dependencies",
                         "generator_location": None,
                         "scheme": "https",
                         "registry": "gcr.io",
                         "repository": "distroless/base",
                         "identifier": "sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
                         "platform": "//external:com_google_distroless_base_single",
                         "target_name": "com_google_distroless_base",
                         "reproducible": True
                    },
                    "output_tree_hash": "f6eedce8d8a9ae8ddc291398c83ca324b53c7e4a19f14876ade55b563bb06766"
               }
          ]
     },
     {
          "original_rule_class": "@@rules_oci//oci/private:pull.bzl%oci_pull",
          "definition_information": "Repository com_google_distroless_base_single instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:74:23: in <toplevel>\n  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:77:17: in container_dependencies\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/pull.bzl:240:18: in oci_pull\nRepository rule oci_pull defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/private/pull.bzl:288:27: in <toplevel>\n",
          "original_attributes": {
               "name": "com_google_distroless_base_single",
               "generator_name": "com_google_distroless_base_single",
               "generator_function": "container_dependencies",
               "generator_location": None,
               "scheme": "https",
               "registry": "gcr.io",
               "repository": "distroless/base",
               "identifier": "sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
               "target_name": "com_google_distroless_base_single",
               "bazel_tags": []
          },
          "repositories": [
               {
                    "rule_class": "@@rules_oci//oci/private:pull.bzl%oci_pull",
                    "attributes": {
                         "name": "com_google_distroless_base_single",
                         "generator_name": "com_google_distroless_base_single",
                         "generator_function": "container_dependencies",
                         "generator_location": None,
                         "scheme": "https",
                         "registry": "gcr.io",
                         "repository": "distroless/base",
                         "identifier": "sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
                         "target_name": "com_google_distroless_base_single",
                         "bazel_tags": []
                    },
                    "output_tree_hash": "4868e396b1a9710ae255bcf1536c1aa5521073351f394e7f34d5a85f59adaa95"
               }
          ]
     },
     {
          "original_rule_class": "@@aspect_bazel_lib//lib/private:copy_to_directory_toolchain.bzl%copy_to_directory_platform_repo",
          "definition_information": "Repository copy_to_directory_darwin_arm64 instantiated at:\n  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:45:24: in <toplevel>\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/repositories.bzl:114:42: in oci_register_toolchains\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/repositories.bzl:290:40: in register_copy_to_directory_toolchains\nRepository rule copy_to_directory_platform_repo defined at:\n  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/aspect_bazel_lib/lib/private/copy_to_directory_toolchain.bzl:182:50: in <toplevel>\n",
          "original_attributes": {
               "name": "copy_to_directory_darwin_arm64",
               "generator_name": "copy_to_directory_darwin_arm64",
               "generator_function": "oci_register_toolchains",
               "generator_location": None,
               "platform": "darwin_arm64"
          },
          "repositories": [
               {
                    "rule_class": "@@aspect_bazel_lib//lib/private:copy_to_directory_toolchain.bzl%copy_to_directory_platform_repo",
                    "attributes": {
                         "name": "copy_to_directory_darwin_arm64",
                         "generator_name": "copy_to_directory_darwin_arm64",
                         "generator_function": "oci_register_toolchains",
                         "generator_location": None,
                         "platform": "darwin_arm64"
                    },
                    "output_tree_hash": "f13ccc9815089e88f682adfe463fa143bda758f6de045f4af6f118c57e02ef3a"
               }
          ]
     }
]
