.PHONY: sample/schema.sql
sample/schema.sql: sample/schema.proto
	bazel run //sample:vendor_schema

.PHONY: sample/schema.entity.go
sample/schema.entity.go: sample/schema.proto
	bazel run //sample:vendor_entity

update-deps:
	bazel run //:vendor_proto_source
	bazel run //:gazelle -- update

.PHONY: update-deps