## 1. Add Guidance to Finding and ParseAmpelOutput

- [ ] 1.1 Add `Guidance string` field to the `Finding` struct in
  `cmd/ampel-provider/results/results.go`
- [ ] 1.2 Extract `er.Error.Guidance` into `Finding.Guidance` in
  the `ParseAmpelOutput` function (fail branch, alongside
  `er.Error.Message`)
- [ ] 1.3 Add unit tests for `ParseAmpelOutput` verifying `Guidance`
  extraction from the fail test fixture
  (`testdata/ampel-verify-fail.json`)
- [ ] 1.4 Add unit test verifying `Guidance` is empty for passing
  tenet evaluations (`testdata/ampel-verify-pass.json`)

## 2. Actionable Message in ToScanResponse

- [ ] 2.1 Replace the generic `fmt.Sprintf("%d of %d repositories
  passed", ...)` with message override logic: iterate steps to find
  the first non-passing step's `Message`; if all pass, use the first
  step's `Message`; fall back to `"X of Y checks passed"` only when
  all step messages are empty
- [ ] 2.2 Populate `AssessmentLog.Recommendation` from the first
  non-passing step's guidance (new `Guidance` field on the step,
  carried via a local struct or by extending the reqGroup)
- [ ] 2.3 Update `TestToScanResponse` to assert the `Message` is
  the first non-passing step's reason text instead of the count
  string
- [ ] 2.4 Update `TestToScanResponse_MultipleChecks` to verify
  message override per requirement group
- [ ] 2.5 Add test for all-passing case verifying `Message` uses
  the first step's assessment message
- [ ] 2.6 Add test verifying `Recommendation` is populated from
  guidance on failure and empty on pass

## 3. Carry Guidance Through Steps

- [ ] 3.1 Extend the step construction in `ToScanResponse` to track
  guidance alongside each step (e.g., via a parallel slice or a local
  wrapper struct in the `reqGroup`)
- [ ] 3.2 Ensure guidance is only sourced from non-passing steps
  (passing tenets do not carry guidance)

## 4. Verification

- [ ] 4.1 Run `make test-unit` and confirm all tests pass
- [ ] 4.2 Run `make lint` and confirm zero lint issues
- [ ] 4.3 Run `make build-ampel-provider` and confirm build succeeds
