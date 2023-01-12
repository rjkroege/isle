.PHONY:
build:
	bash build.sh

# TODO(rjk): need to set these dynamically somehow.

.PHONY:
release: build
	tar -C release -czf os-unknown-arm64.tar.gz .
