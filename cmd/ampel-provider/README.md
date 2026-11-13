# ampel-plugin

## Overview

NOTE: The development of this plugin is in progress and therefore it should only be used for testing purposes at this point.

**ampel-plugin** is a scanning provider which extends complyctl to verify branch protection settings on GitHub and GitLab repositories using [AMPEL](https://github.com/carabiner-dev/ampel) and [snappy](https://github.com/carabiner-dev/snappy). The plugin communicates with complyctl via gRPC using the `pkg/plugin` scanning provider interface, providing a standard and consistent communication mechanism that gives independence for plugin developers to choose their preferred languages. This plugin is structured to allow modular development, ease of packaging, and maintainability.

For now, this plugin is developed together with complyctl for better collaboration during this phase of the project. In the future, this plugin may be decoupled into its own repository.

## Plugin Structure

```
ampel-plugin/
├── config/               # Package for plugin configuration
│ ├── config_test.go      # Tests for functions in config.go
│ └── config.go           # Main code used to process plugin configuration
├── convert/              # Package to match requirement rules to AMPEL policies
│ ├── convert_test.go     # Tests for functions in convert.go
│ ├── convert.go          # Main code used to match and merge AMPEL policies
│ └── types.go            # AMPEL policy type definitions
├── docs/                 # Documentation and configuration reference
│ └── configuration.md    # Detailed configuration reference with examples
├── intoto/               # Package for in-toto attestation handling
│ ├── intoto_test.go      # Tests for functions in intoto.go
│ └── intoto.go           # In-toto statement and DSSE envelope types
├── results/              # Package to parse AMPEL results and produce assessment logs
│ ├── results_test.go     # Tests for functions in results.go
│ └── results.go          # Main code used to parse AMPEL output and produce assessment logs
├── scan/                 # Package to execute snappy and ampel commands
│ ├── scan_test.go        # Tests for functions in scan.go
│ ├── scan.go             # Main code used to orchestrate repository scanning
│ └── specs/              # Embedded spec files for snappy
├── server/               # Package to process server functions. Here is where the plugin communicates with complyctl CLI
│ ├── server_test.go      # Tests for functions in server.go
│ ├── server.go           # Main code used to process server functions
│ ├── unpack.go           # Secure tar.gz extraction for complypack content
│ └── unpack_test.go      # Tests for functions in unpack.go
├── targets/              # Package to parse and validate target repository URLs
│ ├── targets_test.go     # Tests for functions in targets.go
│ └── targets.go          # Main code used to parse repository URLs and detect platforms
├── toolcheck/            # Package to verify required external tools are available
│ ├── toolcheck_test.go   # Tests for functions in toolcheck.go
│ └── toolcheck.go        # Main code used to check snappy and ampel availability
├── main.go               # Plugin entry point
└── README.md             # This file
```

## Features

### Configuration

The plugin receives its configuration through the complyctl scanning provider interface:

- **Global variables** (via `GenerateRequest`): Workspace-scoped settings shared across all targets, such as `ampel_policy_dir` for specifying a custom granular policy source directory.
- **Target variables** (via `ScanRequest`): Per-target settings including repository URL, branch names, snappy spec references, and optional authentication tokens.

See `docs/configuration.md` for the complete configuration reference with examples.

### Target Configuration

Each repository to scan is defined as its own target entry in `complytime.yaml`. Repository details are passed as plain string variables, with multi-value fields using comma-separated strings:

```yaml
targets:
  - id: myorg-frontend
    policies:
      - branch-protection
    variables:
      url: https://github.com/myorg/myrepo
      specs: builtin:github/branch-rules.yaml
      branches: main,release
      access_token: ${MY_GITHUB_PAT}  # optional, expanded from env
  - id: myorg-infra
    policies:
      - branch-protection
    variables:
      url: https://gitlab.com/myorg/infrastructure
      specs: builtin:github/branch-rules.yaml
      branches: main
      access_token: ${GITLAB_API_TOKEN}
```

Each target entry supports the following variables:
- **url** (required): HTTPS URL to a GitHub or GitLab repository.
- **specs** (required): Comma-separated snappy spec file references. Use the `builtin:` prefix for embedded specs (e.g., `builtin:github/branch-rules.yaml`) or absolute paths for custom specs.
- **branches** (optional): Comma-separated branch names to scan. Default: `main`.
- **access_token** (optional): Per-repository authentication token. Supports `${VAR}` env var expansion. When set, the token is injected as `GITHUB_TOKEN` or `GITLAB_TOKEN` (based on the repository URL platform) into the snappy subprocess environment. When omitted, snappy inherits the parent process environment.
- **platform** (optional): `github` or `gitlab`. Required for self-hosted instances; auto-detected for `github.com` and `gitlab.com`.

See `docs/configuration.md` for comprehensive examples including mixed-platform scanning and token authentication.

### AMPEL Policies

The plugin resolves granular AMPEL policy files (one JSON file per control) using the following precedence:
1. **Complypack content** — When a complypack is configured for the ampel evaluator-id, complyctl provides the content path automatically. The plugin extracts `content.tar.gz` archives if needed.
2. **`ampel_policy_dir` global variable** — A custom directory specified in `complytime.yaml`.
3. **Default directory** — `{workspace}/.complytime/ampel/granular-policies/`.

During the `generate` phase, the plugin matches assessment configuration requirement IDs (Gemara `requirement-id` values) to granular AMPEL policy IDs and merges the matched policies into a single bundle used for verification. Generated output is written to `{workspace}/.complytime/ampel/generated/<content-key>/` and selected through `generate-index.json`.

Sample policy files are available in the [complytime-demos](https://github.com/complytime/complytime-demos) repository under `base_ansible_env/files/ampel-policies/`.

### Generate

When the plugin receives the `generate` command from complyctl, it will:
* Resolve the granular policy source directory (complypack > `ampel_policy_dir` > default)
* Extract complypack content archives if the source is a `content.tar.gz` file
* Load granular AMPEL policy files from the resolved policy directory
* Match assessment configuration requirement IDs (Gemara `requirement-id` values) to available granular AMPEL policies
* Merge matched policies into a single policy bundle
* Write the bundle and `scan-config.json` to `{workspace}/.complytime/ampel/generated/<content-key>/`

### Scan

When the plugin receives the `scan` command from complyctl, it will:
* Validate that `snappy` and `ampel` CLI tools are available on the system PATH
* Read target repository configuration from the scan request targets
* For each repository, branch, and spec combination:
  * Run `snappy snap` to collect branch protection data from the GitHub or GitLab API as an in-toto attestation
  * Extract the subject hash from the snappy attestation
  * Run `ampel verify` to evaluate the attestation against the generated policy bundle
  * Parse the AMPEL verification results (supporting both raw and DSSE-wrapped attestations)
* Preserve both attestations in a fresh directory for every repository/branch/spec evaluation
* Return assessment results to complyctl for inclusion in the compliance report

### Evidence and workspace

Each scan preserves the raw outputs referenced by its Gemara Evaluation Log, so
later scans do not overwrite earlier evidence. The layout beneath the selected
workspace root is:

```text
.complytime/ampel/results/<repository>/<branch>/<spec>/evaluation-<UTC-time>-<token>/
  snappy.intoto.json
  ampel.intoto.json
  metadata.json
```

The directory contains one evaluation's attestation pair and best-effort supporting
metadata. Readable labels are bounded to 80 bytes and can collide; exclusive
allocation with a 128-bit random token keeps evaluation instances independent.
The spec label describes snappy's collection spec, not a policy identity.
Repeated or failing-compliance evaluations retain separate outputs, even when
their bytes are identical. Many assessments can share one pair without copies.

**Breaking changes:** fixed-path attestations and normalized per-repository JSON
summaries are no longer written or refreshed. Evidence and mapping-reference IDs
now use `ampel_<token>` and `snappy_<token>`. Resolve each returned evidence
coordinate against the selected workspace root, not the log's directory, and
verify the file against its `sha256:<hex>` digest. Mapping references retain
escaped absolute `file://` URLs to the same files. The log alone defines the
evidence set; metadata and other unreferenced files are not implicit evidence.

Complyctl selects `--workspace`, then `COMPLYTIME_WORKSPACE`, then its invoking
directory and delivers that value through Generate global/Scan target `workspace`
variables. Missing/empty variables use provider CWD for legacy/direct callers;
conflicting request roots fail. Relative inputs, collection spec files and provider
configuration now resolve against that workspace, while absolute paths remain
absolute. Invocation-relative configurations must migrate. Tool execution uses
the selected workspace without changing process-wide CWD.

Preservation is always on: no automatic cleanup, retention setting, compression,
deduplication, input snapshot or historical policy archive is added. Storage grows
with each evaluation and interrupted attempts can leave unreferenced partial
outputs. Metadata failures warn but do not suppress complete verified evidence.
The directories use `0700` and files `0600`, owned by the effective execution
UID/GID. The workspace must be writable and existing parents traversable by that
account. Sudo/direct-root evidence remains root-owned; the original user needs
authorized elevated access to read and hash it. There is no original-user handoff.
For permission or disk-space failures, check execution-account access, parent
traversal and capacity without widening old evidence permissions or changing owners.

Uploading a log alone does not upload its evidence bytes. Customers own storage,
transfers, access policies and longevity: preserve the log and its log-selected
files, keep them retrievable and verify transferred bytes against the digests.
Collection and optional packaging remain separate future complyctl capabilities;
moved files require mapping to original references without rewriting the log.
Coordinates can resolve against a copied workspace with its layout preserved;
absolute URLs still describe the originating filesystem and do not relocate.

Upgrade stops refreshing fixed-path outputs and leaves legacy files and historical
logs untouched, without migration or recovery of already overwritten evidence.
Older binaries resume overwrite behavior
after rollback; retained evaluation directories remain readable by authorized readers.
See the [Ampel man page](../../docs/man/complyctl-provider-ampel.md) for path and
evidence-field reference details and the [shared evidence workflow](../../README.md#preserve-and-verify-scan-evidence)
for workspace discovery, URI decoding and log/file transfer guidance.

## Installation

### Prerequisites

- **Go** version 1.24 or higher
- **Make** (optional, for using the Makefile)
- **snappy** CLI tool
- **ampel** CLI tool (version 1.4.2 or later)
- A **GitHub** and/or **GitLab personal access token** with repository read permissions

### Installing snappy and ampel

Since snappy and ampel are not available as RPM packages, install them using `go install`:

```bash
go install github.com/carabiner-dev/snappy@latest
go install github.com/policylabs/ampel/cmd/ampel@latest
```

Ensure the Go binary directory is in your PATH:

```bash
export PATH=$PATH:$HOME/go/bin
```

You can add this line to your `~/.bashrc` or `~/.zshrc` to make it permanent.

Verify the installation:

```bash
snappy --help
ampel --help
```

### Authentication Tokens

The `snappy` tool requires a valid personal access token to access the GitHub or GitLab API for reading branch protection settings. Set the appropriate environment variable before running a scan:

```bash
# For GitHub repositories
export GITHUB_TOKEN=ghp_your_token_here

# For GitLab repositories
export GITLAB_TOKEN=glpat-your_token_here
```

The token needs at minimum read access to the repositories being scanned. When scanning repositories across both platforms, both environment variables must be set. Alternatively, per-repository tokens can be configured via the `access_token` target variable in `complytime.yaml` (see `docs/configuration.md`).

### Clone the repository

```bash
git clone https://github.com/complytime/complyctl.git
cd complyctl
```

## Build Instructions

To compile complyctl and the ampel-plugin:

```bash
make build
cd cmd/ampel-plugin && go build -mod=vendor -o ../../bin/complyctl-provider-ampel .
```

Note: The main `make build` target compiles complyctl and the openscap-plugin. The ampel-plugin must be built separately as shown above.

### Plugin Registration

After building, register the plugin with complyctl by placing the binary in the providers directory with the required naming convention:

```bash
mkdir -p ~/.local/share/complytime/providers
cp bin/complyctl-provider-ampel ~/.local/share/complytime/providers/
```

The plugin is discovered automatically by complyctl — no manifest files or checksums are required. The evaluator ID is derived from the executable name by removing the `complyctl-provider-` prefix (e.g., `complyctl-provider-ampel` becomes evaluator ID `ampel`).

### Running

To use the plugin with `complyctl`, see the quick start [guide](../../docs/QUICK_START.md).

### Using complytime-demos with a Fedora 43 VM

The [complytime-demos](https://github.com/complytime/complytime-demos) repository provides an automated way to set up a complete environment with complyctl, the ampel-plugin, and all required tools inside a Fedora 43 VM using Vagrant and Ansible.

**Prerequisites:** Vagrant with the libvirt provider and Ansible installed on the host.

1. Clone both repositories on the host machine:

```bash
git clone https://github.com/complytime/complytime-demos.git
git clone https://github.com/complytime/complyctl.git
```

2. Provision the Fedora 43 VM:

```bash
cd complytime-demos/base_vms/fedora
vagrant up
```

3. Deploy complyctl binaries, the ampel-plugin, AMPEL tools (snappy and ampel), policies, and targets to the VM:

```bash
cd ../../base_ansible_env
ansible-playbook populate_complyctl_dev_binaries.yml
```

This playbook builds complyctl from source, copies the binaries and plugin, installs snappy and ampel via `go install`, and deploys the AMPEL policy files and targets configuration to the VM.

4. Deploy the AMPEL policy content (catalog, profile, and component definition):

```bash
ansible-playbook populate_complyctl_dev_content.yml
```

5. SSH into the VM and run a scan:

```bash
vagrant ssh
# or: ssh ansible@<VM_IP>

export GITHUB_TOKEN=ghp_your_token_here
complyctl generate --policy-id branch-protection
complyctl scan --policy-id branch-protection
```

Note: Update the `complyctl_repo_dest` variable in the playbook if your local complyctl clone is not at the default path. See the complytime-demos README for additional configuration options.

### Testing

Tests are organized within each package. Whenever possible a unit test is created for every function.

Run tests using:

```bash
make test-unit
```
