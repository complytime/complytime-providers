BINARY_DIR ?= bin
GIT_TAG_RAW := $(shell git tag --sort=-v:refname | head -1 2>/dev/null)
GIT_TAG ?= $(or $(GIT_TAG_RAW),v0.0.0)
VERSION ?= $(shell echo "$(GIT_TAG)" | sed 's/^v//')
GO_LD_FLAGS := -X github.com/complytime/complytime-providers/internal/version.version=$(VERSION)

.PHONY: build build-openscap-provider build-ampel-provider build-opa-provider test test-cross-repo test-devcontainer vendor lint man

build: build-openscap-provider build-ampel-provider build-opa-provider

build-openscap-provider:
	go build -ldflags="$(GO_LD_FLAGS)" -o $(BINARY_DIR)/complyctl-provider-openscap ./cmd/openscap-provider

build-ampel-provider:
	go build -ldflags="$(GO_LD_FLAGS)" -o $(BINARY_DIR)/complyctl-provider-ampel ./cmd/ampel-provider

build-opa-provider:
	go build -ldflags="$(GO_LD_FLAGS)" -o $(BINARY_DIR)/complyctl-provider-opa ./cmd/opa-provider

test:
	go test ./...

test-cross-repo: ## run cross-repo integration test (requires COMPLYCTL_DIR and GITHUB_TOKEN)
ifndef COMPLYCTL_DIR
	$(error COMPLYCTL_DIR is not set. Set it to the root of a built complyctl checkout)
endif
	timeout 120 $(COMPLYCTL_DIR)/tests/cross-repo/cross_repo_integration_test.sh

test-devcontainer: ## verify devcontainer Containerfile builds and scripts pass shellcheck
	podman build .devcontainer/
	shellcheck .devcontainer/scripts/post-create.sh

vendor:
	go mod vendor

lint:
	golangci-lint run ./...

man: ## generate man pages from docs/man/*.md (requires pandoc)
	@for md in docs/man/*.md; do \
		out="$${md%.md}.1"; \
		pandoc -s -t man "$$md" -o "$$out"; \
		echo "Generated $$out"; \
	done
