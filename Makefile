update-deps:
	bazel run //:vendor

gen:
	bazel query 'attr(generator_function, k8s_code_generator, //...)' | xargs -n1 bazel run

.PHONY: update-deps gen