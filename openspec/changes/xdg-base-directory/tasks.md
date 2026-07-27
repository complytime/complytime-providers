## 1. Re-vendor complyctl

- [x] 1.1 Identify the complyctl version or commit hash that includes PR #736 XDG changes (check for tagged release first, fall back to commit hash)
- [x] 1.2 Run `go get github.com/complytime/complyctl@<version>` and `go mod tidy`
- [x] 1.3 Run `go mod vendor` to update vendored files
- [x] 1.4 Verify vendored `consts.go` contains `ResolveDataDir()` and updated `ResolveCacheDir()`/`ResolveProviderDir()`
- [x] 1.5 Run `make build` to confirm all three providers compile with the new vendor

## 2. Fix openscap-provider tilde expansion

- [x] 2.1 In `cmd/openscap-provider/config/config.go`, replace `user.Current().HomeDir` with `os.UserHomeDir()` in the `expandPath()` function
- [x] 2.2 Remove the `os/user` import from `config.go` (replaced by `os.UserHomeDir()` which is in the `os` package)
- [x] 2.3 Update the error message from "failed to identify current user" to "failed to determine home directory"
- [x] 2.4 In `cmd/openscap-provider/config/config_test.go`, replace `user.Current()` with `os.UserHomeDir()` for home directory resolution in `TestSanitizePath`, and remove the `os/user` import
- [x] 2.5 Run `make test` to verify config tests pass with the updated code and test

## 3. Update documentation and scripts

- [x] 3.1 Update `README.md` — replace `~/.complytime/providers` with `~/.local/share/complytime/providers`
- [x] 3.2 Update `cmd/ampel-provider/README.md` — replace `~/.complytime/providers` with `~/.local/share/complytime/providers`
- [x] 3.3 Update `cmd/opa-provider/README.md` — replace `~/.complytime/providers` with `~/.local/share/complytime/providers`
- [x] 3.4 Update `cmd/openscap-provider/README.md` — replace ALL user-scoped `~/.complytime/` references per the design path mapping table: `~/.complytime/providers` → `~/.local/share/complytime/providers`, `~/.complytime/policies` → `~/.cache/complytime/policies`, `~/.complytime/state.json` → `~/.local/share/complytime/state.json`
- [x] 3.5 Update `docs/provider-guide.md` — replace `~/.complytime/providers` with `~/.local/share/complytime/providers` and `~/.complytime/complypacks` with `~/.cache/complytime/complypacks`
- [x] 3.6 Update `docs/dev-testing-environment.md` — replace `~/.complytime/providers` with `~/.local/share/complytime/providers`
- [x] 3.7 Update `.devcontainer/scripts/post-create.sh` — replace `${HOME}/.complytime/providers` with `${HOME}/.local/share/complytime/providers` at lines 87 and 92

## 4. Packaging and changelog

- [x] 4.1 Evaluate whether `complytime-providers.spec` `Requires: complyctl >= 0.0.8` needs bumping to the minimum complyctl version with XDG support
- [x] 4.2 Update stale path references in Go source comments (`cmd/opa-provider/server/server_test.go:485` comment referencing `~/.complytime/complypacks/`)
- [x] 4.3 Add `CHANGELOG.md` entry documenting: BREAKING `expandPath()` change, documentation updated for XDG paths, re-vendored complyctl with XDG support

## 5. Verification

- [x] 5.1 Run `make build` — all three providers compile
- [x] 5.2 Run `make test` — all unit tests pass
- [x] 5.3 Run `make lint` — 0 issues
- [x] 5.4 Verify no remaining user-scoped `~/.complytime/` references in docs and scripts — grep for `~/.complytime/` across all `.md` files and `.sh` files, confirm all matches are workspace-local (no `~/` prefix) or intentional
- [x] 5.5 Verify workspace-local `.complytime/` references are preserved (grep check)
- [x] 5.6 Grep all `.go` files (excluding vendor/) for `~/.complytime/` references and update any stale comments

<!-- spec-review: passed -->
<!-- code-review: passed -->
