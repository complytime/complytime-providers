## ADDED Requirements

### Requirement: AMPEL provider declares export support
The AMPEL provider's `Describe` RPC response SHALL set `SupportsExport: true` so that complyctl includes it in the export phase when `--format otel` is used.

#### Scenario: Describe reports export capability
- **GIVEN** the AMPEL provider is running
- **WHEN** complyctl calls `Describe` on the AMPEL provider
- **THEN** the response includes `SupportsExport: true` alongside the existing health, version, and required variable fields

### Requirement: AMPEL provider implements Export RPC
The AMPEL provider `ProviderServer` SHALL implement the `provider.Exporter` interface by providing an `Export(ctx, *ExportRequest) (*ExportResponse, error)` method. The method SHALL read AMPEL scan results from the workspace results directory, convert each assessment to a `GemaraEvidence` object, and emit them as OTLP log records via ProofWatch to the collector endpoint specified in the `ExportRequest.Collector` config.

#### Scenario: Successful export of AMPEL scan results
- **GIVEN** a successful scan has produced result files in `.complytime/ampel/results/`
- **WHEN** complyctl calls `Export` with a valid `CollectorConfig` (endpoint and auth token)
- **THEN** the AMPEL provider reads per-repo result files, converts each assessment finding to a `GemaraEvidence` with the correct Gemara attribute mappings, emits them via ProofWatch, and returns an `ExportResponse` with `Success: true` and `ExportedCount` equal to the number of evidence records emitted

#### Scenario: Export with no scan results
- **GIVEN** no scan result files exist in the results directory
- **WHEN** complyctl calls `Export` with a valid `CollectorConfig`
- **THEN** the provider returns `ExportResponse` with `Success: true`, `ExportedCount: 0`, and `FailedCount: 0`

#### Scenario: Export with collector connection failure
- **GIVEN** a successful scan has produced result files
- **WHEN** complyctl calls `Export` with a `CollectorConfig` pointing to an unreachable endpoint
- **THEN** the provider returns `ExportResponse` with `Success: false` and `ErrorMessage` describing the connection failure

#### Scenario: Export with malformed result files
- **GIVEN** a result file exists in the results directory but contains invalid JSON
- **WHEN** complyctl calls `Export` with a valid `CollectorConfig`
- **THEN** the provider returns `ExportResponse` with `Success: false` and `ErrorMessage` describing the parse failure

#### Scenario: Partial export failure
- **GIVEN** a successful scan has produced multiple result files
- **WHEN** some evidence records fail to emit via ProofWatch (e.g., collector becomes unavailable mid-export)
- **THEN** the provider returns `ExportResponse` with `Success: false`, `ExportedCount` reflecting successfully emitted records, `FailedCount` reflecting failed records, and `ErrorMessage` describing the failure count

### Requirement: AMPEL evidence carries correct Gemara attributes
Each `GemaraEvidence` record emitted by the AMPEL provider SHALL carry the following attributes mapped from the scan results:

- `policy.engine.name` SHALL be set to `"ampel"` via `Metadata.Author.Name`
- `compliance.control.id` SHALL be set to the requirement ID (derived from `TenetID` with `check-` prefix stripped) via `AssessmentLog.Requirement.EntryId`
- `policy.rule.id` SHALL be set to the same requirement ID via `AssessmentLog.Plan.EntryId` (AMPEL findings do not distinguish a separate policy ID from the requirement ID)
- `policy.evaluation.result` SHALL map finding result `"pass"` to `gemara.Passed`, `"fail"` to `gemara.Failed`, and any other value to `gemara.Unknown`, via `AssessmentLog.Result`
- `compliance.assessment.id` SHALL be a deterministic composite identifier per evidence record (format: `ampel-<reqID>-<repo>-<branch>`) via `Metadata.Id`, enabling idempotent re-export
- `policy.evaluation.message` SHALL include the step message from the finding when present, via `AssessmentLog.Message`

Note: The OTEL attribute names above (e.g., `policy.engine.name`) are the wire-format attributes produced by ProofWatch internally. The provider sets the Gemara struct fields; ProofWatch handles the OTEL mapping.

#### Scenario: Passed finding maps to correct attributes
- **GIVEN** an AMPEL scan has produced a finding with result `"pass"` for tenet `"check-branch-protection-01"` in repo `"myorg/myrepo"` on branch `"main"`
- **WHEN** the finding is converted to `GemaraEvidence`
- **THEN** the evidence has `Metadata.Author.Name = "ampel"`, `AssessmentLog.Requirement.EntryId = "branch-protection-01"`, `AssessmentLog.Plan.EntryId = "branch-protection-01"`, `AssessmentLog.Result = gemara.Passed`, and `Metadata.Id = "ampel-branch-protection-01-myorg/myrepo-main"`

#### Scenario: Failed finding maps to correct attributes
- **GIVEN** an AMPEL scan has produced a finding with result `"fail"` for tenet `"check-approval-rules-01"` with a step message
- **WHEN** the finding is converted to `GemaraEvidence`
- **THEN** the evidence has `AssessmentLog.Result = gemara.Failed` and `AssessmentLog.Message` includes the step message
