## MODIFIED Requirements

### Requirement: Generate returns error when mapping file is missing
When `complytime-mapping.json` is not found in the OCI policy bundle,
Generate MUST return `{Success: false, ErrorMessage: "..."}` with a
message identifying the expected file name and its expected location
within the bundle.

#### Scenario: Missing mapping file
- **WHEN** Generate pulls the OCI bundle and `complytime-mapping.json`
  does not exist in the bundle root directory
- **THEN** Generate MUST return `{Success: false}` with an error message
  that includes the file name `complytime-mapping.json` and indicates the
  file must be placed in the OCI policy bundle

#### Scenario: Malformed mapping file
- **WHEN** Generate pulls the OCI bundle and `complytime-mapping.json`
  exists but contains invalid JSON, empty fields, or duplicate entries
- **THEN** Generate MUST return `{Success: false}` with the validation
  error from `LoadMapping`, distinct from the missing-file error

### Requirement: Scan rejects nil IDs as configuration error
When `scanCfg.IDs` is nil, Scan MUST treat it as a configuration error
rather than a signal to evaluate all namespaces. The `--all-namespaces`
fallback path MUST be removed.

#### Scenario: Scan config with nil IDs
- **WHEN** Scan reads a `scan-config.json` with `ids: null`
- **THEN** Scan MUST return an error or skip evaluation for that target
  rather than falling back to `--all-namespaces`

#### Scenario: Scan config with valid IDs
- **WHEN** Scan reads a `scan-config.json` with a non-empty `ids` array
- **THEN** Scan MUST pass each ID as a `--namespace` flag to conftest
  (existing behavior, unchanged)

## REMOVED Requirements

### Requirement: Generate fallback on missing mapping (REQ-GEN-007)
**Reason**: Replaced by error response. Fallback mode produces
Rego-derived requirement IDs that complyctl cannot correlate with
the Gemara assessment plan, making results semantically unreliable.
**Migration**: OCI policy bundles MUST include a `complytime-mapping.json`
file. See the proposal for the mapping file format (REQ-MAP-001 through
REQ-MAP-003, unchanged).

### Requirement: Backward compatibility with unmapped bundles (REQ-COMPAT-001)
**Reason**: The OPA provider is not yet in production. Requiring the
mapping file before adoption is cheaper than removing the fallback
after it becomes an implicit contract. All three providers (OpenSCAP,
AMPEL, OPA) now consistently require their mapping/tailoring mechanisms.
**Migration**: Add `complytime-mapping.json` to OCI policy bundles
before using them with the OPA provider.
