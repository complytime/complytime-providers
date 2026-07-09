## Context

The complyctl repository has a devcontainer-based testing environment
(merged to main). This change mirrors that setup in complytime-providers,
following the established pattern where complyctl owns canonical
infrastructure and complytime-providers consumes it.

The key difference: here, providers are built from the local PR branch
while complyctl is cloned from main. In complyctl's devcontainer, it's
the reverse.

## Goals / Non-Goals

**Goals:**

- One-command path to a Fedora testing environment for provider PRs.
- Reuse complyctl's test infrastructure by cloning it at setup time.
- Consistency with complyctl's devcontainer (same image, packages,
  tool versions, operational patterns).

**Non-Goals:**

- Owning or maintaining test content (belongs to complyctl).
- Publishing a pre-built container image.
- Replacing the complytime-demos Vagrant setup.

## Decisions

### D1: Mirror complyctl's devcontainer configuration and patterns

**Decision**: Mirror complyctl's Containerfile, devcontainer.json, and
post-create script patterns. This includes all design decisions
documented in complyctl's `openspec/changes/dev-testing-environment/
design.md` (D1-D10): devcontainer.json standard, Fedora base image,
no custom registry, cross-repo test fixtures, pinned tool versions,
GOTOOLCHAIN=auto, SELinux runArgs, auto-rebuild hook, GITHUB_TOKEN
least-privilege, and canonical source ownership.

**Rationale**: Both devcontainers need identical system-level setup.
Complyctl's implementation has been tested and improved by multiple
maintainers. Mirroring it avoids re-learning the same lessons.

### D2: Reversed build order -- providers local, complyctl from main

**Decision**: The post-create script builds providers from local source
(`make build`) and clones complyctl from main. This is the reverse of
complyctl's devcontainer (D10), which builds complyctl locally and
clones providers from main.

**Rationale**: The PR under review is in complytime-providers, so local
source must be providers. complyctl from main provides the CLI and mock
registry infrastructure.

### D3: Documentation references complyctl docs for shared concepts

**Decision**: Provider docs reference complyctl's
`docs/TESTING_ENVIRONMENT.md` (via GitHub `main` branch blob URL) for
Codespaces, DevPod, VS Code setup, and troubleshooting. Only
provider-specific setup is documented here.

**Rationale**: Avoids duplicating documentation that would drift.

### D4: Graceful handling of missing provider binaries

**Decision**: Loop over all known providers (`openscap`, `ampel`, `opa`)
when copying binaries. Skip missing binaries with a WARNING instead
of failing. The `complyctl-provider-opa` binary is not yet built in
this repository; the graceful skip path will always apply for OPA
until the provider is added.

**Rationale**: Forward-compatible with the OPA provider when it is added
to this repo. Matches complyctl's post-create.sh pattern.

### D5: Tag-based version pinning for development tools

**Decision**: Install snappy (`github.com/carabiner-dev/snappy@v0.2.4`),
ampel (`github.com/carabiner-dev/ampel/cmd/ampel@v1.2.1`), and conftest
(`github.com/open-policy-agent/conftest@v0.68.2`) at pinned version
tags via `go install`. This follows complyctl's approach (D6).

**Rationale**: Tag-based pinning is accepted for a development
environment where reproducibility is valued but full supply chain
verification (commit SHA or checksum pinning) is not warranted. Go
module tags are enforced by the module proxy checksum database
(`sum.golang.org`), which provides integrity verification against
tag mutation. This is a deliberate trade-off: CI uses pinned action
SHAs for higher assurance, while the devcontainer uses version tags
for developer ergonomics.

## Risks / Trade-offs

Shared risks (container limitations, build time, GITHUB_TOKEN, podman
rootless ownership, Fedora tag pinning) are documented in complyctl's
`openspec/changes/dev-testing-environment/design.md` and apply equally
here.

Provider-specific risks:

- **[Dependency on complyctl main]** -- If complyctl main is broken, the
  providers devcontainer fails. Same risk the cross-repo integration test
  already accepts.

- **[Containerfile duplication]** -- Accepted. The Containerfile is
  minimal and both repos need identical system packages. Each
  Containerfile includes a comment referencing the other repo's
  Containerfile as the sync source to signal when updates are needed.

- **[Cross-repo doc link stability]** -- Provider docs reference
  complyctl's `docs/TESTING_ENVIRONMENT.md` via GitHub `main` branch
  URL. If complyctl renames or moves this file, the link will break.
  Mitigation: use `main` branch URLs (auto-update as main advances)
  and include brief inline summaries so the doc remains useful if the
  link breaks.
