PROJECT_NAME = harbor-project-operator
ARGS = -harbor-namespace harbor -harbor-service-name harbor-harbor-core -admin-secret-name harbor-harbor-core

gen:
	bazel run //controllers/$(PROJECT_NAME)/pkg/api:gen.deepcopy
	bazel run //controllers/$(PROJECT_NAME)/pkg/api:gen.client
	bazel run //controllers/$(PROJECT_NAME)/pkg/api:gen.lister
	bazel run //controllers/$(PROJECT_NAME)/pkg/api:gen.informer
	bazel run //controllers/$(PROJECT_NAME)/pkg/api:gen.crd
	bazel run //controllers/$(PROJECT_NAME)/pkg/controller:rbac
	bazel run //:vendor

run:
	bazel run //controllers/$(PROJECT_NAME)/cmd/$(PROJECT_NAME) -- -lease-lock-name harbor-project -lease-lock-namespace default $(ARGS) -dev -v=4

push:
	bazel run //controllers/$(PROJECT_NAME):push
	@cat ../../bazel-bin/controllers/$(PROJECT_NAME)/push.digest
	@echo

build-manifest:
	kustomize build config > config/all-in-one.yaml

.PHONY: run gen push build-manifest