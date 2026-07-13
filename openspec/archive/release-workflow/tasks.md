## 1. Version Infrastructure

- [x] 1.1 Create `internal/version/version.go` with lean version package (strips `v` prefix, defaults to `0.0.0-unknown`)
- [x] 1.2 Create `internal/version/version_test.go` with tests for `Version()` (empty, with prefix, without prefix)
- [x] 1.3 Update `cmd/openscap-provider/server/server.go` to use `version.Version()`
- [x] 1.4 Update `cmd/ampel-provider/server/server.go` to use `version.Version()`
- [x] 1.5 Update `cmd/opa-provider/server/server.go` to use `version.Version()`
- [x] 1.6 Update `cmd/openscap-provider/server/server_test.go` to assert `version.Version()` default (`0.0.0-unknown`)
- [x] 1.7 Update `cmd/ampel-provider/server/server_test.go` to assert `version.Version()` default (`0.0.0-unknown`)
- [x] 1.8 Update `cmd/opa-provider/server/server_test.go` to assert `version.Version()` default (`0.0.0-unknown`)
- [x] 1.9 Update `Makefile` to add ldflags with version injection (strip `v` prefix from git tag)
- [x] 1.10 Run `make build && make test && make lint` to verify version infrastructure

## 2. GoReleaser Configuration

- [x] 2.1 Create `.goreleaser.yaml` with 3 builds, 3 archives, changelog, SBOMs (SPDX JSON), and cosign signing
- [x] 2.2 Run `goreleaser check` to validate the configuration
- [x] 2.3 Run `goreleaser release --snapshot --clean` to verify archive and SBOM generation (cosign signing skipped locally)

## 3. Release Workflow

- [x] 3.1 Create `.github/workflows/release.yml` with `workflow_dispatch` trigger, preflight job (with re-run safe tag check), and release job (per-job least-privilege permissions, actions pinned by SHA)
- [x] 3.2 Simplify `.github/workflows/ci_local.yml`: remove release job and `tags: [v*]` trigger

## 4. RPM and Packaging Updates

- [x] 4.1 Update `complytime-providers.spec`: add OPA sub-package, add ldflags to `%build`, bump version to `0.1.0`
- [x] 4.2 Update `.packit.yaml`: replace `f42` with `f44` in all job sections (copr_build, tests, propose_downstream, koji_build, bodhi_update)
- [x] 4.3 Update `plans/test-RPM-providers.fmf`: add OPA provider binary validation

## 5. Documentation Updates

- [x] 5.1 Update `AGENTS.md`: add `internal/version/` to Project Structure, add release workflow to CI Workflow Structure table, update architecture narrative about shared library code, fix `ci.yml` reference to `ci_local.yml`
- [x] 5.2 Update `CHANGELOG.md`: add entries for release workflow, version injection, OPA RPM sub-package

## 6. Verification

- [x] 6.1 Run `make build && make test && make lint` for final verification
- [x] 6.2 Verify `go mod vendor` produces no changes (no new dependencies)
<!-- spec-review: passed -->
<!-- code-review: passed -->
