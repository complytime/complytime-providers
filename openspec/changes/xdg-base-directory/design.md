## Context

`complyctl` PR #736 (merged Jul 23, 2026) adopted XDG Base Directory paths for all user-scoped directories. The vendored `consts.go` in this repository still contains the pre-XDG implementations of `ResolveCacheDir()` and `ResolveProviderDir()`, and a new `ResolveDataDir()` function was added upstream. Additionally, `cmd/openscap-provider/config/config.go` uses `user.Current().HomeDir` for tilde expansion, which diverges from `os.UserHomeDir()` under `sudo` or in containers where NSS is unavailable.

Path mapping from complyctl #736:

| Data | Old path | New path | XDG category |
|---|---|---|---|
| Policies/complypacks | `~/.complytime/{policies,complypacks}/` | `~/.cache/complytime/...` | Cache |
| Providers | `~/.complytime/providers/` | `~/.local/share/complytime/providers/` | Data |
| state.json | `~/.complytime/state.json` | `~/.local/share/complytime/state.json` | Data |

Workspace-local `.complytime/` (project-scoped) and `/usr/libexec/complytime/providers/` (FHS system path) are unaffected.

## Goals / Non-Goals

### Goals

- Re-vendor complyctl so path resolution functions reflect XDG-compliant locations
- Fix the inconsistent tilde expansion in openscap-provider to use `os.UserHomeDir()`
- Update all documentation and scripts to reference the new XDG paths
- Ensure all existing tests pass after changes and test code aligns with production code

### Non-Goals

- Implementing migration logic or legacy directory warnings in providers (this lives in complyctl)
- Modifying workspace-local `.complytime/` paths (project-scoped, not subject to XDG)
- Changing the `/usr/libexec/complytime/providers/` system path
- Modifying any provider's config.go files that use `provider.WorkspaceDir` for workspace-relative paths
- Adding new XDG env var overrides (e.g., `COMPLYCTL_CACHE_DIR`) — that belongs in complyctl

## Decisions

### D1: Re-vendor complyctl rather than patch locally

**Decision**: Run `go get github.com/complytime/complyctl@<post-736-version>` and `go mod vendor` to pick up all XDG changes automatically.

**Rationale**: The path resolution functions (`ResolveCacheDir`, `ResolveDataDir`, `ResolveProviderDir`) live in complyctl. Re-vendoring is the idiomatic Go approach — no local patches to maintain.

**Alternative considered**: Cherry-picking specific files from complyctl. Rejected because it creates maintenance burden and risks missing related changes.

### D2: Replace `user.Current()` with `os.UserHomeDir()` in expandPath

**Decision**: Replace the `user.Current().HomeDir` call in `cmd/openscap-provider/config/config.go:expandPath()` with `os.UserHomeDir()`. Also update `config_test.go` to use `os.UserHomeDir()` for computing expected values.

**Rationale**: `os.UserHomeDir()` reads `$HOME` directly on Unix and does not require CGO or NSS lookups. This aligns with how complyctl resolves paths and avoids divergence under `sudo` where `user.Current()` returns root's info but `$HOME` may still point to the invoking user. The test must use the same API to avoid false passes when the two functions diverge.

**Alternative considered**: Keeping `user.Current()` and adding a `$HOME` fallback. Rejected — `os.UserHomeDir()` handles this correctly already and is simpler.

### D3: Documentation and script path replacement strategy

**Decision**: Replace all user-scoped `~/.complytime/` references with XDG equivalents in 6 documentation files and 1 script file:
- `~/.complytime/providers` → `~/.local/share/complytime/providers`
- `~/.complytime/policies` / `~/.complytime/complypacks` → `~/.cache/complytime/policies` / `~/.cache/complytime/complypacks`
- `~/.complytime/state.json` → `~/.local/share/complytime/state.json`

Leave workspace-local `.complytime/` references as-is. The two configuration docs (`cmd/ampel-provider/docs/configuration.md`, `cmd/openscap-provider/docs/configuration.md`) contain only workspace-local paths and require no changes.

**Rationale**: Documentation and scripts must match runtime behavior. The distinction between user-scoped paths (XDG) and workspace-local paths (`.complytime/`) is critical for users to understand.

## Risks / Trade-offs

- **Upstream version availability** — The re-vendor depends on complyctl publishing a version or tag that includes PR #736. If only a commit hash is available, we use `go get ...@<commit-hash>` which is less clean but functionally equivalent. Mitigation: check for a tagged release first, fall back to commit hash. Document the target version or commit hash in the PR description.
- **RPM packaging constraint** — The `complytime-providers.spec` has `Requires: complyctl >= 0.0.8`. If the XDG changes require a minimum complyctl version higher than 0.0.8, this constraint must be bumped. Mitigation: evaluate during implementation and update if needed.
- **Breaking change for users with muscle memory** — Users who have scripts or aliases referencing `~/.complytime/providers/` will need to update. Mitigation: complyctl #736 includes `CheckLegacyDir()` which prints a deprecation warning. Our documentation changes provide the updated paths.
- **Test code alignment** — `config_test.go` uses `user.Current()` which must be updated to `os.UserHomeDir()` alongside the production code. The test currently passes by coincidence in most environments, but diverges under sudo or in containers without NSS.
- **Cross-spec impact** — The sibling OpenSpec change `dev-testing-environment` has spec artifacts referencing `~/.complytime/providers/`. After this change lands, those spec artifacts will contain stale path references. They will be corrected when that change is next modified or archived.
- **Deployment coordination** — This change SHOULD ship after or simultaneously with a complyctl release containing #736. Rollback is a simple `git revert` since changes are additive (vendor bump + code fix + docs). The RPM `Requires` constraint is the mechanism that prevents partial deployment in the RPM channel.
