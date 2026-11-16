BINARY_DIR ?= bin
GIT_TAG_RAW := $(shell git tag --sort=-v:refname | head -1 2>/dev/null)
GIT_TAG ?= $(or $(GIT_TAG_RAW),v0.0.0)
VERSION ?= $(shell echo "$(GIT_TAG)" | sed 's/^v//')
UF_WORKFLOW_RETENTION_DAYS ?= 14
UF_FORK_FILES := .opencode/commands/uf.address-feedback.md .opencode/commands/uf.review-council.md .opencode/commands/uf.review-pr.md
# Matches backticked UF plugin tool names; bare prose mentions are ignored.
UF_PLUGIN_TOOL_PATTERN := `(plan_review_dispatch|dispatch_review_runs|consolidate_dispatch|finalize_review_dispatch|capture_review_diff|fetch_pr_review_context|submit_review_findings|submit_lesson_proposal|normalize_model_name|format_unleash_exit|dispatch_agent_run|resolve_base_ref|get_progress|clear_progress|checkpoint)`
SHA256_CHECK := $(shell if command -v sha256sum >/dev/null 2>&1; then echo "sha256sum -c"; else echo "shasum -a 256 -c"; fi)
SHA256_SUM := $(shell if command -v sha256sum >/dev/null 2>&1; then echo sha256sum; else echo "shasum -a 256"; fi)
GO_LD_FLAGS := -X github.com/complytime/complytime-providers/internal/version.version=$(VERSION)

.PHONY: build build-openscap-provider build-ampel-provider build-opa-provider test test-cross-repo test-devcontainer vendor lint man verify-uf-fork update-uf-fork-manifest clean-uf-workflows

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

verify-uf-fork: ## verify fork digests and that no UF plugin tool is required
	@test -f .uf/fork-manifest.sha256 || { echo "error: .uf/fork-manifest.sha256 is missing; run make update-uf-fork-manifest" >&2; exit 1; }
	@$(SHA256_CHECK) .uf/fork-manifest.sha256
	@if grep -nE '$(UF_PLUGIN_TOOL_PATTERN)' $(UF_FORK_FILES); then \
		echo "error: fork-owned commands reference UF plugin tools" >&2; exit 1; \
	fi

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
	@set -e; \
	case "$(UF_WORKFLOW_RETENTION_DAYS)" in \
		''|*[!0-9]*) echo "error: UF_WORKFLOW_RETENTION_DAYS must be a positive integer" >&2; exit 2;; \
	esac; \
	if [ "$(UF_WORKFLOW_RETENTION_DAYS)" -lt 1 ] || [ "$(UF_WORKFLOW_RETENTION_DAYS)" -gt 3650 ]; then \
		echo "error: UF_WORKFLOW_RETENTION_DAYS must be between 1 and 3650" >&2; exit 2; \
	fi; \
	if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
		if [ -n "$$(git ls-files .uf/workflows)" ]; then \
			echo "error: .uf/workflows contains tracked files; refusing to prune" >&2; exit 1; \
		fi; \
	else \
		echo "error: cannot verify that .uf/workflows is untracked; refusing to prune" >&2; exit 1; \
	fi; \
	stale=""; \
	if [ ! -d .uf/workflows ]; then \
		echo "clean-uf-workflows: no .uf/workflows directory"; exit 0; \
	fi; \
	if ! dirs=$$(find .uf/workflows -mindepth 2 -maxdepth 2 -type d); then \
		echo "error: cannot enumerate .uf/workflows" >&2; exit 1; \
	fi; \
	for dir in $$dirs; do \
		if [ -z "$$(find "$$dir" -type f -print -quit)" ]; then continue; fi; \
		if ! newer=$$(find "$$dir" -type f -newermt "-$(UF_WORKFLOW_RETENTION_DAYS) days" -print -quit); then \
			echo "error: cannot evaluate retention for $$dir" >&2; exit 1; \
		fi; \
		if [ -z "$$newer" ]; then stale="$$stale $$dir"; fi; \
	done; \
	if [ -z "$$stale" ]; then \
		echo "clean-uf-workflows: no record lacks a write in the last $(UF_WORKFLOW_RETENTION_DAYS) days"; exit 0; \
	fi; \
	echo "clean-uf-workflows candidates (no write in the last $(UF_WORKFLOW_RETENTION_DAYS) days):"; \
	for dir in $$stale; do echo "  $$dir"; done; \
	if [ "$${PRUNE:-0}" != "1" ]; then \
		echo "clean-uf-workflows: listing only; set PRUNE=1 to delete these records"; exit 0; \
	fi; \
	for dir in $$stale; do rm -rf -- "$$dir"; done; \
	echo "clean-uf-workflows: pruned $$(echo $$stale | wc -w) record(s)"

man: ## generate man pages from docs/man/*.md (requires pandoc)
	@for md in docs/man/*.md; do \
		out="$${md%.md}.1"; \
		pandoc -s -t man "$$md" -o "$$out"; \
		echo "Generated $$out"; \
	done
