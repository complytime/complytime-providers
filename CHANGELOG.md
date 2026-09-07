# Changelog

## Unreleased

### Fixed

- **opa-provider**: Moved `url` and `input_path` from `RequiredTargetVariables` to `OptionalTargetVariableGroups` in `Describe()` response. `complyctl doctor` no longer reports false "missing variable" errors for these mutually exclusive variables. (Fixes #145)

### Security

- **ampel-provider**, **opa-provider**: Added aggregate extraction size limit (500 MB total) and file count limit (10,000 files) for complypack tar.gz archives. Previously, only individual files were capped at 100 MB. (Fixes #71)
- **opa-provider**: Fixed missing cleanup of partial extractions on error. Previously, a failed extraction left corrupted content that would be silently reused on subsequent attempts.

### Documentation

- Added man pages for all three provider binaries (`complyctl-provider-openscap`, `complyctl-provider-ampel`, `complyctl-provider-opa`) documenting configuration variables, environment variables, required external tools, and workspace paths. Man pages are maintained as Pandoc markdown in `docs/man/` and rendered via `make man`.

### Infrastructure

- Replaced inline release preflight and GoReleaser logic with org-infra reusable workflow calls (`reusable_release_preflight.yml`, `reusable_release_goreleaser.yml`), reducing `release.yml` from ~189 to ~63 lines. Adds `skip_semver_check`, `skip_ci_checks`, and `skip_unreleased_check` override inputs. Fixes semver ordering (`sort -V` → proper semver comparison) and adds smart re-run detection. Pinned to org-infra v0.7.1. (Fixes #158)
- Modernized RPM spec for Fedora Go Packaging Guidelines: adopted `go-vendor-tools` for vendored license handling, replaced raw `go build` with `%gobuild` macro, switched to forge macros and `%autorelease`, added aggregated vendored license expression, and introduced main `complytime-providers` package for shared license files.
- Consolidated duplicate tar.gz extraction code from ampel and OPA providers into shared `internal/archive/` package, eliminating code duplication and ensuring consistent security behavior across providers.

### Breaking Changes

- **openscap-provider**: `expandPath()` now uses `os.UserHomeDir()` instead of `user.Current().HomeDir` for tilde expansion. These functions diverge under `sudo` and in containers without NSS. The new behavior reads `$HOME` directly, matching how complyctl resolves paths.
- **all providers**: Re-vendored complyctl to adopt XDG Base Directory paths. Provider discovery moved from `~/.complytime/providers/` to `~/.local/share/complytime/providers/`. Policy/complypack cache moved from `~/.complytime/` to `~/.cache/complytime/`. State file moved from `~/.complytime/state.json` to `~/.local/share/complytime/state.json`. Workspace-local `.complytime/` is unchanged. Documentation and devcontainer scripts updated. Implements the complytime-providers side of ADR-0016.

- **openscap-provider**, **ampel-provider**: Removed Export RPC implementation and `SupportsExport: true` from Describe responses. The upstream complyctl removed the `Exporter` interface and all export infrastructure (complyctl PR #617, issue #606). No action required since complyctl no longer calls Export. Remove any local tooling that checks `SupportsExport`. Dependencies removed: `proofwatch`, `go-gemara`, `otlploggrpc`, `otel/sdk/log`.
- **ampel-provider**: Renamed granular policy IDs from benchmark-coupled `BP-X.YY` format to semantic, benchmark-agnostic slugs (`require-pull-request`, `minimum-approvals`, `block-force-push`, `prevent-admin-bypass`, `require-code-owner-review`). Updated corresponding `meta.controls[].id` references to semantic control IDs.
- **opa-provider**: OCI policy bundles MUST now include a `complytime-mapping.json` file. The fallback mode that evaluated all Rego namespaces via `--all-namespaces` when the mapping file was missing has been removed. Generate returns `{Success: false}` with an actionable error message when the mapping file is missing. (Fixes #34)

### Features

- **ampel-provider**: `AssessmentLog.Message` now uses the tenet's `error.message` (on failure) or `assessment.message` (on pass) instead of the generic `"X of Y repositories passed"` count string. Falls back to `"X of Y checks passed"` only when all step messages are empty.
- **ampel-provider**: `AssessmentLog.Recommendation` is now populated from the `error.guidance` field in Ampel tenet evaluations, providing platform-specific remediation instructions in complyctl reports.
- **ampel-provider**: `LoadGranularPolicies` now recursively walks subdirectories to find policy JSON files, enabling structured policy source directories. Includes symlink safety (skips symlinks), duplicate policy ID detection (returns error naming both paths), and uses `os.Root` for TOCTOU-safe file reads.
- **opa-provider**: Generate now accepts `ComplypackContentPath` from complyctl, using cached complypack content directly instead of requiring `opa_bundle_ref` + `conftest pull`. Supports both directory and tar.gz archive formats (extracted idempotently with path traversal protection). ComplypackContentPath takes precedence when both sources are provided.
- **opa-provider**: Added RPM sub-package (`complytime-providers-opa`) for Fedora packaging. Requires `conftest` CLI at runtime (not yet packaged in Fedora).

### Fixed

- **ampel-provider**: Emit error steps instead of an empty `steps` list when every repository scan errors (e.g. no GitHub token available), so evaluation logs always satisfy the Gemara schema's minimum-one-step constraint. (Fixes #141)
- **ampel-provider**: Synthesize passing assessment logs for requirements with zero findings so every evaluated requirement appears in the scan response. Previously, requirements where all checks passed were silently omitted from the `ScanResponse`. (Fixes #65)
- **openscap-provider**: Datastream auto-detection now uses CPE-based matching instead of filename heuristics. The provider parses `CPE_NAME` from the system's os-release file, extracts CPE platform metadata from each SSG datastream XML file, and selects the first datastream whose CPE matches the system. This eliminates the static `distroAliases` table, automatically adapts to new SSG products without code changes, and prevents false matches on systems with multiple datastreams installed. All file system paths are now configurable via environment variables: `SSG_CONTENT_DIR` (defaults to `/usr/share/xml/scap/ssg/content`) and `OS_RELEASE_FILE` (defaults to `/etc/os-release`). Error messages now provide actionable guidance when datastreams are not found or CPE matching fails. Previously, CentOS Stream fell through to RHEL via `ID_LIKE` fallback, selecting a datastream whose platform gate marked every rule `notapplicable`. Refactored `extractCPEFromOsRelease`, `extractCPEFromDatastream`, and `matchDatastreamByCPE` to accept `io.Reader` and `fs.FS` for testability. Added integration tests with real SSG content and environment variable overrides. (Fixes #91)
- **opa-provider**: Removed synthetic `scan-status` assessment entry that used a hardcoded `RequirementID` not matching any assessment plan ID. All `ScanResponse.Assessments` entries now contain valid plan IDs that `complyctl` can resolve via `resolveAssessmentIDs()`. (Fixes #67)
- **openscap-provider**: Scan results now restore original match IDs from assessment configuration instead of returning XCCDF rule short names. When a PlanID is present, the scan response uses it; otherwise falls back to the XCCDF short name. (Fixes complytime/complyctl#413)

### Infrastructure

- Added automated release workflow with GoReleaser v2, cosign signing, syft SBOMs, and preflight validation (tag format, semver ordering, CI verification, unreleased commits guard, concurrency protection).
- Added build-time version injection via `internal/version` package, replacing hardcoded version strings in all three provider `Describe` RPCs. Version is injected via `-ldflags` in Makefile, GoReleaser, and RPM spec builds.
- Updated Packit configuration to target Fedora 44 (replacing end-of-life Fedora 42).
