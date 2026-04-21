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
│ └── server.go           # Main code used to process server functions
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

The plugin uses granular AMPEL policy files (one JSON file per control) stored in the granular policy directory (default: `{workspace}/ampel/granular-policies/`, configurable via the `ampel_policy_dir` global variable in `complytime.yaml`). During the `generate` phase, the plugin matches assessment configuration requirement IDs to these policies and merges the matched policies into a single bundle used for verification. Generated output is written to `{workspace}/ampel/policy/`.

Sample policy files are available in the [complytime-demos](https://github.com/complytime/complytime-demos) repository under `base_ansible_env/files/ampel-policies/`.

### Generate

When the plugin receives the `generate` command from complyctl, it will:
* Load granular AMPEL policy files from the configured policy directory
* Match assessment configuration requirement IDs to available AMPEL policies
* Merge matched policies into a single policy bundle
* Write the bundle to `{workspace}/ampel/policy/complytime-ampel-policy.json`

### Scan

When the plugin receives the `scan` command from complyctl, it will:
* Validate that `snappy` and `ampel` CLI tools are available on the system PATH
* Read target repository configuration from the scan request targets
* For each repository, branch, and spec combination:
  * Run `snappy snap` to collect branch protection data from the GitHub or GitLab API as an in-toto attestation
  * Extract the subject hash from the snappy attestation
  * Run `ampel verify` to evaluate the attestation against the generated policy bundle
  * Parse the AMPEL verification results (supporting both raw and DSSE-wrapped attestations)
* Write per-repository result files to the configured results directory
* Return assessment results to complyctl for inclusion in the compliance report

## Installation

### Prerequisites

- **Go** version 1.24 or higher
- **Make** (optional, for using the Makefile)
- **snappy** CLI tool
- **ampel** CLI tool
- A **GitHub** and/or **GitLab personal access token** with repository read permissions

### Installing snappy and ampel

Since snappy and ampel are not available as RPM packages, install them using `go install`:

```bash
go install github.com/carabiner-dev/snappy@latest
go install github.com/carabiner-dev/ampel/cmd/ampel@latest
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
mkdir -p ~/.complytime/providers
cp bin/complyctl-provider-ampel ~/.complytime/providers/
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
