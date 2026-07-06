## Project Overview

complytime-providers ships compliance-scanning provider plugins for
the `complyctl` CLI. Each provider implements the `complyctl`
gRPC plugin interface (hashicorp/go-plugin) with three core RPCs
(`Describe`, `Generate`, `Scan`).

- **Type**: Multi-binary Go plugin repository
- **Binaries**: `complyctl-provider-openscap`, `complyctl-provider-ampel`,
  `complyctl-provider-opa`
- **License**: Apache-2.0
- **Go version**: 1.25.0
- **Key dependencies**: complyctl (plugin framework), hashicorp/go-plugin,
  stretchr/testify, antchfx/xmlquery

## Build & Test Commands

Build, test, and lint commands derived from `Makefile` and
`.github/workflows/ci_local.yml`:

```bash
# Build all provider binaries to bin/
make build

# Build individual providers
make build-openscap-provider
make build-ampel-provider
make build-opa-provider

# Run all unit tests
make test

# Run linter (golangci-lint with default config)
make lint

# Vendor dependencies
make vendor

# Smoke-test devcontainer (Containerfile build + shellcheck)
make test-devcontainer
```

### CI Workflow Structure

| Workflow | File | Triggers | Jobs |
|----------|------|----------|------|
| ci | `.github/workflows/ci_local.yml` | push to main, PRs | build-and-test |
| release | `.github/workflows/release.yml` | workflow_dispatch | preflight, release |

## Project Structure

```text
complytime-providers/
├── .devcontainer/
│   ├── Containerfile          #   Fedora 43 dev environment
│   ├── devcontainer.json      #   Devcontainer configuration
│   └── scripts/
│       └── post-create.sh     #   Setup automation script
├── cmd/
│   ├── openscap-provider/     # Binary: complyctl-provider-openscap
│   │   ├── config/            #   Configuration handling
│   │   ├── oscap/             #   OpenSCAP tool invocation
│   │   ├── scan/              #   Scan orchestration
│   │   ├── server/            #   gRPC provider implementation
│   │   ├── xccdf/             #   XCCDF datastream & tailoring
│   │   └── xccdftype/         #   XCCDF type definitions
│   ├── ampel-provider/        # Binary: complyctl-provider-ampel
│   │   ├── config/            #   Configuration handling
│   │   ├── convert/           #   Format conversion & types
│   │   ├── generate/          #   Generate RPC: scan config persistence
│   │   ├── intoto/            #   in-toto attestation handling
│   │   ├── results/           #   Results processing
│   │   ├── scan/              #   Scan orchestration
│   │   ├── server/            #   gRPC provider implementation
│   │   ├── targets/           #   Target resolution
│   │   └── toolcheck/         #   Tool availability checking
│   └── opa-provider/          # Binary: complyctl-provider-opa
│       ├── config/            #   Configuration handling
│       ├── generate/          #   Generate RPC: mapping & scan config
│       ├── loader/            #   Data loading (git clone, local path)
│       ├── results/           #   Conftest result parsing & mapping
│       ├── scan/              #   Conftest command execution
│       ├── server/            #   gRPC provider implementation
│       ├── targets/           #   Target resolution & URL parsing
│       └── toolcheck/         #   Tool availability checking
├── internal/
│   ├── complytime/
│   │   └── testdata/openscap/ # XML test fixtures
│   └── version/               # Build-time version injection
├── docs/                      # Provider development guide
├── plans/                     # TMT/FMF RPM validation tests
├── .github/workflows/         # CI configuration
├── openspec/                  # OpenSpec change workflow
├── .opencode/                 # Agent definitions & convention packs
├── .goreleaser.yaml           # GoReleaser v2 release config
└── complytime-providers.spec  # RPM packaging spec
```

## Coding Conventions

- **Formatting**: `gofmt` and `goimports` (standard Go formatting)
- **Linting**: `golangci-lint run ./...` (default configuration,
  no `.golangci.yml`)
- **File headers**: `// SPDX-License-Identifier: Apache-2.0`
- **Import grouping**: stdlib, external, internal (goimports order)
- **Error handling**: errors MUST be checked and returned to caller
  when the current function cannot resolve them
- **Naming**: lowercase packages, no underscores in package names;
  file names use lowercase with underscores
- **Line length**: 99 characters unless exceeding improves readability

## Testing Conventions

- **Framework**: Go stdlib `testing` + `github.com/stretchr/testify`
  (assert/require)
- **Naming**: `TestFunctionName_Description` pattern
- **Coverage**: 28 test files across all subpackages (every non-type
  package has a corresponding `_test.go`)
- **Test data**: XML fixtures stored in
  `internal/complytime/testdata/openscap/`
- **Run**: `go test ./...` (via `make test`)

## Behavioral Rules

These rules derive from the ComplyTime Constitution at
`.specify/memory/constitution.md`. That file is the authoritative
source. The rules below are the non-negotiable operational
constraints extracted for agent enforcement. Violations are
CRITICAL severity.

- **Gatekeeping**: MUST NOT modify quality/governance gates
  (coverage thresholds, CRAP scores, severity definitions,
  CI flags, agent settings, constitution MUST rules, review
  limits, workflow markers). Stop and report instead.
- **Phase boundaries**: MUST NOT cross workflow phase boundaries.
  Spec phases: spec artifacts only. Implement: source code.
  Review: fixes only. Violation = process error, stop immediately.
- **CI parity**: MUST replicate CI checks locally before marking
  tasks complete. Derive commands from `.github/workflows/`.
- **Review council**: MUST run `/review-council` before PR
  submission. Resolve all REQUEST CHANGES. No code changes
  between APPROVE and PR. Exempt: constitution amendments,
  docs-only, emergency hotfixes.
- **Branch protection**: MUST NOT commit directly to `main`.
  All changes via feature branches and PRs.
- **Documentation gate**: Before marking a task complete,
  assess documentation impact: `CHANGELOG.md` for change
  entries, `AGENTS.md` for structural updates (project
  structure, conventions, build commands), `README.md` for
  description changes.
- **Website gate**: MUST file `unbound-force/website` issue
  for user-facing changes before PR merge. Exempt: internal
  refactoring, test-only, CI-only, spec artifacts.
- **Zero-waste**: No orphaned specs, unused standards, or
  aspirational documents that do not map to actionable work.

### PR Review Commands

| Command | When | Scope |
|---------|------|-------|
| `/review-council` | Pre-PR (local) | 5+ Divisor agents |
| `/review-pr [N]` | Post-PR (GitHub) | Single agent, CI analysis |

## Specification Workflow

All non-trivial changes MUST be preceded by a spec workflow.

| Tier | Tool | When | Artifacts |
|------|------|------|-----------|
| Strategic | Speckit | >= 3 stories, cross-repo | `specs/NNN-*/` |
| Tactical | OpenSpec | < 3 stories, single-repo | `openspec/changes/*/` |

Pipeline: `constitution → specify → clarify → plan → tasks →
analyze → checklist → implement`

**Ordering**: Constitution before specs. Spec before plan. Plan
before tasks. Tasks before implementation. Spec artifacts MUST
be committed/pushed before implementation begins.

**Branches**: Speckit: `NNN-<name>`. OpenSpec: `opsx/<name>`.

**Task bookkeeping**: Mark checkboxes `[x]` immediately on
completion. `[P]` marks parallel-eligible tasks.

**When in doubt**: Start with OpenSpec. Escalate to Speckit if
scope grows beyond 3 stories or crosses repo boundaries.

**What requires a spec**: New features, refactoring that changes
signatures, test additions across multiple functions, agent
changes, CI changes, data model changes.

**Exempt**: Constitution amendments, typo fixes, emergency
hotfixes (retroactively documented).

## Convention Packs

This repository uses convention packs scaffolded by
unbound-force. Agents MUST read the applicable pack(s)
before writing or reviewing code.

- `.opencode/uf/packs/default.md`
- `.opencode/uf/packs/default-custom.md`
- `.opencode/uf/packs/severity.md`
- `.opencode/uf/packs/content.md`
- `.opencode/uf/packs/content-custom.md`
- `.opencode/uf/packs/go.md`
- `.opencode/uf/packs/go-custom.md`

## Architecture

All three providers follow an identical plugin pattern: `main.go`
instantiates the provider via `server.New()` and passes it to
`provider.Serve()` from the `complyctl` package. The complyctl
framework manages gRPC subprocess lifecycle via hashicorp/go-plugin.

Each provider is self-contained under `cmd/<name>-provider/` with
its own subpackage hierarchy (config, scan, server, plus
domain-specific packages). The only shared code between providers is `internal/version/`,
which provides build-time version injection via ldflags.
`internal/complytime/` contains only test fixtures.
