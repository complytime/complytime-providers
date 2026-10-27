# Design

## Context

See proposal.md for motivation. The ampel provider invokes `ampel verify`
as a CLI tool and parses the resulting in-toto ResultSet attestation.
Three gaps prevent correct handling of policies that don't apply to a
given attestation's predicate type.

Key findings from testing ampel 1.4.2 (validated locally):

1. **`predicates` in `when:` context = ALL loaded attestations**, not
   filtered by the policy's tenet predicate types. The expression
   `size(predicates) > 0` (as suggested in the issue) does NOT work —
   it's always true when any attestation exists.

2. **The correct `when` expression** must filter by `predicate_type`:
   ```
   predicates.exists(p, p.predicate_type == '<type-url>')
   ```

3. **SKIP eval results** have `id: "when"`, `status: "SKIP"`, and
   carry the skip reason in `assessment.message`.

4. **Overall status** is `"PASS"` when mixed PASS+SKIP results exist.
   Overall `"SKIP"` only occurs when ALL policies skip. The
   `--allow-skip` flag controls whether all-SKIP produces `"SKIP"`
   (with flag) or `"FAIL"` (without flag) at the root level.

## Goals / Non-Goals

**Goals:**
- Parse SKIP eval results and overall status correctly
- Pass `--allow-skip` to preserve per-policy SKIP signals
- Support `WhenClause` serialization for policy round-trip
- Detect all-SKIP results as error (no applicable policies)
- Add comprehensive test coverage for SKIP paths

**Non-Goals:**
- Auto-injecting `when` guards during bundle generation (policy
  authoring concern, belongs in org-infra)
- VSA/SVR output format support (separate future work)
- Updating org-infra policies (separate repo/PR)

## Decisions

### 1. Parse SKIP in ParseAmpelOutput (results.go)

**Decision**: Add explicit `"SKIP"` case to both the eval result switch
(line 155) and overall status check (line 180-185).

**Rationale**: The existing `mapResult` function already handles
`"skip"` → `provider.ResultSkipped` (line 553) but is unreachable
because `ParseAmpelOutput` never produces `finding.Result = "skip"`.
Adding the case makes the existing downstream path reachable.

For eval results, SKIP carries `assessment.message` (not `error`), so
the SKIP case mirrors the PASS case for message extraction.

For overall status, convert the binary if/else to a switch for
consistency and add a `"SKIP"` case.

**Alternative considered**: Treating SKIP as a new finding type (not
reusing the existing `"skip"` string). Rejected because the `mapResult`
path already uses `"skip"` and `provider.ResultSkipped`.

### 2. Pass --allow-skip unconditionally (scan.go)

**Decision**: Add `"--allow-skip"` to `constructAmpelVerifyCommand`.

**Rationale**: Without this flag, ampel normalizes root-level SKIP to
FAIL, which prevents the provider from detecting the all-SKIP error
condition. The provider uses `--attest-results` (ResultSet format)
exclusively, and `--allow-skip` is safe to use with it.

**Alternative considered**: Making `--allow-skip` conditional (e.g.,
only when multi-predicate policies detected). Rejected because it
adds complexity with no benefit — the flag is harmless when no policies
skip, and beneficial when they do.

### 3. All-SKIP = error in server.Scan (server.go)

**Decision**: After `ParseAmpelOutput` returns, check whether all
findings have result `"skip"`. If so, convert the result to error
status with a descriptive message.

**Rationale**: If every policy in the bundle skipped for a given
attestation, it means no policy's declared predicate types matched
the attestation type. This is a misconfiguration, not a valid scan
result. The provider should report it as error.

The check is a method on `PerRepoResult` (in results.go) for
testability, but the conversion to error lives in `server.Scan`
to keep `ParseAmpelOutput` as a pure parser.

**Alternative considered**: Checking in `ParseAmpelOutput`. Rejected
because it mixes business logic with parsing. Also considered checking
in `ToScanResponse`, but that's too late — each `PerRepoResult` is
also written to disk, and the error should be reflected there.

### 4. WhenClause type (convert/types.go)

**Decision**: Add `When *WhenClause` field to `AmpelPolicy` and a
`WhenClause` struct with `Expression string`.

**Rationale**: The provider loads policies from disk, merges them
into a bundle, and writes the bundle for ampel. If the source policies
contain `when:` guards, the type must carry the field through the
round-trip. The `*WhenClause` pointer with `omitempty` avoids
emitting the field for policies that don't need it.

**Alternative considered**: Using `map[string]interface{}` for
forward-compatible extensibility. Rejected because ampel's `when`
schema is `{expression: string}` and Go's type safety is preferred
over speculative flexibility.

### 5. Version guard in toolcheck (toolcheck.go)

**Decision**: Add `CheckVersion(minVersion string) error` to
`toolcheck.go`. Run `ampel version`, parse the version string,
and return an error if below the minimum. Call from
`checkRequiredTools` in `server.go` after the existing PATH check.

**Rationale**: Adding `--allow-skip` unconditionally means ampel
< 1.4.2 will fail with a confusing "unknown flag" error that
surfaces only when reading the missing results file. The version
guard fails fast with an actionable message ("ampel >= 1.4.2
required") at `Generate`/`Scan` time, before any work is done.
No semver library is needed — a simple major.minor.patch string
comparison is sufficient for a fixed version floor.

**Alternative considered**: Making `--allow-skip` conditional on
detected version (degrade gracefully). Rejected because it would
silently hide SKIP results for old ampel versions, defeating the
purpose of this change. Also considered documentation-only
approach, but the RPM spec cannot express the constraint (ampel
is not packaged) and the devcontainer/README are not enforcement
mechanisms.

## Risks / Trade-offs

- **[Risk] `when` expression requires per-policy predicate type URLs**:
  The correct `when` expression is
  `predicates.exists(p, p.predicate_type == '<url>')`, not the generic
  `size(predicates) > 0` described in the issue. This means each
  policy needs its own `when` expression matching its tenet predicate
  types. → Mitigation: This is a policy authoring concern (org-infra),
  not a provider concern. The provider just round-trips whatever `when`
  value the policy has.

- **[Risk] Ampel behavior may change in future versions**: The SKIP
  handling and `when` guard semantics are specific to ampel 1.4.x. →
  Mitigation: Test fixtures capture the exact output format. Changes
  in ampel's output format would surface as test failures.

- **[Risk] `--allow-skip` is unconditional**: A policy that
  unexpectedly SKIPs (due to a bug) would be silently accepted. →
  Mitigation: The all-SKIP error detection catches the worst case
  (no policies matched at all). Individual unexpected SKIPs in a mixed
  result are surfaced as `provider.ResultSkipped` for the consumer to
  handle.
