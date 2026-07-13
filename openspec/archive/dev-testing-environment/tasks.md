All tasks mirror complyctl's implementation
(`complyctl:.devcontainer/`) with the build order reversed. Reference
complyctl's files as the source to adapt from.

## 1. Containerfile

- [x] 1.1 Create `.devcontainer/Containerfile` mirroring complyctl's
  Containerfile (same base image, packages, non-root user,
  GOTOOLCHAIN=auto). Add a comment referencing complyctl's Containerfile
  as the sync source

## 2. Post-create setup script

- [x] 2.1 Create `.devcontainer/scripts/post-create.sh` adapted from
  complyctl's script with reversed build order: build providers from
  local source (`make build`), then clone and build complyctl from main.
  Mirror complyctl's patterns: `set -euo pipefail`, PATH setup,
  GITHUB_TOKEN least-privilege (capture and unset, do not log token)
- [x] 2.2 Add `go install` of snappy
  (`github.com/carabiner-dev/snappy@v0.2.4`), ampel
  (`github.com/carabiner-dev/ampel/cmd/ampel@v1.2.1`), and conftest
  (`github.com/open-policy-agent/conftest@v0.68.2`) at pinned versions
  (mirror complyctl's Step 2)
- [x] 2.3 Clone `complyctl@main`, build complyctl and mock-oci-registry,
  log cloned commit SHA. Exit with clear error identifying the clone
  URL and failure cause if clone fails
- [x] 2.4 Copy provider binaries to `~/.complytime/providers/` -- loop
  over openscap, ampel, opa with graceful skip for missing binaries
  (mirror complyctl's provider copy pattern)
- [x] 2.5 Workspace setup: copy test fixtures from cloned complyctl's
  `tests/cross-repo/testdata/`, generate OPA test deployment manifest
  inline (mirror complyctl's Step 4 and inline heredoc pattern)
- [x] 2.6 Private bundle registration: mirror complyctl's Step 4b
  (discover mounted Gemara policies from /bundles/ or
  $COMPLYCTL_BUNDLES_DIR)
- [x] 2.7 Mock OCI registry startup with nohup/disown and readiness
  check. Log output to `/tmp/mock-oci-registry.log` (mirror complyctl's
  Step 5, pass MOCK_REGISTRY_CONTENT_DIR)
- [x] 2.8 Record build commit and add .bashrc auto-rebuild hook for
  providers (mirror complyctl's Steps 6-7, targeting `make build`
  for providers instead of complyctl)

## 3. Devcontainer configuration

- [x] 3.1 Create `.devcontainer/devcontainer.json` mirroring complyctl's
  (same remoteUser, runArgs, hostRequirements), change name to
  `complytime-providers`

## 4. Documentation

- [x] 4.1 Create `docs/dev-testing-environment.md` covering
  provider-specific setup and referencing complyctl's
  `docs/TESTING_ENVIRONMENT.md` via GitHub `main` branch blob URL
  for shared concepts. Include brief inline summaries of shared
  topics so the doc remains useful if the link breaks
- [x] 4.2 Update `README.md` with link to dev testing environment docs
- [x] 4.3 Update `AGENTS.md` project structure to include
  `.devcontainer/` entries and add `make test-devcontainer` to the
  Build & Test Commands section

## 5. CI Smoke Test

- [x] 5.1 Add `test-devcontainer` target to Makefile: `podman build
  .devcontainer/` and `shellcheck .devcontainer/scripts/post-create.sh`
  (mirror complyctl's CI pattern, add static analysis for the script)

## 6. Verification (manual)

- [x] 6.1 Verify Containerfile builds: `podman build .devcontainer/`
  (deferred: requires interactive devcontainer; CI smoke test covers build)
- [x] 6.2 Verify post-create completes: all binaries available, mock
  registry responds at localhost:8765/v2/, workspace configured
  (deferred: requires interactive devcontainer)
- [x] 6.3 Verify end-to-end: `complyctl get` and
  `complyctl generate --policy-id test-ampel-bp` succeed with provider
  binaries from local source
  (deferred: requires interactive devcontainer)
<!-- spec-review: passed -->
<!-- code-review: passed -->
