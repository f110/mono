update-deps:
	bazel run //:vendor

gen:
	bazel query 'kind(vendor_ddl, //...)' | xargs -n1 bazel run
	bazel query 'kind(vendor_grpc_source, //...)' | xargs -n1 bazel run
	bazel query 'kind(vendor_proto_source, //...)' | xargs -n1 bazel run
	bazel query 'kind(vendor_kubeproto, //...)' | xargs -n1 bazel run

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
MINIO_ACCESS_KEY        = MsdgKFqgT7Bw
MINIO_SECRET_ACCESS_KEY = P2ThRFth7w1p6gDROcE3ya3gXoIEevuA

DSN = $(DATABASE_USER):$(DATABASE_PASSWORD)@tcp($(DATABASE_HOST):$(DATABASE_PORT))/$(DATABASE_NAME)
GITHUB = --github-app-id $(APP_ID) --github-installation-id $(INSTALLATION_ID) --github-private-key-file $(PRIVATEKEY_FILE)
MINIO = --minio-endpoint http://127.0.0.1:9000 --minio-port $(MINIO_PORT) --minio-bucket $(MINIO_BUCKET) --minio-access-key $(MINIO_ACCESS_KEY) --minio-secret-access-key $(MINIO_SECRET_ACCESS_KEY)

DASHBOARDFLAGS = --addr 127.0.0.1:8080 --dsn "$(DSN)" --log-level debug --api http://127.0.0.1:8081 --dev $(MINIO)
APIFLAGS = --addr 127.0.0.1:8081 --dsn "$(DSN)" --namespace default --lease-lock-name builder --lease-lock-namespace default --log-level debug --dev $(GITHUB) $(MINIO)

.PHONY: run-dashboard
run-dashboard:
	bazel run //go/cmd/build -- dashboard $(DASHBOARDFLAGS)

.PHONY: run-api
run-api:
	bazel run //go/cmd/build -- builder $(APIFLAGS)

.PHONY: run-migrate
run-migrate:
	bazel run @dev_f110_protoc_ddl//cmd/migrate -- --schema $(CURDIR)/go/build/database/schema.sql --driver mysql --dsn "$(DSN)" --execute
