## Why

The OPA provider's `Generate` RPC is a stub that returns `{Success: true}` and ignores the Gemara assessment plan entirely. During Scan, it pulls a full OCI policy bundle and evaluates every rule with `--all-namespaces`. This means OPA evaluates rules the assessment plan didn't ask for, returns Rego-derived IDs that complyctl cannot correlate back to the policy's requirement structure, and bypasses the Generate/Scan contract that both OpenSCAP and AMPEL providers honor. Implementing Generate brings the OPA provider to parity with the other providers and enables complyctl to produce accurate, requirement-scoped compliance reports for OPA-based policies.

## What Changes

- **Implement the OPA provider's `Generate` RPC** to read `req.Configuration` (Gemara assessment plan RequirementIDs), pull the OCI policy bundle, read a mapping file from the bundle, match RequirementIDs to Rego namespaces, and write a generation artifact (`scan-config.json`) that Scan consumes.
- **Add namespace-filtered conftest evaluation** so Scan passes `--namespace ns1 --namespace ns2 ...` instead of `--all-namespaces` when a generation artifact exists.
- **Add reverse ID mapping in scan results** so `AssessmentLog.RequirementID` reports Gemara requirement IDs (e.g., `"CIS-K8S-5.2.6"`) instead of Rego-derived names (e.g., `"kubernetes.run_as_root"`) when a mapping is available.
- **Graceful fallback** for bundles without a mapping file: Generate logs a warning and Scan falls back to `--all-namespaces` with Rego-derived IDs. This is a first-class supported mode, not a deprecated shim.

## Capabilities

### New Capabilities
- `opa-generate`: Implement the Generate RPC for the OPA provider, including mapping file loading, RequirementID matching, namespace filtering, and generation artifact management.

### Modified Capabilities
<!-- None -- no existing specs to modify -->

## Impact

- **Code**: `cmd/opa-provider/` -- server, scan, results, and config packages. New `generate` package for mapping types and orchestration.
- **OCI bundle convention**: Policy bundles may optionally include a `complytime-mapping.json` file. Existing bundles without this file continue to work unchanged.
- **No upstream changes**: No modifications to `complyctl`, the gRPC plugin interface, or the other two providers.
- **No new external dependencies**: Uses existing `conftest` CLI flags (`--namespace`).
- **Backward compatible**: Bundles without mapping files produce identical behavior to today.
