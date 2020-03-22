update-deps:
	bazel run //:vendor

gen:
	bazel run //controllers/minio-extra-operator/pkg/api:gen.deepcopy
	bazel run //controllers/minio-extra-operator/pkg/api:gen.client
	bazel run //controllers/minio-extra-operator/pkg/api:gen.lister
	bazel run //controllers/minio-extra-operator/pkg/api:gen.informer
	bazel run //:vendor

run:
	bazel run //controllers/minio-extra-operator/cmd/minio-extra-operator -- -lease-lock-name minio-eo -lease-lock-namespace default -dev -v=4

.PHONY: update-deps