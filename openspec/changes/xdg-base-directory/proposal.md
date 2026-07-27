## Why

`complyctl` merged XDG Base Directory support in PR complytime/complyctl#736 (Jul 23, 2026), moving user-scoped paths from `~/.complytime/` to XDG-compliant locations (`~/.cache/complytime/`, `~/.local/share/complytime/`). This repository vendors complyctl path constants and must re-vendor, fix one inconsistent code path, and update all documentation to stay aligned before the next release. Implements the `complytime-providers` side of ADR-0016.

## What Changes

- Re-vendor `github.com/complytime/complyctl` to the post-#736 version — vendored `ResolveCacheDir()`, `ResolveDataDir()`, and `ResolveProviderDir()` update automatically
- **BREAKING**: Fix `cmd/openscap-provider/config/config.go` `expandPath()` to use `os.UserHomeDir()` instead of `user.Current().HomeDir` — these diverge under `sudo` and the latter requires CGO for NSS lookups
- Update `cmd/openscap-provider/config/config_test.go` to use `os.UserHomeDir()` for consistency with the production code change
- Update 7 documentation and script files replacing `~/.complytime/providers/` with `~/.local/share/complytime/providers/`, `~/.complytime/policies/`/`~/.complytime/complypacks/` with `~/.cache/complytime/` equivalents, and `~/.complytime/state.json` with `~/.local/share/complytime/state.json`
- Update `.devcontainer/scripts/post-create.sh` to install providers to the XDG data path
- Workspace-local `.complytime/` references remain unchanged (project-scoped, not user-scoped)

## Capabilities

### New Capabilities

- `xdg-provider-path-docs`: Documentation accurately reflects XDG-compliant user-scoped paths across all provider README files and guides

### Modified Capabilities

### Removed Capabilities

## Impact

- **Source code**: `cmd/openscap-provider/config/config.go` (`expandPath` function, lines 116-125), `cmd/openscap-provider/config/config_test.go` (test alignment)
- **Vendored dependency**: `vendor/github.com/complytime/complyctl/` (full re-vendor after version bump)
- **Scripts**: `.devcontainer/scripts/post-create.sh` (provider install path)
- **Documentation** (6 files):
  - `README.md`
  - `cmd/ampel-provider/README.md`
  - `cmd/opa-provider/README.md`
  - `cmd/openscap-provider/README.md`
  - `docs/provider-guide.md`
  - `docs/dev-testing-environment.md`
- **Packaging**: `complytime-providers.spec` — evaluate whether `Requires: complyctl >= 0.0.8` needs bumping
- **Unaffected**: Workspace-local `.complytime/` paths, `provider.WorkspaceDir` constant, `/usr/libexec/complytime/providers/` system path, `cmd/ampel-provider/docs/configuration.md` (workspace-local paths only), `cmd/openscap-provider/docs/configuration.md` (workspace-local paths only)
