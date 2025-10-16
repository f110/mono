## Migration of `io_bazel_rules_go`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository io_bazel_rules_go instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "io_bazel_rules_go",
  urls = [
    "https://github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip",
    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip"
  ],
  sha256 = "d93ef02f1e72c82d8bb3d5169519b36167b33cf68c252525e3b9d3d5dd143de7",
  strip_prefix = "",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found partially name matches in BCR: `rules_go`

It has been introduced as a Bazel module:

	bazel_dep(name = "rules_go", version = "0.57.0", repo_name = "io_bazel_rules_go")
## Migration of `dev_f110_protoc_ddl`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository dev_f110_protoc_ddl instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:9:15: in <toplevel>
Repository rule git_repository defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/git.bzl:189:33: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")
git_repository(
  name = "dev_f110_protoc_ddl",
  remote = "https://github.com/f110/protoc-ddl",
  commit = "1cb0fefe60f4aeecc458a2f48abbc4a4e59f637f",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced with `use_repo_rule`:

## Migration of `bazel_skylib`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository bazel_skylib instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:23:22: in <toplevel>
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:51:12: in go_rules_dependencies
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/io_bazel_rules_go/go/private/repositories.bzl:305:18: in _maybe
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "bazel_skylib",
  urls = [
    "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz",
    "https://github.com/bazelbuild/bazel-skylib/releases/download/1.6.1/bazel-skylib-1.6.1.tar.gz"
  ],
  sha256 = "9f38886a40548c6e96c106b752f242130ee11aaa068a56ba7e56f4511f33e4f2",
  strip_prefix = "",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `bazel_skylib`

Found partially name matches in BCR: `bazel_skylib_gazelle_plugin`

It has been introduced as a Bazel module:

	bazel_dep(name = "bazel_skylib", version = "1.8.1")
## Migration of `rules_proto`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository rules_proto instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "rules_proto",
  urls = [
    "https://github.com/bazelbuild/rules_proto/releases/download/6.0.0/rules_proto-6.0.0.tar.gz",
    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_proto/releases/download/6.0.0/rules_proto-6.0.0.tar.gz"
  ],
  sha256 = "303e86e722a520f6f326a50b41cfc16b98fe6d1955ce46642a5b7a67c11c0f5d",
  strip_prefix = "rules_proto-6.0.0",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `rules_proto`

It has been introduced as a Bazel module:

	bazel_dep(name = "rules_proto", version = "7.1.0")
## Migration of `dev_f110_kubeproto`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository dev_f110_kubeproto instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:15:15: in <toplevel>
Repository rule git_repository defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/git.bzl:189:33: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")
git_repository(
  name = "dev_f110_kubeproto",
  remote = "https://github.com/f110/kubeproto",
  commit = "90d00e364ad040d388c54b32c9ac3d85604bc6ec",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced with `use_repo_rule`:

## Migration of `kind`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository kind instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:72:24: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:34:16: in repository_dependencies
Repository rule kind_binary defined at:
  /Users/dexter/dev/src/github.com/f110/mono/build/rules/kind/def.bzl:9:30: in <toplevel>

```

#### Definition
```python
load("//build/rules/kind:def.bzl", "kind_binary")
kind_binary(
  name = "kind",
  version = "0.22.0",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced with `use_repo_rule`:

## Migration of `com_google_protobuf`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository com_google_protobuf instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "com_google_protobuf",
  urls = [
    "https://github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz",
    "https://mirror.bucket.x.f110.dev/github.com/protocolbuffers/protobuf/archive/refs/tags/v3.21.1.tar.gz"
  ],
  sha256 = "a295dd3b9551d3e2749a9969583dea110c6cdcc39d02088f7c7bb1100077e081",
  strip_prefix = "protobuf-3.21.1",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found partially name matches in BCR: `protobuf`

It has been introduced as a Bazel module:

	bazel_dep(name = "protobuf", version = "32.1", repo_name = "com_google_protobuf")
## Migration of `bazel_gazelle`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository bazel_gazelle instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "bazel_gazelle",
  urls = [
    "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz",
    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/bazel-gazelle/releases/download/v0.38.0/bazel-gazelle-v0.38.0.tar.gz"
  ],
  sha256 = "8ad77552825b078a10ad960bec6ef77d2ff8ec70faef2fd038db713f410f5d87",
  strip_prefix = "",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found partially name matches in BCR: `gazelle`

It has been introduced as a Bazel module:

	bazel_dep(name = "gazelle", version = "0.45.0", repo_name = "bazel_gazelle")
## Migration of `rules_pkg`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository rules_pkg instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "rules_pkg",
  urls = [
    "https://github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz",
    "https://mirror.bucket.x.f110.dev/github.com/bazelbuild/rules_pkg/releases/download/1.0.1/rules_pkg-1.0.1.tar.gz"
  ],
  sha256 = "d20c951960ed77cb7b341c2a59488534e494d5ad1d30c4818c736d57772a9fef",
  strip_prefix = "",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `rules_pkg`

It has been introduced as a Bazel module:

	bazel_dep(name = "rules_pkg", version = "1.1.0")
## Migration of `rules_oci`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository rules_oci instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:7:19: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/rules_dependencies.bzl:17:21: in rules_dependencies
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "rules_oci",
  urls = [
    "https://github.com/bazel-contrib/rules_oci/releases/download/v2.2.1/rules_oci-v2.2.1.tar.gz",
    "https://mirror.bucket.x.f110.dev/github.com/bazel-contrib/rules_oci/releases/download/v2.2.1/rules_oci-v2.2.1.tar.gz"
  ],
  sha256 = "cfea16076ebbec1faea494882ab97d94b1a62d6bcd5aceabad8f95ea0d0a1361",
  strip_prefix = "rules_oci-2.2.1",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `rules_oci`

It has been introduced as a Bazel module:

	bazel_dep(name = "rules_oci", version = "2.2.6")
## Migration of `kustomize`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository kustomize instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:72:24: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:29:21: in repository_dependencies
Repository rule kustomize_binary defined at:
  /Users/dexter/dev/src/github.com/f110/mono/build/rules/kustomize/def.bzl:13:35: in <toplevel>

```

#### Definition
```python
load("//build/rules/kustomize:def.bzl", "kustomize_binary")
kustomize_binary(
  name = "kustomize",
  version = "v4.5.4",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced with `use_repo_rule`:

TODO: Please remove the usages of referring your own repo via `@mono//`, targets should be referenced directly with `//`. 
If it's used in a macro, you can use `Label("//foo/bar")` to make sure it always points to your repo no matter where the macro is used.
You can temporarily work around this by adding `repo_name` attribute to the `module` directive in your MODULE.bazel file.
Repository definition for `mono` is not found in ./resolved_deps.py file, please add `--force/-f` flag to force update it.
TODO: Please remove the usages of referring your own repo via `@mono//`, targets should be referenced directly with `//`. 
If it's used in a macro, you can use `Label("//foo/bar")` to make sure it always points to your repo no matter where the macro is used.
You can temporarily work around this by adding `repo_name` attribute to the `module` directive in your MODULE.bazel file.
Repository definition for `mono` is not found in ./resolved_deps.py file, please add `--force/-f` flag to force update it.
## Migration of `vault`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository vault instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:72:24: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:49:17: in repository_dependencies
Repository rule vault_binary defined at:
  /Users/dexter/dev/src/github.com/f110/mono/build/rules/vault/def.bzl:154:31: in <toplevel>

```

#### Definition
```python
load("//build/rules/vault:def.bzl", "vault_binary")
vault_binary(
  name = "vault",
  version = "1.11.4",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced with `use_repo_rule`:

## Migration of `com_google_distroless_base`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository com_google_distroless_base instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:74:23: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:77:17: in container_dependencies
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/pull.bzl:251:14: in oci_pull
Repository rule oci_alias defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/private/pull.bzl:417:28: in <toplevel>

```

#### Definition
```python
load("@@rules_oci//oci/private:pull.bzl", "oci_alias")
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
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced using a module extension:

## Migration of `com_google_distroless_base_single`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository com_google_distroless_base_single instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:74:23: in <toplevel>
  /Users/dexter/dev/src/github.com/f110/mono/dependencies.bzl:77:17: in container_dependencies
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/pull.bzl:240:18: in oci_pull
Repository rule oci_pull defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/private/pull.bzl:288:27: in <toplevel>

```

#### Definition
```python
load("@@rules_oci//oci/private:pull.bzl", "oci_pull")
oci_pull(
  name = "com_google_distroless_base_single",
  scheme = "https",
  registry = "gcr.io",
  repository = "distroless/base",
  identifier = "sha256:8267a5d9fa15a538227a8850e81cf6c548a78de73458e99a67e8799bbffb1ba0",
  target_name = "com_google_distroless_base_single",
  bazel_tags = [  ],
)
```
**Tip**: URLs usually show which version was used.
</details>

___
	It is not found in BCR. 

	It has been introduced using a module extension:

## Migration of `aspect_bazel_lib`:

<details>
<summary>Click here to see where and how the repo was declared in the WORKSPACE file</summary>

#### Location
```python
Repository aspect_bazel_lib instantiated at:
  /Users/dexter/dev/src/github.com/f110/mono/WORKSPACE:41:23: in <toplevel>
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/dependencies.bzl:23:17: in rules_oci_dependencies
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/rules_oci/oci/dependencies.bzl:11:10: in http_archive
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/utils.bzl:268:18: in maybe
Repository rule http_archive defined at:
  /private/var/tmp/_bazel_dexter/8c9d0d0f17cdc128991cb3936ea70f21/external/bazel_tools/tools/build_defs/repo/http.bzl:387:31: in <toplevel>

```

#### Definition
```python
load("@@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
http_archive(
  name = "aspect_bazel_lib",
  url = "https://github.com/aspect-build/bazel-lib/releases/download/v2.7.2/bazel-lib-v2.7.2.tar.gz",
  sha256 = "a8a92645e7298bbf538aa880131c6adb4cf6239bbd27230f077a00414d58e4ce",
  strip_prefix = "bazel-lib-2.7.2",
)
```
**Tip**: URLs usually show which version was used.
</details>

___
Found perfect name match in BCR: `aspect_bazel_lib`

It has been introduced as a Bazel module:

	bazel_dep(name = "aspect_bazel_lib", version = "2.21.1")
Repository definition for `debian12_libgdbm6` is not found in ./resolved_deps.py file, please add `--force/-f` flag to force update it.
