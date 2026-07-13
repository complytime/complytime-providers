# Capability: opa-generate

Implement the Generate RPC for the OPA provider to filter OPA policy bundles
to only the requirements specified in the Gemara assessment plan, following
the mapping pattern established by the AMPEL provider.

## Requirements

### Generate RPC

- **REQ-GEN-001**: Generate MUST read `req.Configuration` and extract
  `RequirementID` values from the Gemara assessment plan.
- **REQ-GEN-002**: Generate MUST pull the OCI policy bundle using
  `conftest pull` before reading the mapping file.
- **REQ-GEN-003**: Generate MUST look for `complytime-mapping.json` in the
  root of the pulled bundle directory.
- **REQ-GEN-004**: When a mapping file is present, Generate MUST match each
  `RequirementID` from the assessment plan against the mapping's
  `requirement_id` entries using exact string equality.
- **REQ-GEN-005**: Unmatched RequirementIDs MUST produce a warning log but
  MUST NOT cause Generate to fail.
- **REQ-GEN-006**: Generate MUST write a `scan-config.json` artifact to
  `.complytime/opa/generated/` containing the matched ID list (Rego
  namespaces) and reverse mapping.
- **REQ-GEN-007**: When no mapping file is found, Generate MUST log a
  warning and write a `scan-config.json` with `ids: null` and
  `reverse_mapping: null`.
- **REQ-GEN-008**: Generate MUST return `{Success: false}` with an error
  message when `req.Configuration` is empty.
- **REQ-GEN-009**: Generate MUST check tool availability (conftest, git)
  before proceeding and return `{Success: false}` if tools are missing.
- **REQ-GEN-010**: Generate MUST return `{Success: false}` when
  `opa_bundle_ref` is not provided in global or target variables.

### Scan Integration

- **REQ-SCAN-001**: When `scan-config.json` exists with a non-null
  `ids` list, Scan MUST pass each ID as a `--namespace` flag to conftest
  instead of `--all-namespaces`.
- **REQ-SCAN-002**: When `scan-config.json` is absent or has
  `ids: null`, Scan MUST use `--all-namespaces` (current behavior).
- **REQ-SCAN-003**: When a reverse mapping is available, `ToScanResponse`
  MUST replace Rego-derived RequirementIDs with Gemara RequirementIDs in
  `AssessmentLog.RequirementID`.
- **REQ-SCAN-004**: When no reverse mapping is available, `ToScanResponse`
  MUST use `deriveIDFromQuery` as today.

### Mapping File Format

- **REQ-MAP-001**: The mapping file format MUST be JSON with a `version`
  field and a `mappings` array of `{id, requirement_id}` objects. The
  `id` field is the Rego package namespace (semantic policy identity);
  `requirement_id` maps to the Gemara assessment plan.
- **REQ-MAP-002**: The mapping file MUST be validated: reject empty
  `id` or `requirement_id` values, reject duplicate `id` entries,
  reject duplicate `requirement_id` entries.
- **REQ-MAP-003**: The mapping file name MUST be `complytime-mapping.json`.

### Backward Compatibility

- **REQ-COMPAT-001**: Existing OCI bundles without `complytime-mapping.json`
  MUST produce identical scan behavior to the current implementation.
- **REQ-COMPAT-002**: The `ToScanResponse` function signature change MUST
  not break existing callers -- the reverse mapping parameter should have a
  zero-value that preserves current behavior.
