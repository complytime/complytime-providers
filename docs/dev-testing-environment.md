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
