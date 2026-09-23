## Why

The OPA provider's `Describe()` currently declares both `url` and `input_path` in
`RequiredTargetVariables`. This causes `complyctl doctor` to demand both variables,
but `complyctl scan` rejects configs where both are present
(`"specify either url or input_path, not both"`). Users cannot satisfy both commands
simultaneously. The upstream fix (complyctl#759 / PR #778) added
`OptionalTargetVariableGroups` to the `DescribeResponse` proto and is now merged.
This provider-side change completes the fix.

## What Changes

- Move `url` and `input_path` from `RequiredTargetVariables` to
  `OptionalTargetVariableGroups` as `"url|input_path"` in `Describe()`.
- Update `TestDescribe_Variables` to assert the new field instead of
  `RequiredTargetVariables`.

## Capabilities

### New Capabilities

### Modified Capabilities

## Impact

- `cmd/opa-provider/server/server.go`: `Describe()` return value changes.
- `cmd/opa-provider/server/server_test.go`: `TestDescribe_Variables` assertions change.
- No dependency update needed: vendored complyctl already includes
  `OptionalTargetVariableGroups` (v1.0.0).
- No breaking changes to external consumers: the proto field is additive.
