## Why

The OPA provider's Generate RPC (PR #31) includes a fallback mode for OCI
policy bundles that lack a `complytime-mapping.json` file. In fallback mode,
Generate writes a scan config with `ids: null` and Scan evaluates every Rego
namespace via `--all-namespaces`, returning Rego-derived requirement IDs that
complyctl cannot correlate with the Gemara assessment plan. This produces
results that are structurally valid but semantically unreliable for compliance
reporting. Since the OPA provider is not yet in production and related Gemara
policies are still in early development, this is the right time to require
the mapping file before the fallback becomes an implicit contract.

## What Changes

- Generate returns `{Success: false}` with an actionable error message when
  `complytime-mapping.json` is missing from the OCI bundle, instead of
  silently degrading to unfiltered evaluation
- Generate distinguishes missing-file errors from malformed-file errors in
  `LoadMapping`, so validation errors (bad JSON, empty fields, duplicates)
  are not masked as "missing file"
- Scan treats `scanCfg.IDs == nil` as a configuration error rather than a
  signal to fall back to `--all-namespaces`
- **BREAKING**: OCI policy bundles MUST include a `complytime-mapping.json`
  file to be usable with the OPA provider

## Capabilities

### New Capabilities

### Modified Capabilities

- `opa-generate`: REQ-GEN-007 (fallback on missing mapping) and
  REQ-COMPAT-001 (backward compatibility with unmapped bundles) are
  superseded -- missing mapping file is now a hard error, and the
  `--all-namespaces` fallback path in Scan is removed

## Impact

- `cmd/opa-provider/generate/mapping.go`: `LoadMapping` must distinguish
  `os.ErrNotExist` from other errors
- `cmd/opa-provider/server/server.go`: `Generate` method fallback branch
  (lines 133-143) replaced with error return; `evalAndParse` nil-IDs
  branch removed
- `cmd/opa-provider/scan/scan.go`: `EvalPolicy` (the `--all-namespaces`
  function) and `constructConftestTestCommand` can be removed
- `cmd/opa-provider/results/results.go`: `ResolveRequirementID` nil-map
  passthrough path can be removed
- Test files: `TestGenerate_Scan_FallbackPath`, `TestGenerate_WithoutMapping`,
  and related nil-mapping tests updated to verify error responses
- Fixes: https://github.com/complytime/complytime-providers/issues/34
