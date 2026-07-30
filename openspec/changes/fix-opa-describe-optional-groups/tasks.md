## 1. Update Describe()

- [x] 1.1 In `cmd/opa-provider/server/server.go`, remove `url` and `input_path` from `RequiredTargetVariables` and add `OptionalTargetVariableGroups: []string{"url|input_path"}` to the `DescribeResponse`.

## 2. Update Tests

- [x] 2.1 In `cmd/opa-provider/server/server_test.go`, update `TestDescribe_Variables` to assert that `RequiredTargetVariables` does not contain `url` or `input_path`, and that `OptionalTargetVariableGroups` contains `"url|input_path"`.

## 3. Verify

- [x] 3.1 Run `make test` and confirm all tests pass.
- [x] 3.2 Run `make lint` and confirm no lint errors.
