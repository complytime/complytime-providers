## 1. Add Guidance to Finding and ParseAmpelOutput

- [x] 1.1 Add `Guidance string` field to the `Finding` struct in
  `cmd/ampel-provider/results/results.go`
- [x] 1.2 Extract `er.Error.Guidance` into `Finding.Guidance` in
  the `ParseAmpelOutput` function (fail branch, alongside
  `er.Error.Message`)
- [x] 1.3 Add unit tests for `ParseAmpelOutput` verifying `Guidance`
  extraction from the fail test fixture
  (`testdata/ampel-verify-fail.json`)
- [x] 1.4 Add unit test verifying `Guidance` is empty for passing
  tenet evaluations (`testdata/ampel-verify-pass.json`)
- [x] 1.5 Add unit test verifying `Guidance` is empty when a failing
  tenet has no `error.guidance` field (using inline
  `ampelResultStatement` construction matching the pattern in
  `TestParseAmpelOutput_ControlCharsStripped`)
- [x] 1.6 Add unit test verifying `Guidance` is sanitized with
  `stripControlChars()` (matching the `Reason` pattern) and rejected
  when exceeding `maxFieldSize` (matching the `checkID`/`description`
  pattern)

## 2. Actionable Message in ToScanResponse

- [x] 2.1 Replace the generic `fmt.Sprintf("%d of %d repositories
  passed", ...)` with message override logic: iterate steps to find
  the first non-passing step's `Message`; if all pass, use the first
  step's `Message`; fall back to `"X of Y checks passed"` only when
  all step messages are empty
- [x] 2.2 Extend the step construction in `ToScanResponse` to track
  guidance alongside each step (e.g., via a parallel slice or a local
  wrapper struct in the `reqGroup`)
- [x] 2.3 Ensure guidance is only sourced from non-passing steps
  (passing tenets do not carry guidance)
- [x] 2.4 Populate `AssessmentLog.Recommendation` from the first
  non-passing step's guidance (requires 2.2)
- [x] 2.5 Update `TestToScanResponse` to assert the `Message` is
  the first non-passing step's reason text instead of the count
  string
- [x] 2.6 Update `TestToScanResponse_MultipleChecks` to verify
  message override per requirement group
- [x] 2.7 Add test for all-passing case verifying `Message` uses
  the first step's assessment message
- [x] 2.8 Add test verifying `Recommendation` is populated from
  guidance on failure, empty when guidance is absent on failure, and
  empty on pass
- [x] 2.9 Add test for empty-message fallback: when all steps have
  empty `Message` fields, verify `AssessmentLog.Message` falls back
  to `"X of Y checks passed"` (validates vocabulary fix from
  "repositories" to "checks")

## 3. Verification

- [x] 3.1 Run `make test` and confirm all tests pass
- [x] 3.2 Run `make lint` and confirm zero lint issues
- [x] 3.3 Run `make build-ampel-provider` and confirm build succeeds
- [x] 3.4 Update `CHANGELOG.md` with entries for actionable scan
  messages and guidance recommendation propagation
<!-- spec-review: passed -->
<!-- code-review: passed -->
