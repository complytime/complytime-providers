## Why

Maintainers reviewing provider PRs need to test CLI UX changes by running
`complyctl` commands with the provider binaries from the PR branch. The
complyctl repository has a devcontainer-based testing environment (merged
to main with follow-up improvements from multiple maintainers). This
change adds the corresponding devcontainer to complytime-providers so
maintainers can test provider PRs with the same one-command workflow.

## What Changes

- Add a `.devcontainer/` configuration mirroring complyctl's devcontainer
  with the build order reversed: providers from local source, complyctl
  from main.
- The post-create script mirrors complyctl's operational patterns
  (GITHUB_TOKEN handling, pinned tool versions, nohup/disown for the
  mock registry, auto-rebuild hook, private bundle registration).
  See complyctl's `openspec/changes/dev-testing-environment/design.md`
  (D4-D10) for rationale.
- Add documentation in `docs/` referencing complyctl's docs for shared
  concepts and a `test-devcontainer` CI smoke test target.

## Capabilities

### New Capabilities

- `dev-testing-environment`: Devcontainer configuration and setup automation
  providing a one-command Fedora environment for interactive provider
  testing during PR reviews.

### Modified Capabilities

(none)

## Impact

- **New files**: `.devcontainer/Containerfile`, `.devcontainer/devcontainer.json`,
  `.devcontainer/scripts/post-create.sh`, `docs/dev-testing-environment.md`
- **Modified files**: `README.md` (link to docs), `Makefile` (add
  `test-devcontainer` target), `AGENTS.md` (update project structure)
- **Dependencies**: Same as complyctl's devcontainer (Fedora 43, system
  packages, snappy v0.2.4, ampel v1.2.1, conftest v0.68.2). Clones
  `complyctl` from GitHub at post-create time.
- **No changes to existing code**: Purely additive infrastructure.
- **Upstream dependency**: complyctl `dev-testing-environment` (already
  merged).
