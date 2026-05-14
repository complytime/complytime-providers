# opa-provider

## Overview

NOTE: The development of this plugin is in progress and therefore it should only be used for testing purposes at this point.

**opa-provider** is a scanning provider which extends complyctl to evaluate configuration files (Kubernetes manifests, Terraform plans, Dockerfiles, etc.) against OPA/Rego policies using [conftest](https://www.conftest.dev/). The plugin communicates with complyctl via gRPC using the `pkg/plugin` scanning provider interface.

This provider complements the existing openscap (system-level XCCDF scanning) and ampel (in-toto attestation verification) providers by covering configuration-as-code policy evaluation.

## Plugin Structure

```
opa-provider/
├── config/               # Package for workspace directory configuration
│ ├── config_test.go      # Tests for functions in config.go
│ └── config.go           # Workspace path construction and directory creation
├── loader/               # Package for loading scan input data
│ ├── loader_test.go      # Tests for functions in loader.go
│ ├── loader.go           # DataLoader interface with GitLoader, LocalPathLoader, Router
│ ├── vars_test.go        # Tests for target variable constants
│ └── vars.go             # Target variable key constants
├── results/              # Package to parse conftest output and produce assessment logs
│ ├── results_test.go     # Tests for functions in results.go
│ └── results.go          # Conftest JSON parsing, OSCAL mapping, scan-status assessment
├── scan/                 # Package to execute conftest commands
│ ├── scan_test.go        # Tests for functions in scan.go
│ └── scan.go             # CommandRunner interface, PullBundle, EvalPolicy
├── server/               # Package implementing the gRPC provider interface
│ ├── server_test.go      # Tests for functions in server.go
│ └── server.go           # Describe, Generate, Scan RPCs with ServerOptions injection
├── targets/              # Package for URL parsing and path validation
│ ├── targets_test.go     # Tests for functions in targets.go
│ └── targets.go          # ParseRepoURL, SanitizeRepoURL, ValidateInputPath
├── toolcheck/            # Package to verify required external tools are available
│ ├── toolcheck_test.go   # Tests for functions in toolcheck.go
│ └── toolcheck.go        # Checks conftest and git availability on PATH
├── main.go               # Plugin entry point
└── README.md             # This file
```

## Features

### Target Configuration

Each target to scan is defined in `complytime.yaml`. Targets support either remote git repositories or local filesystem paths:

```yaml
targets:
  - id: myorg-k8s-configs
    policies:
      - container-security
    variables:
      url: https://github.com/myorg/k8s-configs
      opa_bundle_ref: ghcr.io/myorg/opa-policies:latest
      branches: main,staging
      scan_path: deploy/kubernetes
      access_token: ${MY_GITHUB_PAT}  # optional, expanded from env
  - id: local-terraform
    policies:
      - infra-compliance
    variables:
      input_path: /path/to/terraform/configs
      opa_bundle_ref: ghcr.io/myorg/opa-policies:latest
```

Each target entry supports the following variables:

| Variable | Required | Description |
|:---------|:---------|:------------|
| `url` | One of `url` or `input_path` | HTTPS URL to a git repository |
| `input_path` | One of `url` or `input_path` | Absolute local filesystem path to scan |
| `opa_bundle_ref` | Yes | OCI reference for the conftest policy bundle (e.g., `ghcr.io/org/bundle:v1`) |
| `branches` | No | Comma-separated branch names to scan. Default: `main` |
| `access_token` | No | Git authentication token. Injected via `GIT_CONFIG_COUNT` credential helper |
| `scan_path` | No | Subdirectory within the cloned repository to scan |

**Validation rules:**
- `url` and `input_path` are mutually exclusive; setting both is an error
- `url` must use the HTTPS scheme
- Branch names must match `^[a-zA-Z0-9._/-]+$` and must not contain `..`
- `scan_path` must not contain `..`
- `access_token` must not contain newline, carriage return, or null characters

### OPA Policy Bundles

The provider uses conftest to pull OPA policy bundles from OCI registries. The `opa_bundle_ref` variable specifies the bundle reference. Bundles are pulled once per unique reference and cached for the duration of the scan.

For private OCI registries, authenticate using `docker login` before running a scan:

```bash
docker login ghcr.io
```

### Generate

The Generate phase is currently a stub that returns success. Policy selection from assessment plans is deferred to a future release.

### Scan

When the plugin receives the `scan` command from complyctl, it will:
1. Validate that `conftest` and `git` are available on the system PATH
2. Create workspace directories under `<workspace>/opa/{policy,repos,results}/`
3. For each target:
   - Pull the OPA policy bundle from the OCI registry (cached per unique `opa_bundle_ref`)
   - Load input data (clone git repo with `--depth 1`, or validate local path)
   - Run `conftest test <path> --policy <dir> --output json --all-namespaces --no-fail`
   - Parse conftest JSON output into findings grouped by requirement ID
   - Write per-target result files as JSON to the results directory
4. Return assessment results to complyctl with a `scan-status` summary prepended

**Requirement ID derivation:** Requirement IDs are extracted from conftest query metadata. For example, the query `data.kubernetes.run_as_root.deny` produces the requirement ID `kubernetes.run_as_root` (the `data.` prefix and rule type suffix are stripped).

**Error handling:** Per-target errors (clone failures, policy evaluation errors) are captured in the results and the scan continues with remaining targets. Global errors (missing tools, no targets) return an error immediately.

### Workspace Layout

```
<workspace>/opa/
├── policy/       # Downloaded OPA policy bundles
├── repos/        # Cloned git repositories
└── results/      # Per-target scan result JSON files
```

Directories are created with mode `0750`. Result files are written with mode `0600`.

## Installation

### Prerequisites

- **Go** version 1.25 or higher
- **conftest** CLI tool ([install guide](https://www.conftest.dev/install/))
- **git** CLI tool
- Access to an OCI registry hosting OPA policy bundles

### Installing conftest

```bash
# macOS
brew install conftest

# Linux (download binary)
LATEST=$(curl -s https://api.github.com/repos/open-policy-agent/conftest/releases/latest | grep tag_name | cut -d '"' -f 4 | sed 's/v//')
curl -L "https://github.com/open-policy-agent/conftest/releases/download/v${LATEST}/conftest_${LATEST}_Linux_x86_64.tar.gz" | tar xz
sudo mv conftest /usr/local/bin/
```

### Build

```bash
make build-opa-provider
```

This produces `bin/complyctl-provider-opa`.

### Plugin Registration

After building, register the plugin with complyctl by placing the binary in the providers directory:

```bash
mkdir -p ~/.complytime/providers
cp bin/complyctl-provider-opa ~/.complytime/providers/
```

The plugin is discovered automatically by complyctl — no manifest files or checksums are required. The evaluator ID is derived from the executable name by removing the `complyctl-provider-` prefix (e.g., `complyctl-provider-opa` becomes evaluator ID `opa`).

### Running

To use the plugin with `complyctl`, see the quick start [guide](../../docs/QUICK_START.md).

### Testing

Tests are organized within each package. Run tests using:

```bash
make test
```

Run with the race detector:

```bash
go test ./cmd/opa-provider/... -race -count=1
```

## Troubleshooting

| Symptom | Cause | Fix |
|:--------|:------|:----|
| `required tools not found: conftest` | conftest not installed or not on PATH | Install conftest and ensure it is in your PATH |
| `required tools not found: git` | git not installed or not on PATH | Install git |
| `opa_bundle_ref variable is required but not set` | Missing `opa_bundle_ref` in target variables | Add `opa_bundle_ref` to the target's variables in `complytime.yaml` |
| `url must use HTTPS scheme` | Target URL uses `http://` or `ssh://` | Change the URL to use `https://` |
| `specify either url or input_path, not both` | Both `url` and `input_path` set on the same target | Use one or the other per target |
| `branch name contains path traversal` | Branch name contains `..` | Use a valid branch name |
| `input path contains directory traversal` | `input_path` contains `..` | Use an absolute path without traversal sequences |
| `pulling policy bundle: ...` | OCI registry authentication failure | Run `docker login <registry>` before scanning |
| `cloning repository: ...` | Git clone failed (auth, network, or bad URL) | Verify the URL and access token; check network connectivity |
