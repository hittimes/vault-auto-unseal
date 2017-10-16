SHELL := /bin/bash

BUILD_DIR                 := build
DOCKER_CONTAINER_NAME_TAG := $(shell scripts/print-docker-container-name-tag)

all:
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) vault-auto-unseal

.PHONY: all


lint:
	export CGO_ENABLED=0 ; \
	concurrency_flag="$${CIRCLECI:+--concurrency=1}" ; \
	nice gometalinter --vendor --vendored-linters --aggregate \
		--deadline=60s $${concurrency_flag} \
		--disable-all \
		--enable=gofmt \
		--enable=vet \
		--enable=vetshadow \
		--enable=varcheck \
		--enable=structcheck \
		--enable=errcheck \
		--enable=unconvert \
		./...

.PHONY: lint


test:
	go test ./...

.PHONY: test


vault-auto-unseal: \
	vault-auto-unseal-darwin-amd64 \
	vault-auto-unseal-linux-amd64

.PHONY: vault-auto-unseal


vault-auto-unseal-darwin-amd64:
	$(call compile-bin,darwin,amd64,.,vault-auto-unseal)

.PHONY: vault-auto-unseal-darwin-amd64


vault-auto-unseal-linux-amd64:
	$(call compile-bin,linux,amd64,.,vault-auto-unseal)

.PHONY: vault-auto-unseal-linux-amd64


container:
	[[ -n "$(DOCKER_CONTAINER_NAME_TAG)" ]]
	docker build --tag="$(DOCKER_CONTAINER_NAME_TAG)" .

.PHONY: container


container-push:
	[[ -n "$(DOCKER_CONTAINER_NAME_TAG)" ]]
	scripts/push-container-image-docker-hub "$(DOCKER_CONTAINER_NAME_TAG)"

.PHONY: container-push


# Args:
#   1: GOOS
#   2: GOARCH
#   3: Relative filesystem path to Go 'main' package
#   4: Name to give the resulting binary
#
# Envs:
#   GO_LDFLAGS: Arguments to pass through to 'go tool link'.
#   GO_PKG_CACHE: Set to a non-empty string to force the use of the Go package
#   cache.  By default, the Go package cache will be bypassed when cross
#   compiling.
#
define compile-bin
	@( \
		is_native() { \
			( \
				eval $$(go env) && \
				if [[ "$${GOOS}" = "$(1)" && "$${GOARCH}" = "$(2)" ]] ; then \
					return 0 ; \
				fi ; \
				return 1 \
			) ; \
		} ; \
		\
		BIN_DIR='$(BUILD_DIR)/$(1)_$(2)' && \
		\
		GO_BUILD_FLAGS=() ; \
		if [[ -n "$${GO_PKG_CACHE}" ]] ; then \
			GO_BUILD_FLAGS+=('-i') ; \
		elif is_native ; then \
			GO_BUILD_FLAGS+=('-i') ; \
		fi ; \
		\
		GO_BUILD_FLAGS+=('-ldflags' "$(GO_LDFLAGS)") ; \
		GO_BUILD_FLAGS+=('-o' "$${BIN_DIR}/$(4)") ; \
		\
		mkdir -p "$${BIN_DIR}" && \
		CGO_ENABLED=0 \
		GOOS='$(1)' \
		GOARCH='$(2)' \
		go build "$${GO_BUILD_FLAGS[@]}" './$(3)' \
	)
endef
