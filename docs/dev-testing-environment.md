# Dev Testing Environment

This repository includes a Fedora-based devcontainer
for testing provider PR branches against complyctl from
main. Providers are built from your local source while
complyctl is cloned and built from its main branch,
giving you an isolated environment to validate provider
changes end-to-end.

## Quick Start

Open this repository in any devcontainer-compatible tool
(Codespaces, DevPod, or VS Code Dev Containers). The
post-create script builds everything automatically.

## Shared Concepts

For full setup details, prerequisites, and tool-specific
guides, see the complyctl testing environment docs:

[complyctl Dev Testing Environment][complyctl-docs]

The sections below summarize the key topics so this doc
remains useful if the link is unavailable.

**GitHub Codespaces**: Navigate to the PR on GitHub,
click **Code** > **Codespaces** > **Create codespace on
\<branch\>**. The devcontainer builds automatically.

**DevPod**: From a local clone, run:

```bash
devpod up . --ide none \
  && devpod ssh complytime-providers
```

**VS Code**: Open the repository folder in VS Code and
click **Reopen in Container** when prompted.

**GITHUB_TOKEN**: Required for `complyctl scan`. Set it
via Codespaces secrets (**Settings > Codespaces >
Secrets**) or export it manually in the terminal:

```bash
export GITHUB_TOKEN=<your-token>
```

## What's Different from complyctl's Devcontainer

- **Providers built from local source**: The post-create
  script runs `make build` against your PR branch.
  complyctl is cloned and built from main.
- **Auto-rebuild targets providers**: When the container
  detects a new commit on login, it rebuilds providers
  (not complyctl) via `make build`.
- **OPA provider binary skipped if absent**: The setup
  loop copies openscap, ampel, and opa provider binaries
  to `~/.local/share/complytime/providers/`, gracefully skipping any
  that are not present. This is forward-compatible with
  providers added later.

## Command Reference

```bash
cd ~/test-workspace

# Fetch policies from the mock registry
complyctl get

# Generate a policy bundle for the ampel provider
complyctl generate --policy-id test-ampel-bp

# Run a scan (requires GITHUB_TOKEN)
GITHUB_TOKEN=<your-token> complyctl scan \
  --policy-id test-ampel-bp
```

## Released-host integration

The released-baseline CI job tests the selected stable complyctl
implementation with separately pinned canonical assertions. This keeps
evidence checks current without rebuilding or patching the released host.
The independent main-consumer job still tracks complyctl main; its result
does not replace released-host validation.

### Revision ownership and pin updates

The workflow's `CANONICAL_HARNESS_COMMIT` initially names the merged,
approved [complyctl PR #975](https://github.com/complytime/complyctl/pull/975),
commit `3cdfdd2a4f2d66f186b91e8558710d07e6df6413`. It provides mapped
evidence checks and the `COMPLYCTL_BIN_DIR` selection interface. The
workflow owns the pin; local commands read it rather than maintain a
second configuration value.

| Input | Owner |
| --- | --- |
| Released tag/commit | Existing compatibility-line release resolver |
| complyctl and mock registry binaries | Unmodified released checkout's `bin/` |
| Assertions, helpers, and fixtures | Verified canonical harness checkout |
| All three provider binaries | Isolated workspace built/tested against that release |

Refresh the harness pin through a reviewed PR, never mutable runtime
resolution. Verify the candidate commit is merged, inspect changes to
all sourced helpers, fixtures, and host-selection behavior, and rerun
offline execution tests plus real released-host and main-host integration.
Record all three repository revisions and actual tool versions. A new
harness requiring unsupported host or registry behavior is a compatibility
failure, not permission to substitute newer binaries or weaken assertions.

### Reproduce dispatch with prebuilt checkouts

First reproduce the workflow's preparation: resolve the stable baseline,
verify its checkout, and build its `build` target. In a separate provider
workspace, replace the complyctl module with that released checkout using
`go mod edit -replace`, run `go mod tidy`, then run `GOFLAGS=-mod=mod make
build` and `GOFLAGS=-mod=mod make test`. Do not change the committed
dependency in your original working tree or reuse its provider binaries.
The canonical checkout is for assertions only; do not build it.

From the root of that built, isolated provider workspace, set the
following inputs from the CI provenance record or release resolver:

- `COMPLYCTL_DIR`: canonical harness checkout, not the released checkout.
- `RELEASED_COMPLYCTL_DIR`: absolute path to the unmodified released checkout.
- `RELEASED_COMPLYCTL_COMMIT`: full commit reported by the release resolver.
- `COMPLYCTL_TAG`: selected stable tag from the same resolver result.
- `GITHUB_TOKEN`: token with read access to public repository branch protection.

The current harness also requires Git, Bash, Make, timeout, jq, curl,
GNU realpath/sha256sum, Snappy, Ampel, and Conftest. Run:

```bash
: "${COMPLYCTL_DIR:?Set the canonical harness checkout}"
: "${RELEASED_COMPLYCTL_DIR:?Set the absolute released checkout path}"
: "${RELEASED_COMPLYCTL_COMMIT:?Set the resolved full released commit}"
: "${COMPLYCTL_TAG:?Set the resolved released tag}"
: "${GITHUB_TOKEN:?Set a token with the required read access}"
export COMPLYCTL_DIR RELEASED_COMPLYCTL_DIR RELEASED_COMPLYCTL_COMMIT COMPLYCTL_TAG GITHUB_TOKEN
CANONICAL_HARNESS_COMMIT=$(awk '$1 == "CANONICAL_HARNESS_COMMIT:" {print $2}' \
    .github/workflows/ci_cross_repo_integration.yml)
export CANONICAL_HARNESS_COMMIT
export PROVIDERS_BIN_DIR="$PWD/bin"
bash .github/scripts/run-released-complyctl-integration.sh
```

Paths containing spaces are supported. Dispatch invokes the provider
Make target with the absolute released `COMPLYCTL_BIN_DIR`; it does not
invoke the released checkout's historical integration Make target or
build either host. The canonical harness owns token handling and assertions.
The released dispatcher rejects generation-index-only fixture mode so it
cannot be mistaken for a live integration result.

Inspect source provenance without printing credentials:

```bash
git -C "$COMPLYCTL_DIR" rev-parse HEAD
git -C "$RELEASED_COMPLYCTL_DIR" rev-parse HEAD
git -C "$RELEASED_COMPLYCTL_DIR" diff --exit-code HEAD --
git rev-parse HEAD
go version -m "$RELEASED_COMPLYCTL_DIR/bin/complyctl"
go version -m "$RELEASED_COMPLYCTL_DIR/bin/mock-oci-registry"
```

Logs and the released job summary identify the host tag/commit, harness
commit, and provider commit separately. `canonical-harness-preparation:`
identifies a harness checkout/interface/fixture problem;
`released-integration-preparation:` identifies missing or mismatched
released inputs. Provider dependency/build/test and integration failures
retain their distinct stage diagnostics and nonzero results.

### Offline execution regressions

Run these without credentials or live hosts:

```bash
bash .github/scripts/test/resolve-complyctl-release-baseline_test.sh
bash .github/scripts/test/validate-released-complyctl-checkout_test.sh
bash .github/scripts/test/released-baseline-diagnostics_test.sh
bash .github/scripts/test/canonical-complyctl-harness_test.sh
```

The execution suite uses distinguishable fixture executables and the real
provider Make/dispatch path, including paths with spaces, input rejection,
and failure propagation. It also executes extracted workflow preparation,
build, and test commands with failing tool fixtures to prove integration
does not run after provider checks fail. These are wiring regressions,
not evidence of live released-host or retained-provider compatibility.

## Troubleshooting

For the full troubleshooting guide, see the
[complyctl docs][complyctl-docs]. The most common
issues are summarized below.

**Post-create script did not run**: If `complyctl` is
not found after `devpod up`, the `postCreateCommand`
may have failed silently. Re-run it manually:

```bash
devpod ssh complytime-providers
bash .devcontainer/scripts/post-create.sh
```

**"unset system credential helper" error**: DevPod may
show `error unset system credential helper exit status 5`
during container setup. This is a cosmetic error from
DevPod's git credential forwarding and does not affect
the devcontainer. The post-create script runs
independently of git credential configuration.

**DevPod prompts for workspace**: Always specify `.`
for a local directory:

```bash
devpod up . --ide none
```

Without `.`, DevPod prompts to select an existing
workspace instead of creating one from the current
directory.

**Mock registry not running**: If `complyctl get` fails
to connect, start the registry manually:

```bash
./bin/mock-oci-registry &
```

**GITHUB_TOKEN not set**: `complyctl scan` fails without
a valid token. Export it in your shell:

```bash
export GITHUB_TOKEN=<your-token>
```

**File ownership changed after using DevPod (podman)**:
When using DevPod with podman rootless, the container's
user namespace remaps your host UID. This can change
file ownership on the host after the workspace stops.
Fix with:

```bash
podman unshare chown -R 0:0 \
  /path/to/complytime-providers
```

**OpenSCAP limitations in containers**: OpenSCAP system
scans have limited functionality inside containers due
to missing host-level access. Use the ampel provider
with the mock registry for CLI testing. Full OpenSCAP
testing is available via
[complytime-demos][complytime-demos].

## See Also

- [complyctl Dev Testing Environment][complyctl-docs]
- [Provider Development Guide](./provider-guide.md)
- [complytime-demos][complytime-demos]

[complyctl-docs]: https://github.com/complytime/complyctl/blob/main/docs/TESTING_ENVIRONMENT.md
[complytime-demos]: https://github.com/complytime/complytime-demos
