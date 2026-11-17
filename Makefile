UF_WORKFLOW_RETENTION_DAYS ?= 14
# Capture before any $(shell ...) can export/evaluate command-line variables.
UF_WORKFLOW_RETENTION_DAYS_INPUT := $(value UF_WORKFLOW_RETENTION_DAYS)
unexport UF_WORKFLOW_RETENTION_DAYS
export UF_WORKFLOW_RETENTION_DAYS_INPUT
BINARY_DIR ?= bin
GIT_TAG_RAW := $(shell git tag --sort=-v:refname | head -1 2>/dev/null)
GIT_TAG ?= $(or $(GIT_TAG_RAW),v0.0.0)
VERSION ?= $(shell echo "$(GIT_TAG)" | sed 's/^v//')
UF_FORK_FILES := .opencode/commands/uf.address-feedback.md .opencode/commands/uf.review-council.md .opencode/commands/uf.review-pr.md
SHA256_CHECK := $(shell if command -v sha256sum >/dev/null 2>&1; then echo "sha256sum -c"; else echo "shasum -a 256 -c"; fi)
SHA256_SUM := $(shell if command -v sha256sum >/dev/null 2>&1; then echo sha256sum; else echo "shasum -a 256"; fi)
GO_LD_FLAGS := -X github.com/complytime/complytime-providers/internal/version.version=$(VERSION)

.PHONY: build build-openscap-provider build-ampel-provider build-opa-provider test test-cross-repo test-devcontainer vendor lint man verify-uf-fork update-uf-fork-manifest clean-uf-workflows test-uf-review

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
	timeout 120 "$(COMPLYCTL_DIR)/tests/cross-repo/cross_repo_integration_test.sh"

test-devcontainer: ## verify devcontainer Containerfile builds and scripts pass shellcheck
	podman build .devcontainer/
	shellcheck .devcontainer/scripts/post-create.sh

vendor:
	go mod vendor

lint:
	golangci-lint run ./...

test-uf-review: ## test local fork isolation, preflight, and recovery (requires jq)
	bash .github/scripts/test/verify_uf_fork_test.sh
	bash .github/scripts/test/uf_review_preflight_test.sh
	bash .github/scripts/test/uf_review_recovery_test.sh

verify-uf-fork: ## verify fork digests and plugin/native instruction isolation
	@test -f .uf/fork-manifest.sha256 || { echo "error: .uf/fork-manifest.sha256 is missing; run make update-uf-fork-manifest" >&2; exit 1; }
	@$(SHA256_CHECK) .uf/fork-manifest.sha256
	@set -e; for file in $(UF_FORK_FILES); do \
		awk -f .github/scripts/verify-uf-paths.awk "$$file"; \
	done

update-uf-fork-manifest: ## rewrite the fork manifest after an intentional command change
	@printf '%s\n' \
		'# Repository-owned fork of the UF v0.21.0 review commands.' \
		'# Verified by `make verify-uf-fork`. Update this manifest in the same commit as' \
		'# any intentional command change. Do not run `uf init --force` while the fork is' \
		'# active; reconcile with upstream first (see AGENTS.md).' > .uf/fork-manifest.tmp
	@$(SHA256_SUM) $(UF_FORK_FILES) >> .uf/fork-manifest.tmp
	@mv .uf/fork-manifest.tmp .uf/fork-manifest.sha256
	@echo "Updated .uf/fork-manifest.sha256"

clean-uf-workflows: ## list prunable workflow records; PRUNE=1 deletes them
	@bash .github/scripts/clean-uf-workflows.sh

man: ## generate man pages from docs/man/*.md (requires pandoc)
	@for md in docs/man/*.md; do \
		out="$${md%.md}.1"; \
		pandoc -s -t man "$$md" -o "$$out"; \
		echo "Generated $$out"; \
	done
