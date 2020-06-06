gen:
	bazel query 'attr(generator_function, k8s_code_generator, //...)' | xargs -n1 bazel run
	bazel run //:vendor

run:
	bazel run //controllers/minio-extra-operator/cmd/minio-extra-operator -- -lease-lock-name minio-eo -lease-lock-namespace default -dev -v=4

push:
	bazel run //controllers/minio-extra-operator:push
	@cat ../../bazel-bin/controllers/minio-extra-operator/push.digest
	@echo

build-manifest:
	kustomize build config | bazel run //tools/manifest-cleaner > config/all-in-one.yaml

.PHONY: run gen push build-manifest