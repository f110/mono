run:
	bazel run //bot/continuous-delivery/cmd/maintenance-bot -- -c $(CURDIR)/config_debug.yaml

push:
	bazel query 'kind(container_push, //bot/continuous-delivery/...)' | xargs -n1 bazel run

.PHONY: update run push