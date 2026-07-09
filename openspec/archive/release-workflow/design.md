## Context

complytime-providers ships three gRPC provider plugins (openscap, ampel, opa)
for the complyctl CLI. The repository was split from complyctl and has never
had its own release. The version is hardcoded as `"0.1.0"` in all three
provider `Describe` RPC responses and as `0.0.1` in the RPM spec. The Packit
integration for Fedora packaging is fully configured but blocked on a first
GitHub Release.

complyctl already has a working release pipeline using GoReleaser v2,
cosign signing, syft SBOMs, and a `workflow_dispatch` trigger. The gaze
project (unbound-force/gaze PR #100) extended this pattern with a preflight
validation job. complyctl issue #560 proposes adopting the same preflight
improvements.

This design adopts the preflight pattern from the start, producing a release
workflow that is already ahead of complyctl's current state.

## Goals / Non-Goals

**Goals:**

- One-command release: `gh workflow run release.yml -f tag=v0.1.0`
- Preflight validation: tag format, semver ordering, CI verification,
  unreleased commits guard, idempotent tag creation, concurrency protection
- Per-provider binary archives with checksums, cosign signatures, and SBOMs
- Build-time version injection replacing all hardcoded version strings
- OPA provider included in release artifacts, RPM spec, and TMT tests
- Packit target alignment with complyctl (f44, f43 instead of f42, f43)
- Consistent patterns with complyctl for cross-project maintainability

**Non-Goals:**

- Multi-architecture builds (Linux amd64 only, matching complyctl)
- macOS/Windows support
- Homebrew tap or other distribution channels beyond RPM
- CHANGELOG.md automation (remains manually curated; version bump happens at
  release time, not in this change)
- Full complyctl version parity (commit, buildDate, gitTreeState) -- providers
  expose only a single `Version` string through the Describe RPC

## Decisions

### 1. Lean version package with v-prefix normalization

**Decision**: Create `internal/version/version.go` with a single `version`
variable and `Version()` accessor function that strips the `v` prefix if
present. The canonical version format is `0.1.0` (no `v` prefix). Do not
include `commit`, `buildDate`, or `gitTreeState`.

**Rationale**: The provider `Describe` RPC returns a single `Version` string.
There is no `--version` CLI flag and no user-facing version template. Injecting
four variables adds complexity to the Makefile, spec, and GoReleaser config
for information that is never surfaced. The `v` prefix is stripped in
`Version()` to ensure consistent output regardless of whether the injected
value comes from a git tag (`v0.1.0`), GoReleaser (`0.1.0` via
`{{ .Version }}`), or the RPM spec (`0.1.0` via `%{version}`). If a richer
version is needed later, the package can be extended without breaking changes.

**Alternative considered**: Mirror complyctl's `internal/version/version.go`
exactly (4 variables, `WriteVersion()` function, template rendering). Rejected
as speculative complexity.

### 2. Per-provider archives over a combined archive

**Decision**: GoReleaser produces three separate `.tar.gz` archives, one per
provider binary.

**Rationale**: The RPM spec already distributes providers as separate
sub-packages. Users install only what they need. Per-provider archives
maintain this principle at the GitHub Release level. Each archive is named
`complyctl-provider-<name>_linux_x86_64.tar.gz`.

**Alternative considered**: Single combined archive containing all three
binaries. Simpler GoReleaser config, but forces downloading all providers.

### 3. workflow_dispatch trigger over tag-push auto-release

**Decision**: Release is triggered manually via `workflow_dispatch` with a
`tag` input. The workflow creates the annotated git tag after preflight
validation passes.

**Rationale**: Manual trigger gives maintainers explicit control during the
initial release phase. The preflight job creates the tag idempotently, so the
maintainer does not need to push a tag separately. This matches complyctl's
pattern and avoids accidental releases from tag pushes.

### 4. Preflight validation job (gaze pattern)

**Decision**: Add a `preflight` job before the `release` job with six
validation steps: tag format, tag uniqueness, semver ordering, CI verification
via GitHub Checks API, unreleased commits guard, and annotated tag creation.

**Rationale**: Implements all five improvements from complyctl issue #560.
Better to build these safety nets from the start than retrofit them. The CI
check names to verify are `Build and test` (from `ci_local.yml`) and
`Standardized CI / Run linters` (from `ci_checks.yml` calling
`reusable_ci.yml`).

### 5. Remove release job and tag trigger from ci_local.yml

**Decision**: Strip the `release` job and `tags: [v*]` trigger from
`ci_local.yml`, leaving it as build+test only.

**Rationale**: The preflight job verifies CI status on HEAD via the Checks API.
The tag is created by the preflight job and points at the already-verified
commit. Running CI again on the tag is redundant work.

### 6. OPA RPM sub-package without Requires for conftest

**Decision**: Add `%package opa` to the spec with `Requires: complyctl >= 0.0.8`
and `Requires: git` but no `Requires: conftest`. Document the conftest
requirement in the package description.

**Rationale**: `conftest` is not yet available in Fedora repositories. The
provider performs runtime tool checking (`toolcheck.CheckTools`) and reports
missing tools through the Describe RPC's `ErrorMessage` field. This matches
the ampel provider pattern where `snappy` and `ampel` CLI tools are also not
packaged.

**Scope justification**: Including the OPA sub-package in this change rather
than a separate one is deliberate. Shipping a first release (v0.1.0) without
all existing providers would create a false packaging gap -- users would find
the OPA provider in the source code and build system but not in the release
artifacts or RPM packages. Since the OPA provider is already built and tested
by CI, adding it to the spec and release is a packaging concern (not a feature
addition) that belongs in the release infrastructure change.

### 7. Workflow file naming: release.yml

**Decision**: Name the release workflow `release.yml`, not `ci_release.yml`.

**Rationale**: The constitution defines `ci_` prefix for consumer workflows
and `reusable_` for reusable workflows. However, the release workflow is
not a CI workflow -- it does not run on push or PR events. It is a manual
release operation triggered by `workflow_dispatch`. Using `release.yml`
matches complyctl's naming (`release.yml`) for cross-project consistency,
which is a stated goal of this change. The semantic distinction between
CI (automated, event-driven) and release (manual, human-triggered) justifies
the naming divergence from the `ci_*` convention used by the seven existing
event-driven workflows.

## Risks / Trade-offs

**[CI check name coupling]** The preflight job hardcodes CI check names
(`Build and test`, `Standardized CI / Run linters`). If workflow job names
change in `ci_local.yml` or org-infra's `reusable_ci.yml`, the preflight
will fail closed (release blocked until names are updated). This is the safe
failure mode -- a false block is preferable to releasing without CI
verification. A comment in the workflow documents this coupling.

**[First release bootstrapping]** The semver ordering check and unreleased
commits guard use `git tag -l 'v[0-9]*'` to find the latest tag. For the
first-ever release, no tags exist, and both steps handle this gracefully
(first release is always valid, all commits count as unreleased).

**[GoReleaser vendor hook]** The GoReleaser `before.hooks` runs `go mod tidy`
and `go mod vendor` like complyctl. This ensures the release build uses fresh
vendored dependencies. If `go.sum` has drifted from `go.mod`, GoReleaser will
catch it during the pre-hook rather than producing a broken release.

**[Tag protection rules]** The preflight job creates and pushes annotated
tags using the `GITHUB_TOKEN`. If the repository has tag protection rules
enabled, the `GITHUB_TOKEN` may lack permission to create tags, causing a
clear error at the tag creation step. This is acceptable -- tag protection
is a repository setting that maintainers control, and the error message from
`git push` is sufficient to diagnose the issue.

**[Pre-release versions]** The tag format validation (`vMAJOR.MINOR.PATCH`)
explicitly rejects pre-release suffixes (e.g., `v1.0.0-rc1`). If pre-release
support is needed later, the regex and semver comparison logic can be extended
without changing the workflow structure or other requirements.

**[Cosign local testing]** Cosign keyless signing requires a CI OIDC token
and cannot be fully tested locally. The `goreleaser release --snapshot`
dry-run validates everything except signing. The first real release is the
only full end-to-end test of the signing configuration.
