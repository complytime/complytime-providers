# Tasks

## 1. Shared evidence utility

- [x] 1.1 Create `internal/evidence/evidence.go` with a `FileDigest(path string) (string, error)` function that computes `sha256:<hex>` for a given file path. Define evidence type and ID prefix constants in this package for use by all providers. Verify with unit tests in `internal/evidence/evidence_test.go` including: (a) hashing a known temp file and asserting the expected digest string format and value, (b) calling with a non-existent path and asserting an error is returned, (c) calling with a directory path and asserting an error is returned.

## 2. OpenSCAP provider evidence

- [x] 2.1 In `cmd/openscap-provider/server/server.go`, capture `time.Now().UTC().Format(time.RFC3339)` once before calling `runScanAndParseARF`, and compute `evidence.FileDigest(config.ARFPath)` after it returns. Verify the scan still builds successfully with `make build-openscap-provider`.
- [x] 2.2 In the `Scan` method (server.go), after `buildAssessmentsFromARF` returns, attach a single `provider.Evidence` entry to each `AssessmentLog` with `ID: "arf"`, `Type: "ARF"`, `Source.ReferenceID: "openscap-arf"`, `Source.Coordinate: config.ARFPath`, `Source.Digest: <computed>`, and `CollectedAt: <timestamp>`. Alternatively, add evidence parameters to `buildAssessmentsFromARF` to keep it testable with the existing XML fixture pattern. Verify with a test that asserts every returned `AssessmentLog` has exactly one `Evidence` entry with the expected field values.
- [x] 2.3 Set `ScanResponse.MappingReferences` to a single `provider.MappingReference{ID: "openscap-arf", Title: "OpenSCAP ARF scan result", Description: "Asset Reporting Format result from OpenSCAP evaluation", URL: "file://" + config.ARFPath}`. Verify with a test that asserts the `ScanResponse` contains one `MappingReference` with `ID == "openscap-arf"`.
- [x] 2.4 Run `go test ./cmd/openscap-provider/...` and confirm all existing and new tests pass.

## 3. Ampel provider evidence

- [x] 3.1 Add `AmpelAttestationPath string` and `SnappyAttestationPath string` fields to `scan.RawScanResult` in `cmd/ampel-provider/scan/scan.go`. Populate them in `ScanRepository` from the existing local variables (`attestationFile` at line 285, `ampelResultFile` at line 303). Verify the build succeeds with `make build-ampel-provider`.
- [x] 3.2 Add `AmpelAttestationPath string` and `SnappyAttestationPath string` fields to `results.PerRepoResult` in `cmd/ampel-provider/results/results.go`. Thread the values from `RawScanResult` through `ParseAmpelOutput` (or in `server.go` after `ParseAmpelOutput` returns). Verify with a test that `PerRepoResult` carries the expected paths after parsing.
- [x] 3.3 Update `results.ToScanResponse` to build two `provider.Evidence` entries per assessment log: one for ampel attestation (`ID: "ampel-<prefix>"`, `Type: "IntotoAttestation"`, `Source.ReferenceID: "ampel-<prefix>"`, `Source.Coordinate: <ampelPath>`, `Source.Digest: <sha256>`) and one for snappy attestation (`ID: "snappy-<prefix>"`, same fields with the snappy path). Compute digests using `evidence.FileDigest`. Verify with a test that every `AssessmentLog` has two `Evidence` entries with correct prefixes and non-empty digests.
- [x] 3.4 Build the `MappingReferences` slice on `ScanResponse` from the unique attestation paths across all repo results. Each pair of attestation files produces two `MappingReference` entries. Verify with a test that `ScanResponse.MappingReferences` contains the expected entries and that each `Evidence.Source.ReferenceID` matches a `MappingReference.ID`.
- [x] 3.5 Run `go test ./cmd/ampel-provider/...` and confirm all existing and new tests pass.

## 4. OPA provider evidence

- [x] 4.1 Add `InputPath string` field to `results.PerTargetResult` in `cmd/opa-provider/results/results.go`. Populate it from the `inputPath` variable in `evalAndParse` or `processTarget` in `server.go`. Verify the build succeeds with `make build-opa-provider`.
- [x] 4.2 Update `results.ToScanResponse` to build one `provider.Evidence` entry per assessment log with `ID: "conftest-<target>[-<branch>]"`, `Type: "ConftestResult"`, `Source.ReferenceID: "conftest-input-<target>[-<branch>]"`, `Source.Coordinate: <inputPath>`, `Source.Digest: ""`, and `CollectedAt: <timestamp>`. Verify with a test that every `AssessmentLog` has one `Evidence` entry with the expected `Type` and non-empty `Coordinate`.
- [x] 4.3 Build the `MappingReferences` slice on `ScanResponse` from the unique target/branch combinations. Each target produces one `MappingReference`. Verify with a test that each `Evidence.Source.ReferenceID` matches a `MappingReference.ID`.
- [x] 4.4 Run `go test ./cmd/opa-provider/...` and confirm all existing and new tests pass.

## 5. Man page updates

- [x] 5.1 Update `docs/man/complyctl-provider-openscap.md`: expand the `FILES` section to list `arf.xml`, `results.xml`, `tailoring.xml`, and the remediation scripts with their full workspace-relative paths. Add an `EVIDENCE` section (between `FILES` and `EXIT CODES`) describing the ARF file as evidence, how it is produced by `oscap xccdf eval --results-arf`, and how it appears in the EvaluationLog as an `Evidence` entry with `Source.Coordinate` and `Source.Digest`. Verify the markdown renders correctly with `pandoc -s -t man docs/man/complyctl-provider-openscap.md -o /dev/null`.
- [x] 5.2 Update `docs/man/complyctl-provider-ampel.md`: expand the `FILES` section to list the attestation file naming pattern (`<repo>-<branch>-<spec>-{snappy,ampel}.intoto.json`), per-repo result JSONs, and `scan-config.json`. Add an `EVIDENCE` section describing both attestation types, that snappy collects raw API data and ampel evaluates policies against it, and how both appear as `Evidence` entries in the EvaluationLog. Verify with pandoc.
- [x] 5.3 Update `docs/man/complyctl-provider-opa.md`: expand the `FILES` section to list per-target result JSONs, `scan-config.json`, `complytime-mapping.json`, the clone directory under `repos/`, and the `generated/` directory. Add an `EVIDENCE` section describing the conftest input path as evidence and its representation in the EvaluationLog. Verify with pandoc.
- [x] 5.4 Regenerate all `.1` man pages with `make man` and verify no pandoc errors.

## 6. Integration verification

- [x] 6.1 Run `make test` to confirm all unit tests pass across the entire project.
- [x] 6.2 Run `make lint` to confirm zero lint issues.
- [x] 6.3 Run `make build` to confirm all three provider binaries build successfully.
- [x] 6.4 Update `AGENTS.md` project structure to include `internal/evidence/` with a description of the shared evidence utility package.
- [x] 6.5 Add a `CHANGELOG.md` entry documenting the evidence source population feature for all three providers.
- [x] 6.6 Update `README.md` project structure paragraph to mention `internal/evidence/` alongside `archive/` and `version/`.

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result with `openspec validate`.

<!-- spec-review: passed -->
<!-- code-review: passed -->
