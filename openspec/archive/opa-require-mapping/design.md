## Context

PR #31 implemented the OPA provider's Generate RPC with an optional
`complytime-mapping.json` file in OCI policy bundles. When the file is
missing, Generate writes a scan config with `ids: null` and Scan falls
back to `--all-namespaces`, producing Rego-derived requirement IDs that
complyctl cannot correlate with the Gemara assessment plan.

Issue #34 identifies that this fallback produces semantically unreliable
compliance results. The provider is not yet in production and the
related Gemara policies are in early development, making this the
cheapest time to remove the fallback.

The current fallback chain spans four sites:
1. `server.go` Generate method (lines 133-143): catches all
   `LoadMapping` errors, writes nil scan config, returns Success
2. `server.go` evalAndParse (line 418): branches on `scanCfg.IDs != nil`
3. `scan.go` EvalPolicy: the `--all-namespaces` conftest command
4. `results.go` ResolveRequirementID: nil-map passthrough

## Goals / Non-Goals

**Goals:**
- Require `complytime-mapping.json` in OCI bundles for the OPA provider
- Return `{Success: false}` with an actionable error when the file is
  missing from the bundle
- Distinguish missing-file errors from malformed-file errors so
  validation failures are not masked
- Remove the `--all-namespaces` fallback path from Scan
- Update tests to verify error behavior instead of fallback behavior

**Non-Goals:**
- Changing the mapping file format or validation rules
- Modifying how matched requirements or reverse mappings work (the
  happy path is unchanged)
- Removing `EvalPolicy` if it is used by other callers (it is not --
  but verify before removing)

## Decisions

### 1. Distinguish missing vs. malformed in LoadMapping

`LoadMapping` currently wraps all errors identically. The change adds
an `os.IsNotExist` check on the `os.ReadFile` error to return a
sentinel error (or a distinguishable error type) for "file not found."
The caller in Generate uses this to produce a specific error message
for missing files vs. propagating validation errors for malformed files.

**Alternative considered**: returning `(nil, nil)` for missing files.
Rejected because nil-nil returns are error-prone and the caller needs
distinct error messages for each case.

### 2. Error in Generate, not in LoadMapping

The `{Success: false}` response is constructed in the Generate method,
not in LoadMapping. LoadMapping remains a pure I/O + validation function
that returns errors. Generate decides how to surface those errors to the
plugin framework. This keeps LoadMapping testable without coupling it
to the provider response type.

### 3. Remove the nil-IDs branch in evalAndParse

The `scanCfg.IDs == nil` branch in evalAndParse (line 418) currently
triggers `--all-namespaces`. Since Generate now guarantees IDs are
never nil on success, this branch becomes dead code. Remove it and
treat nil IDs as a configuration error.

### 4. Keep EvalPolicy and constructConftestTestCommand

Although the `--all-namespaces` path is no longer reachable through
the Generate/Scan pipeline, `EvalPolicy` is a valid conftest
invocation function that may be useful for future features (e.g.,
bundle validation tooling). However, since it currently has no
callers outside the fallback path, it can be safely removed to
reduce dead code. If needed later, it can be re-added.

## Risks / Trade-offs

- **[Risk]** OCI bundles without mapping files stop working →
  Mitigated by the fact that the provider is not in production and the
  error message tells the user exactly what file to add and where.
- **[Risk]** Removing EvalPolicy breaks future use →
  Mitigated by the function being trivial to re-add (5 lines). Dead
  code removal is preferred per project conventions.
- **[Risk]** Existing tests rely on fallback behavior →
  Mitigated by updating tests in the same change. The fallback
  integration test becomes an error-verification test.
