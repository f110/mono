gen:
	bazel run //controllers/minio-extra-operator/pkg/api:gen.deepcopy
	bazel run //controllers/minio-extra-operator/pkg/api:gen.client
	bazel run //controllers/minio-extra-operator/pkg/api:gen.lister
	bazel run //controllers/minio-extra-operator/pkg/api:gen.informer
	bazel run //controllers/minio-extra-operator/pkg/controller:rbac
	bazel run //:vendor

run:
	bazel run //controllers/minio-extra-operator/cmd/minio-extra-operator -- -lease-lock-name minio-eo -lease-lock-namespace default -dev -v=4

build-manifest:
	kustomize build config > config/all-in-one.yaml

.PHONY: run gen build-manifest