# Changelog

## Unreleased

### Breaking Changes

- **ampel-provider**: Renamed granular policy IDs from benchmark-coupled `BP-X.YY` format to semantic, benchmark-agnostic slugs (`require-pull-request`, `minimum-approvals`, `block-force-push`, `prevent-admin-bypass`, `require-code-owner-review`). Updated corresponding `meta.controls[].id` references to semantic control IDs.
- **opa-provider**: OCI policy bundles MUST now include a `complytime-mapping.json` file. The fallback mode that evaluated all Rego namespaces via `--all-namespaces` when the mapping file was missing has been removed. Generate returns `{Success: false}` with an actionable error message when the mapping file is missing. (Fixes #34)

### Bug Fixes

- **opa-provider**: `ToScanResponse` now synthesizes passing `AssessmentLog` entries for requirements with zero findings, ensuring Gemara EvaluationLog CUE schema compliance. Previously, requirements where all checks passed produced `steps: []`, which `cue vet` rejected. (Fixes #63)

### Features

- **ampel-provider**: `LoadGranularPolicies` now recursively walks subdirectories to find policy JSON files, enabling structured policy source directories. Includes symlink safety (skips symlinks), duplicate policy ID detection (returns error naming both paths), and uses `os.Root` for TOCTOU-safe file reads.
- **opa-provider**: Generate now accepts `ComplypackContentPath` from complyctl, using cached complypack content directly instead of requiring `opa_bundle_ref` + `conftest pull`. Supports both directory and tar.gz archive formats (extracted idempotently with path traversal protection). ComplypackContentPath takes precedence when both sources are provided.
