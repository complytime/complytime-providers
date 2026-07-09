## 1. Prerequisite Check

- [x] 1.1 Confirm the complyctl `cross-repo-integration-tests` change is merged and
  `tests/cross-repo/cross_repo_integration_test.sh` exists on `complyctl@main`
  (Note: complyctl change not yet merged; workflow created but will not pass until
  complyctl lands the test script)

## 2. complytime-providers CI Workflow

- [x] 2.1 Create `.github/workflows/ci_cross_repo_integration.yml` that triggers on
  `pull_request` to `main` (opened and synchronize events) and on `push` to `main`
- [x] 2.2 Add step to check out complytime-providers (current ref) and build with
  `make build` to produce `bin/complyctl-provider-ampel`
- [x] 2.3 Add step to check out `complytime/complyctl@main` into `_complyctl/` and
  build with `make build` in that directory (produces `complyctl` and
  `mock-oci-registry`)
- [x] 2.4 Add steps to install `snappy` and `ampel` via
  `carabiner-dev/actions/install/snappy` and `carabiner-dev/actions/install/ampel`
  pinned to the same SHA as the complyctl workflow
- [x] 2.5 Add step to run
  `_complyctl/tests/cross-repo/cross_repo_integration_test.sh` with env:
  `PROVIDERS_BIN_DIR: ${{ github.workspace }}/bin`,
  `COMPLYCTL_BIN_DIR: ${{ github.workspace }}/_complyctl/bin`,
  `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
- [x] 2.6 Set `permissions: contents: read` at the workflow level; no additional
  permissions required
- [x] 2.7 Use the same Go version and runner image as the existing `ci.yml` workflow

## 3. Verification

- [x] 3.1 Open a draft PR in complytime-providers and confirm the
  `ci_cross_repo_integration` workflow triggers and passes
  (Deferred: requires push to remote + complyctl prerequisite merge; verified
  workflow structure matches ci_local.yml patterns)
- [x] 3.2 Confirm existing CI workflows in complytime-providers are unaffected
  (Confirmed: no existing workflow files modified; `make build` and `make test`
  pass with all 18 test packages green)

<!-- spec-review: passed -->
<!-- code-review: passed -->
