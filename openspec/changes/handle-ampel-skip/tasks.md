# Tasks

## 1. Add WhenClause type for policy serialization

- [ ] 1.1 Add `WhenClause` struct and `When *WhenClause` field to `AmpelPolicy` in `cmd/ampel-provider/convert/types.go`. Verify with `go build ./cmd/ampel-provider/...`.
- [ ] 1.2 Add test in `cmd/ampel-provider/convert/convert_test.go` that loads a policy JSON with a `"when"` field, merges it into a bundle, writes it, and verifies the `"when"` field is preserved in the output. Create a testdata policy fixture with a `when` guard for this purpose. Verify with `go test ./cmd/ampel-provider/convert/...`.

## 2. Handle SKIP status in ParseAmpelOutput

- [ ] 2.1 Add explicit `"SKIP"` case to the eval result status switch in `ParseAmpelOutput` (`cmd/ampel-provider/results/results.go:155`). Set `finding.Result = "skip"` and extract `assessment.message` into `finding.Reason`. Verify with `go build ./cmd/ampel-provider/...`.
- [ ] 2.2 Convert the overall status check (`results.go:180-185`) from binary if/else to a switch with `"PASS"`, `"SKIP"`, and default `"fail"` cases. Verify with `go build ./cmd/ampel-provider/...`.
- [ ] 2.3 Create testdata fixture `cmd/ampel-provider/results/testdata/ampel-verify-skip.json` containing an ampel ResultSet with mixed PASS and SKIP eval results (SKIP result has `id: "when"`, `status: "SKIP"`, and `assessment.message`). Add `TestParseAmpelOutput_Skip` test that parses it and verifies SKIP findings have `result: "skip"` with the correct reason. Verify with `go test ./cmd/ampel-provider/results/...`.
- [ ] 2.4 Create testdata fixture `cmd/ampel-provider/results/testdata/ampel-verify-all-skip.json` with all policies SKIPped and overall status `"SKIP"`. Add `TestParseAmpelOutput_AllSkip` test that verifies the overall status parses as `"skip"`. Verify with `go test ./cmd/ampel-provider/results/...`.

## 3. Pass --allow-skip to ampel verify

- [ ] 3.1 Add `"--allow-skip"` to the argument list in `constructAmpelVerifyCommand` (`cmd/ampel-provider/scan/scan.go:196`). Verify with `go build ./cmd/ampel-provider/...`.
- [ ] 3.2 Update `TestConstructAmpelVerifyCommand` in `cmd/ampel-provider/scan/scan_test.go` to assert `"--allow-skip"` is present in the returned argument list. Verify with `go test ./cmd/ampel-provider/scan/...`.

## 4. Detect all-SKIP as error in server.Scan

- [ ] 4.1 Add `AllFindingsSkipped() bool` method on `PerRepoResult` in `cmd/ampel-provider/results/results.go`. Returns `true` when `len(Findings) > 0` and every finding has `Result == "skip"`. Add `TestAllFindingsSkipped` in `results_test.go` with cases: empty findings (false), mixed pass+skip (false), all skip (true), all pass (false). Verify with `go test ./cmd/ampel-provider/results/...`.
- [ ] 4.2 In `cmd/ampel-provider/server/server.go`, after `ParseAmpelOutput` returns successfully (~line 248), add a check: if `parsed.AllFindingsSkipped()`, set `parsed.Status = "error"` and `parsed.Error` to a descriptive message. Verify with `go build ./cmd/ampel-provider/...`.
- [ ] 4.3 Add test in `cmd/ampel-provider/server/server_test.go` that verifies the all-SKIP-to-error conversion by mocking ampel output where all policies produce SKIP. Verify with `go test ./cmd/ampel-provider/server/...`.

## 5. Integration verification

- [ ] 5.1 Run `make test` to confirm all existing and new tests pass with zero failures. Run `make lint` to confirm no lint issues.

## Workflow follow-up

- Update issue [#200](https://github.com/complytime/complytime-providers/issues/200) comment noting that the correct `when` expression is `predicates.exists(p, p.predicate_type == '<type-url>')`, not `size(predicates) > 0` as originally described.
- In org-infra (separate PR): add `when` guards to the 5 branch-protection policies in `compliance/ampel/branch-protection/` using the predicate-type-specific expression.
- Archive the change after the project's review requirements are satisfied.
