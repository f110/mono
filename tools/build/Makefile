DATABASE_HOST = localhost
DATABASE_USER = build
DATABASE_NAME = build

MINIO_NAME              = object-storage
MINIO_NAMESPACE         = default
MINIO_PORT              = 9000
MINIO_BUCKET            = logs
MINIO_ACCESS_KEY        = MsdgKFqgT7Bw
MINIO_SECRET_ACCESS_KEY = P2ThRFth7w1p6gDROcE3ya3gXoIEevuA

DSN = $(DATABASE_USER)@tcp($(DATABASE_HOST))/$(DATABASE_NAME)
GITHUB = --github-app-id $(APP_ID) --github-installation-id $(INSTALLATION_ID) --github-private-key-file $(PRIVATEKEY_FILE)
MINIO = --minio-name $(MINIO_NAME) --minio-namespace $(MINIO_NAMESPACE) --minio-port $(MINIO_PORT) --minio-bucket $(MINIO_BUCKET) --minio-access-key $(MINIO_ACCESS_KEY) --minio-secret-access-key $(MINIO_SECRET_ACCESS_KEY)

DASHBOARDFLAGS = --addr 127.0.0.1:8080 --dsn "$(DSN)" --log-level debug --api http://127.0.0.1:8081 --namespace default --dev $(MINIO)
APIFLAGS = --addr 127.0.0.1:8081 --dsn "$(DSN)" --namespace default --lease-lock-name builder --lease-lock-namespace default --log-level debug --dev $(GITHUB) $(MINIO)

.PHONY: migrate
migrate:
	bazel run @dev_f110_protoc_ddl//cmd/migrate -- --schema $(CURDIR)/pkg/database/schema.sql --driver mysql --dsn "$(DSN)" --execute

.PHONY: push
push:
	bazel query 'kind(container_push, //tools/build/...)' | xargs -n1 bazel run

.PHONY: run-dashboard
run-dashboard:
	bazel run //tools/build/cmd/build -- dashboard $(DASHBOARDFLAGS)

.PHONY: run-api
run-api:
	bazel run //tools/build/cmd/build -- builder $(APIFLAGS)

.PHONY: vendor
vendor:
	bazel query 'attr(generator_function, vendor_ddl, //...)' --output label | xargs -n1 bazel run