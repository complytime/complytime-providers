## Context

The OPA provider's `Describe()` returns `RequiredTargetVariables: ["url", "input_path"]`.
The complyctl `doctor` command treats these as all-required, demanding both be present.
However, `scan` validates that exactly one of `url` or `input_path` is set (mutual
exclusion enforced by `validateTargetVariables()`).

complyctl PR #778 added `OptionalTargetVariableGroups` (field 7) to the
`DescribeResponse` proto. Each entry is a pipe-delimited group (e.g. `"url|input_path"`)
where doctor validates "at least one member present" instead of "all required."
The vendored complyctl at v1.0.0 already includes this field.

## Goals / Non-Goals

**Goals:**
- Move `url` and `input_path` out of `RequiredTargetVariables` and into
  `OptionalTargetVariableGroups` as `"url|input_path"`.
- Update the corresponding test to assert the new behavior.

**Non-Goals:**
- Updating `go.mod` or re-vendoring complyctl (already at the correct version).
- Changing mutual exclusivity enforcement in `validateTargetVariables()` (that
  stays at scan time, as designed).
- Modifying other providers (openscap, ampel).

## Decisions

1. **Single group entry `"url|input_path"`** rather than two separate groups.
   Both variables serve the same purpose (data source location), and the
   provider requires exactly one. A single pipe-delimited group expresses
   this "one-of" relationship directly.

2. **Empty `RequiredTargetVariables`** after removing `url` and `input_path`.
   The OPA provider has no variables that are unconditionally required for
   every target. The `opa_bundle_ref` is resolved through scan config from
   Generate, not required at Describe time.

## Risks / Trade-offs

- [Risk] Older complyctl versions that don't understand
  `OptionalTargetVariableGroups` will silently ignore the field.
  -> Mitigation: The proto field is additive; older clients simply won't
  validate the group, which is the same behavior as before this fix.
  Doctor on old complyctl will stop reporting false "missing" errors
  because `RequiredTargetVariables` will be empty.
