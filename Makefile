BAZEL ?= bazel
GO ?= $(BAZEL) run @io_bazel_rules_go//go --

update-deps:
	$(GO) mod tidy
	$(BAZEL) run //:vendor

gen:
	bazel query 'kind(vendor_ddl, //...)' | xargs -n1 bazel run
	bazel query 'kind(vendor_grpc_source, //...)' | xargs -n1 bazel run
	bazel query 'kind(vendor_proto_source, //...)' | xargs -n1 bazel run
	bazel query 'kind(vendor_kubeproto, //...)' | xargs -n1 bazel run

deb_packages.bzl: deb_packages.yaml
	bazel run //build/private/deb_manager -- -conf $(CURDIR)/deb_packages.yaml -macro $(CURDIR)/build/rules/deb/deb_pkg.bzl $(CURDIR)/deb_packages.bzl

.PHONY: update-deps gen

push-unifibackup:
	bazel run --platforms=@io_bazel_rules_go//go/toolchain:linux_arm64 //containers/unifibackup:push

DATABASE_HOST     = localhost
DATABASE_PORT     = 13306
DATABASE_USER     = build
DATABASE_NAME     = build
DATABASE_PASSWORD = build

# This credentials is for local cluster
MINIO_NAME              = object-storage
MINIO_NAMESPACE         = default
MINIO_PORT              = 9000
MINIO_BUCKET            = logs
MINIO_ACCESS_KEY        = minioadmin
MINIO_SECRET_ACCESS_KEY = minioadmin

APP_ID          = 51841
INSTALLATION_ID = 6365451
PRIVATEKEY_FILE = $(CURDIR)/github-privatekey.pem

DSN = $(DATABASE_USER):$(DATABASE_PASSWORD)@tcp($(DATABASE_HOST):$(DATABASE_PORT))/$(DATABASE_NAME)
GITHUB = --github-app-id $(APP_ID) --github-installation-id $(INSTALLATION_ID) --github-private-key-file $(PRIVATEKEY_FILE)
MINIO = --minio-endpoint http://127.0.0.1:9000 --minio-port $(MINIO_PORT) --minio-bucket $(MINIO_BUCKET) --minio-access-key $(MINIO_ACCESS_KEY) --minio-secret-access-key $(MINIO_SECRET_ACCESS_KEY)
BAZEL_MIRROR_MINIO = --bazel-mirror-endpoint http://127.0.0.1:9000 --bazel-mirror-bucket build --bazel-mirror-access-key $(MINIO_ACCESS_KEY) --bazel-mirror-secret-access-key $(MINIO_SECRET_ACCESS_KEY) --bazel-mirror-prefix releases.bazel.build/

DASHBOARDFLAGS = --addr 127.0.0.1:8080 --dsn "$(DSN)" --log-level debug --api http://127.0.0.1:8081 --internal-api http://127.0.0.1:8081 --dev $(MINIO)
APIFLAGS = --addr 127.0.0.1:8081 --dsn "$(DSN)" --namespace default --lease-lock-name builder --lease-lock-namespace default --log-level debug --dev $(GITHUB) $(MINIO) $(BAZEL_MIRROR_MINIO)

.PHONY: run-dashboard
run-dashboard:
	bazel run //go/cmd/build -- dashboard $(DASHBOARDFLAGS)

.PHONY: run-api
run-api:
	bazel run //go/cmd/build -- builder $(APIFLAGS)

.PHONY: run-migrate
run-migrate:
	bazel run @dev_f110_protoc_ddl//cmd/migrate -- --schema $(CURDIR)/go/build/database/schema.sql --driver mysql --dsn "$(DSN)" --execute
