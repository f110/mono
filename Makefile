update-deps:
	bazel run //:gazelle -- update-repos -from_file=go.mod -to_macro=repositories.bzl%go_repositories

update:
	bazel run //:gazelle -- update

.PHONY: update-deps update