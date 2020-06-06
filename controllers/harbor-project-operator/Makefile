PROJECT_NAME = harbor-project-operator
ARGS = -harbor-namespace harbor -harbor-service-name harbor-harbor-core -admin-secret-name harbor-harbor-core -core-configmap-name harbor-harbor-core

gen:
	bazel query 'attr(generator_function, k8s_code_generator, //controllers/harbor-project-operator/...)' | xargs -n1 bazel run
	bazel run //:vendor

run:
	bazel run //controllers/$(PROJECT_NAME)/cmd/$(PROJECT_NAME) -- -lease-lock-name harbor-project -lease-lock-namespace default $(ARGS) -dev -v=4

push:
	bazel run //controllers/$(PROJECT_NAME):push
	@cat ../../bazel-bin/controllers/$(PROJECT_NAME)/push.digest
	@echo

build-manifest:
	kustomize build config | bazel run //tools/manifest-cleaner > config/all-in-one.yaml

.PHONY: run gen push build-manifest