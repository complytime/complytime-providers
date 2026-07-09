## Why

complytime-providers lacks an automated, repeatable release process. Providers
were previously co-released with complyctl from a shared repository; after the
split, no release workflow was established. Without one, cutting a release
requires manual binary builds, no supply-chain artifacts (SBOMs, signatures),
and no safety nets against human error. The Packit integration for Fedora
packaging is fully wired but has never been triggered because no GitHub Release
has ever been created.

Establishing a release workflow now unblocks the first versioned release
(v0.1.0), enables automated Fedora package proposals via Packit, and aligns
with complyctl's release standards for consistency across the project.

## What Changes

- Add a `release.yml` GitHub Actions workflow with `workflow_dispatch` trigger
  and a preflight validation job (tag format, semver ordering, CI verification
  via Checks API, unreleased commits guard, idempotent annotated tag creation,
  concurrency protection).
- Add GoReleaser v2 configuration producing per-provider binary archives,
  checksums signed with cosign (Sigstore keyless), SBOMs via syft, and
  auto-generated changelogs from conventional commits.
- Add a lean `internal/version` package with a single `version` variable
  injected via `-ldflags` at build time, replacing hardcoded `"0.1.0"` strings
  in all three provider `Describe` RPCs.
- Update `Makefile` to inject version via ldflags in all build targets.
- Update `complytime-providers.spec` to add the OPA provider sub-package, add
  ldflags-based version injection in `%build`, and bump to version `0.1.0`.
- Update `.packit.yaml` to replace end-of-life `f42` targets with `f44`.
- Update `plans/test-RPM-providers.fmf` to validate the OPA provider binary.
- Simplify `ci_local.yml` to build+test only (remove release job and tag
  trigger).

## Capabilities

### New Capabilities

- `release-pipeline`: Automated release workflow with preflight validation,
  GoReleaser-based artifact generation, supply-chain signing, and SBOM
  production.
- `version-injection`: Build-time version injection via `-ldflags` replacing
  hardcoded version strings, with consistent behavior across GoReleaser, Make,
  and RPM spec builds.

### Modified Capabilities

None.

## Impact

- **New files**: `internal/version/version.go`, `.goreleaser.yaml`,
  `.github/workflows/release.yml`
- **Modified files**: `cmd/*/server/server.go` (3 files), `Makefile`,
  `complytime-providers.spec`, `.packit.yaml`, `.github/workflows/ci_local.yml`,
  `plans/test-RPM-providers.fmf`
- **Dependencies**: No new Go dependencies (version package is pure stdlib).
  GoReleaser, cosign, and syft are CI-only tools installed during workflow
  execution.
- **RPM packaging**: New `complytime-providers-opa` sub-package. OPA provider
  requires `conftest` at runtime (not yet in Fedora; documented in spec
  description).
- **Downstream**: First GitHub Release triggers Packit `propose_downstream`,
  creating dist-git PRs for Fedora rawhide, f44, f43.
