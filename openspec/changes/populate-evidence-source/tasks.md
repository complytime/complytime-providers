# Tasks

## 1. Shared evidence utility

- [ ] 1.1 Create `internal/evidence/evidence.go` with a `FileDigest(path string) (string, error)` function that computes `sha256:<hex>` for a given file path. Verify with a unit test in `internal/evidence/evidence_test.go` that hashes a known temp file and asserts the expected digest string format and value.

## 2. OpenSCAP provider evidence

- [ ] 2.1 In `cmd/openscap-provider/server/server.go`, capture `time.Now().UTC().Format(time.RFC3339)` once before calling `runScanAndParseARF`, and compute `evidence.FileDigest(config.ARFPath)` after it returns. Verify the scan still builds successfully with `make build-openscap-provider`.
- [ ] 2.2 Update `buildAssessmentsFromARF` (or the Scan method) to attach a single `provider.Evidence` entry with `ID: "arf"`, `Type: "ARF"`, `Source.ReferenceID: "openscap-arf"`, `Source.Coordinate: config.ARFPath`, `Source.Digest: <computed>`, and `CollectedAt: <timestamp>` to each `AssessmentLog`. Verify with a test that asserts every returned `AssessmentLog` has exactly one `Evidence` entry with the expected field values.
- [ ] 2.3 Set `ScanResponse.MappingReferences` to a single `provider.MappingReference{ID: "openscap-arf", Title: "OpenSCAP ARF scan result", Description: "Asset Reporting Format result from OpenSCAP evaluation", URL: "file://" + config.ARFPath}`. Verify with a test that asserts the `ScanResponse` contains one `MappingReference` with `ID == "openscap-arf"`.
- [ ] 2.4 Run `go test ./cmd/openscap-provider/...` and confirm all existing and new tests pass.

## 3. Ampel provider evidence

- [ ] 3.1 Add `AmpelAttestationPath string` and `SnappyAttestationPath string` fields to `scan.RawScanResult` in `cmd/ampel-provider/scan/scan.go`. Populate them in `ScanRepository` from the existing local variables (`attestationFile` at line 285, `ampelResultFile` at line 303). Verify the build succeeds with `make build-ampel-provider`.
- [ ] 3.2 Add `AmpelAttestationPath string` and `SnappyAttestationPath string` fields to `results.PerRepoResult` in `cmd/ampel-provider/results/results.go`. Thread the values from `RawScanResult` through `ParseAmpelOutput` (or in `server.go` after `ParseAmpelOutput` returns). Verify with a test that `PerRepoResult` carries the expected paths after parsing.
- [ ] 3.3 Update `results.ToScanResponse` to build two `provider.Evidence` entries per assessment log: one for ampel attestation (`ID: "ampel-<prefix>"`, `Type: "IntotoAttestation"`, `Source.ReferenceID: "ampel-<prefix>"`, `Source.Coordinate: <ampelPath>`, `Source.Digest: <sha256>`) and one for snappy attestation (`ID: "snappy-<prefix>"`, same fields with the snappy path). Compute digests using `evidence.FileDigest`. Verify with a test that every `AssessmentLog` has two `Evidence` entries with correct prefixes and non-empty digests.
- [ ] 3.4 Build the `MappingReferences` slice on `ScanResponse` from the unique attestation paths across all repo results. Each pair of attestation files produces two `MappingReference` entries. Verify with a test that `ScanResponse.MappingReferences` contains the expected entries and that each `Evidence.Source.ReferenceID` matches a `MappingReference.ID`.
- [ ] 3.5 Run `go test ./cmd/ampel-provider/...` and confirm all existing and new tests pass.

## 4. OPA provider evidence

- [ ] 4.1 Add `InputPath string` field to `results.PerTargetResult` in `cmd/opa-provider/results/results.go`. Populate it from the `inputPath` variable in `evalAndParse` or `processTarget` in `server.go`. Verify the build succeeds with `make build-opa-provider`.
- [ ] 4.2 Update `results.ToScanResponse` to build one `provider.Evidence` entry per assessment log with `ID: "conftest-<target>[-<branch>]"`, `Type: "ConftestResult"`, `Source.ReferenceID: "conftest-input-<target>[-<branch>]"`, `Source.Coordinate: <inputPath>`, `Source.Digest: ""`, and `CollectedAt: <timestamp>`. Verify with a test that every `AssessmentLog` has one `Evidence` entry with the expected `Type` and non-empty `Coordinate`.
- [ ] 4.3 Build the `MappingReferences` slice on `ScanResponse` from the unique target/branch combinations. Each target produces one `MappingReference`. Verify with a test that each `Evidence.Source.ReferenceID` matches a `MappingReference.ID`.
- [ ] 4.4 Run `go test ./cmd/opa-provider/...` and confirm all existing and new tests pass.

## 5. Man page updates

- [ ] 5.1 Update `docs/man/complyctl-provider-openscap.md`: expand the `FILES` section to list `arf.xml`, `results.xml`, `tailoring.xml`, and the remediation scripts with their full workspace-relative paths. Add an `EVIDENCE` section (between `FILES` and `EXIT CODES`) describing the ARF file as evidence, how it is produced by `oscap xccdf eval --results-arf`, and how it appears in the EvaluationLog as an `Evidence` entry with `Source.Coordinate` and `Source.Digest`. Verify the markdown renders correctly with `pandoc -s -t man docs/man/complyctl-provider-openscap.md -o /dev/null`.
- [ ] 5.2 Update `docs/man/complyctl-provider-ampel.md`: expand the `FILES` section to list the attestation file naming pattern (`<repo>-<branch>-<spec>-{snappy,ampel}.intoto.json`), per-repo result JSONs, and `scan-config.json`. Add an `EVIDENCE` section describing both attestation types, that snappy collects raw API data and ampel evaluates policies against it, and how both appear as `Evidence` entries in the EvaluationLog. Verify with pandoc.
- [ ] 5.3 Update `docs/man/complyctl-provider-opa.md`: expand the `FILES` section to list per-target result JSONs, `scan-config.json`, `complytime-mapping.json`, the clone directory under `repos/`, and the `generated/` directory. Add an `EVIDENCE` section describing the conftest input path as evidence and its representation in the EvaluationLog. Verify with pandoc.
- [ ] 5.4 Regenerate all `.1` man pages with `make man` and verify no pandoc errors.

## 6. Integration verification

- [ ] 6.1 Run `make test` to confirm all unit tests pass across the entire project.
- [ ] 6.2 Run `make lint` to confirm zero lint issues.
- [ ] 6.3 Run `make build` to confirm all three provider binaries build successfully.

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result with `openspec validate`.
