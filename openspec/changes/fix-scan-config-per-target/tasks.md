# Tasks

## 1. Shared content-key and index infrastructure

- [ ] 1.1 Create a shared `internal/genindex` package with `ContentKey(requirementIDs []string) string` that returns the first 16 hex characters of the SHA-256 of sorted, newline-joined requirement IDs. Verify with a unit test that identical inputs produce the same key and different inputs produce different keys.
- [ ] 1.2 Add `GenerateIndex` struct and `ReadIndex`/`WriteIndex` functions to `internal/genindex` that read and write `generate-index.json` with schema `{"version":1,"targets":{"<sanitized-url>":"<content-key>",...}}`. Verify with unit tests covering create-new, read-existing, update-entry, and malformed-file-error cases.
- [ ] 1.3 Add `UpdateIndex(indexDir, sanitizedURL, contentKey string) error` convenience function that performs the read-modify-write cycle. Include a source code comment noting that concurrent complyctl instances sharing the same workspace may race on this file and that file-level locking is a future consideration. Verify with a unit test.

## 2. OPA provider Generate-side changes

- [ ] 2.1 Add `GeneratedDirForKey(contentKey string) string` to `cmd/opa-provider/config/config.go` returning `<WorkspaceDir>/opa/generated/<contentKey>/`. Add `contentKey` directory to `EnsureDirectories` as needed. Verify the path is correct with a unit test.
- [ ] 2.2 Modify `server.go` `Generate` to compute the content key from matched IDs via `genindex.ContentKey`, write `scan-config.json` to `cfg.GeneratedDirForKey(contentKey)` instead of `cfg.GeneratedDirPath()`, and call `genindex.UpdateIndex` to map the target URL to the content key. Verify with an existing single-target test that still passes (`make test`).
- [ ] 2.3 Add a multi-target Generate test: call Generate twice with different `opa_bundle_ref` values (and thus different matched IDs), then verify that two distinct content-keyed directories exist under `generated/`, each with correct `scan-config.json` contents, and that `generate-index.json` has two entries. Run `make test`.

## 3. OPA provider Scan-side changes

- [ ] 3.1 Modify `server.go` `Scan` to read `generate-index.json` from `cfg.GeneratedDirPath()` at the start. Replace the single pre-loop `ReadScanConfig(cfg.GeneratedDirPath())` with a per-target read: for each target, look up its content key in the index using `targets.SanitizeURL(target.Variables["url"])`, then `ReadScanConfig(cfg.GeneratedDirForKey(contentKey))`. Verify existing single-target Scan tests still pass (`make test`).
- [ ] 3.2 Implement graceful fallback: when the index is missing or the target URL is not in the index, fall back to the existing no-scan-config behavior (pull via `opa_bundle_ref`, log a warning). Verify with a test that Scan works without a prior Generate call.
- [ ] 3.3 Update `resolveScanPolicyDir` to read `BundleDir` from the per-target scan-config instead of a shared scan-config. Verify with existing tests (`make test`).
- [ ] 3.4 Add a multi-target Scan integration test: set up two content-keyed directories with different scan-configs (different IDs, different BundleDirs), write a valid `generate-index.json`, then call Scan with two targets and verify each target's results reflect its own scan-config. Run `make test`.

## 4. Ampel provider Generate-side changes

- [ ] 4.1 Add a `generated/` directory to Ampel's workspace layout: add a `GeneratedDirPath() string` function to `cmd/ampel-provider/config/config.go` returning `<WorkspaceDir>/ampel/generated/` and a `GeneratedDirForKey(contentKey string) string` function. Add `generated/` to `EnsureDirectories`. Verify with a unit test.
- [ ] 4.2 Modify `server.go` `Generate` to compute the content key from matched requirement IDs, write both `complytime-ampel-policy.json` and `scan-config.json` to `config.GeneratedDirForKey(contentKey)` instead of `config.GeneratedPolicyDirPath()`, and call `genindex.UpdateIndex`. Verify with existing single-target Generate tests (`make test`).
- [ ] 4.3 Add a multi-target Generate test: call Generate twice with different requirement sets (simulating targets with different policy assignments), then verify that two distinct content-keyed directories exist under `generated/`, each with correct merged bundle and scan-config contents, and that `generate-index.json` has two entries. Run `make test`.

## 5. Ampel provider Scan-side changes

- [ ] 5.1 Modify `server.go` `Scan` to read `generate-index.json` from `config.GeneratedDirPath()`. Replace the shared `PolicyPath` in the pre-loop `scan.ScanConfig` with a per-target resolution: for each target, look up the content key in the index, then set `PolicyPath` to the merged bundle in the content-keyed directory. Move `ReadScanConfig` inside the target loop to read from the per-target content directory. Verify existing single-target tests pass (`make test`).
- [ ] 5.2 Implement graceful fallback: when the index is missing or the target URL is not found, fall back to reading from the legacy shared path (`config.ScanConfigDirPath()`) and log a warning. Verify with a test that Scan works without a prior Generate call.
- [ ] 5.3 Add a multi-target Scan integration test: set up two content-keyed directories with different merged bundles and scan-configs, write a valid `generate-index.json`, then call Scan with two targets and verify each target is scanned against its own policy bundle and requirement IDs. Run `make test`.

## 6. Cross-provider verification

- [ ] 6.1 Run `make lint` and fix any lint issues introduced by the changes.
- [ ] 6.2 Run `make build` and verify all three provider binaries compile successfully.
- [ ] 6.3 Run `make test` and verify all tests pass, including existing tests and new multi-target tests.

## Workflow follow-up

- Archive the change after the project's review requirements are satisfied.
- Verify the archived result.
