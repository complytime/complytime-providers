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
│ └── results.go          # Conftest JSON parsing and assessment mapping
├── generate/             # Package for Generate RPC mapping and scan config
│ ├── mapping_test.go     # Tests for functions in mapping.go
│ ├── mapping.go          # MappingFile loading, validation, requirement matching
│ ├── scanconfig_test.go  # Tests for functions in scanconfig.go
│ └── scanconfig.go       # ScanConfig read/write for Generate→Scan handoff
├── scan/                 # Package to execute conftest commands
│ ├── scan_test.go        # Tests for functions in scan.go
│ └── scan.go             # CommandRunner interface, PullBundle, namespace-scoped evaluation
├── server/               # gRPC provider and workspace/input safety preflight
│ ├── server.go           # Describe, Generate, Scan RPCs with ServerOptions injection
│ ├── retention.go        # Request-local workspace dependencies and evidence context
│ ├── conftest_config.go  # Conftest configuration discovery
│ ├── input_selection.go  # Target-wide rejection and evaluated-input errors
│ ├── file_identity.go    # Filesystem identity representation
│ ├── file_identity_unix.go, file_identity_windows.go, file_identity_other.go
│ │                       # Platform-specific identity checks
│ ├── workspace.go        # Workspace boundary resolution
│ ├── workspace_alias.go  # Input alias inspection
│ ├── workspace_context.go # Cancellation handling
│ ├── workspace_identity.go # Workspace identity collection
│ ├── workspace_ignore.go # Conftest workspace ignore expressions
│ ├── workspace_remote.go # Remote clone input checks
│ └── *_test.go           # Unit, real-conftest, CLI, and benchmark tests
├── targets/              # Package for URL parsing and path validation
│ ├── targets_test.go     # Tests for functions in targets.go
│ └── targets.go          # ParseRepoURL, SanitizeRepoURL, ValidateInputPath
├── toolcheck/            # Package to verify required external tools are available
│ ├── toolcheck.go        # Checks conftest and git availability on PATH
│ └── toolcheck_test.go   # Tool availability tests
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
| `input_path` | One of `url` or `input_path` | Local filesystem path to scan |
| `opa_bundle_ref` | When no complypack is available | OCI reference for the conftest policy bundle (e.g., `ghcr.io/org/bundle:v1`) |
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

When the plugin receives the `generate` command from complyctl, it:

1. Validates that `conftest` and `git` are available on the system PATH
2. Pulls the OCI policy bundle specified by `opa_bundle_ref`
3. Looks for a `complytime-mapping.json` file in the root of the pulled bundle
4. If found: matches each `RequirementID` (the Gemara `requirement-id` value, not the plan's own `id`) from the assessment plan against the mapping entries, produces a list of matched Rego namespace IDs and a reverse mapping for result resolution
5. Writes `.complytime/opa/generated/<content-key>/scan-config.json` and a target index beneath the selected workspace for Scan to consume

**Mapping file format:** Policy bundle authors MUST include a `complytime-mapping.json` file in their OCI bundle to declare which Rego namespace corresponds to which compliance requirement:

```json
{
  "version": "1",
  "mappings": [
    {"id": "kubernetes.run_as_root", "requirement_id": "CIS-K8S-5.2.6"},
    {"id": "kubernetes.resource_limits", "requirement_id": "CIS-K8S-5.4.1"}
  ]
}
```

The `id` field is the Rego package namespace (the semantic, benchmark-agnostic identity, equivalent to AMPEL's granular policy `id` field). The `requirement_id` must match the Gemara assessment plan's `requirement-id` field (e.g., `CIS-K8S-5.2.6`), not the plan's own `id` field.

**Missing mapping file:** If the OCI bundle does not contain a `complytime-mapping.json` file, Generate returns `{Success: false}` with an error message identifying the expected file. If the file exists but is malformed (invalid JSON, empty fields, duplicate entries), Generate returns a distinct validation error.

### Scan

When the plugin receives the `scan` command from complyctl, it will:
1. Validate that `conftest` and `git` are available on the system PATH
2. Resolve one workspace from the RPC variables and create `.complytime/opa/{policy,repos,results,generated}/` beneath it
3. Read `scan-config.json` from the generated directory (Generate must have been run first)
4. For each target:
   - Pull the OPA policy bundle from the OCI registry (cached per unique `opa_bundle_ref`)
   - Load input data (clone git repo with `--depth 1`, or validate local path)
   - Run `conftest test` with `--policy`, `--output json`, `--no-fail`, and one `--namespace` per scan-config ID. For a local directory enclosing a workspace, add a composed `--ignore` expression to exclude workspace paths while preserving an effective `CONFTEST_IGNORE` rule.
   - Preserve exact JSON stdout as `conftest.json` in a fresh evaluation directory; keep diagnostic stderr separate
   - Parse and hash the retained bytes into findings grouped by requirement ID
   - Resolve Rego-derived IDs to Gemara requirement IDs using the reverse mapping from the scan config
   - Attempt supporting metadata, then verify execution-account ownership and private modes before publishing evidence
5. Return assessment results to complyctl. Operational errors are reported via `ScanResponse.Errors`.

**Requirement ID resolution:** Generate provides a reverse mapping that resolves Rego-derived IDs (e.g., `kubernetes.run_as_root`) to Gemara requirement IDs (from `AssessmentConfiguration.RequirementID`, e.g., `CIS-K8S-5.2.6`) in the scan response.

**Error handling:** Per-target errors, including missing conftest, are reported via
`ScanResponse.Errors`; the scan continues with remaining targets. If any selected remote branch
fails input-safety preflight or has no confirmed evaluated directory input, no branch of that
configured target contributes assessments. Other configured targets retain their results.
Other global errors, such as invalid/conflicting workspaces, execution-identity or ownership/mode-verification failures, a missing git executable or no targets, return a gRPC-level error without a response. Ordinary branch-scoped tool/storage errors remain branch-scoped. Artifacts from rejected targets remain on disk without published evidence references.

### Workspace Layout

```text
<workspace>/.complytime/opa/
├── generated/<content-key>/scan-config.json
├── generated/generate-index.json
├── policy/       # Replaceable downloaded policies, not archived evidence
├── repos/<repository>/<branch>/  # Current replaceable clone only
└── results/
    ├── <repository>/<branch>/evaluation-<UTC-time>-<token>/
    │   ├── conftest.json
    │   └── metadata.json
    └── <local-target>/evaluation-<UTC-time>-<token>/
        ├── conftest.json
        └── metadata.json
```

Labels are bounded display hints, not unique identities. Each evaluation uses an
independent random token and a UTC collection-start time such as `20261110T235959Z`.
The same instant supplies evidence timestamps and supporting metadata.

### Locate and verify retained evidence

**Breaking changes:** OPA evidence now identifies retained result files for remote
repositories, local files and local directories, never the evaluated inputs.
Evidence IDs use `conftest_<token>` and matching reference IDs use
`conftest-result_<token>`. Every artifact has a SHA256 digest of its stored output,
even when the input was a directory. Multiple assessments share one result file.

Use the Gemara Evaluation Log's evidence mappings to find the evidence set. Resolve
each `Source.Coordinate` against the selected workspace root, not the log directory
or your current directory, then compare the file's SHA256 with `Source.Digest`
(`sha256:<hex>`). The linked mapping reference provides an escaped absolute
`file://` URL for the same file; decode URI escapes such as `%20` and `%25` before
using it as a filesystem path. In-place reading and verification require no package
or storage integration. Metadata supports correlation but is best-effort and is
not evidence merely because it sits beside `conftest.json`.

Complyctl selects `--workspace` over `COMPLYTIME_WORKSPACE` over the invoking
directory and delivers the root through Generate global and Scan target
`workspace` variables. The provider uses those variables without reapplying
environment precedence; direct callers that omit them or supply an empty value
use provider CWD. All targets in an RPC must agree on the root. **Breaking:** relative
inputs, policy/configuration paths, conftest discovery and relative
`CONFTEST_CONFIG_FILE` paths now use that root, without process-wide chdir.
Absolute external input paths remain absolute. Input exclusion still protects
both the selected root's `.complytime` and a configured provider artifact directory
when different, including an existing invocation-relative default. An absent old
default is not created in the invoking tree.

Preservation is always on: repeated scans retain changed and identical outputs
separately, with no automatic cleanup, retention settings, compression or
deduplication. Clones remain replaceable; local inputs stay in place. Conftest's
embedded input filenames are provenance, not retained evidence links. Inputs and
policies are not archived; historical re-evaluation is not guaranteed, and unchanged
findings do not prove unchanged inputs. Storage and workspace identity-preflight
work grow with retained history. Inaccessible private history fails preflight
closed; use the authorized execution account or elevated access rather than widening
historical permissions or treating deletion as an automatic remedy.

Evaluation leaves use mode `0700`, and result/metadata files use `0600`, owned by
the process's effective UID/GID. Sudo and direct-root scans remain root-owned;
the original user needs authorized elevated access to read/hash them. The workspace
must be writable and all context parents traversable by that account. Disk or
permission failures can leave unreferenced partial outputs. Fix storage capacity or
account access and retry; the provider never changes older owners or grants access
to another user. Metadata write/close failures warn without suppressing complete,
verified evidence.

The log alone does not contain or transfer artifact bytes: uploading a log does
not upload its evidence. Customer-owned storage and transfer integrations must
preserve both the log and its log-selected files, keep them retrievable and verify
transferred bytes against the recorded digests. Future optional collection or
packaging must map moved files to original references without rewriting the log.
Neither collection nor managed long-term retention is implemented here. A copied
workspace can resolve coordinates against its new root; absolute URLs remain
specific to the originating filesystem and do not relocate automatically.

### Upgrade and rollback

New scans no longer write normalized per-target summaries or refresh fixed-path
output aliases. Existing summaries, artifacts and historical logs remain untouched
and are not migrated; old summaries are not current results. Previously overwritten
evidence cannot be recovered. Older binaries resume overwrite behavior after
rollback, while retained evaluation directories remain readable by authorized users.

See the [OPA man page](../../docs/man/complyctl-provider-opa.md#evidence) for the
evidence contract and [input-safety guidance](../../docs/man/complyctl-provider-opa.md#selecting-scan-input),
and the [shared evidence workflow](../../README.md#preserve-and-verify-scan-evidence)
for workspace discovery and log/file transfer guidance.

## Installation

### Prerequisites

- **Go** version 1.26 or higher
- **conftest** CLI ([releases](https://github.com/open-policy-agent/conftest/releases))
- **git** CLI tool
- Access to an OCI registry hosting OPA policy bundles

### Installing conftest

Install conftest from its [releases](https://github.com/open-policy-agent/conftest/releases)
or build it from source, and ensure it is on `PATH`. The provider does not require an exact
version string or embedded module metadata. Real-tool contracts exercise input isolation and
result handling; a source build reporting `dev` is not rejected for that reason.
See the [OPA man page](../../docs/man/complyctl-provider-opa.md#external-tools) for tested versions
and the limits of compatibility coverage.

### Build

```bash
make build-opa-provider
```

This produces `bin/complyctl-provider-opa`.

### Plugin Registration

After building, register the plugin with complyctl by placing the binary in the providers directory:

```bash
mkdir -p ~/.local/share/complytime/providers
cp bin/complyctl-provider-opa ~/.local/share/complytime/providers/
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
| `required tools not found: conftest` | Conftest is missing or not on `PATH` | Install conftest and put it on `PATH` |
| `required tools not found: git` | git not installed or not on PATH | Install git |
| One branch errors and the target has no assessments | An input-safety or evaluated-input rejection invalidates all branches of that configured target | Fix the rejected branch's input or select only the intended eligible branches; other configured targets retain their results |
| `is inside workspace` | The selected local file or directory is reserved workspace content | Copy the intended input outside both workspace boundaries and select the copy |
| `resolves to workspace` or `shares filesystem identity with workspace` | A symlink or hard link reaches workspace content | Remove the alias or copy the intended input outside the workspace; do not use a hard link |
| `no files found`, `no eligible input could be confirmed`, or `no eligible input was confirmed` | Local or remote directory input is empty, workspace-only, or fully ignored | Add a supported input file or adjust the effective ignore rule |
| `may define an ignore rule` | A local directory overlaps a workspace and `CONFTEST_IGNORE` is empty or unset | Select a non-overlapping target or set `CONFTEST_IGNORE` to the existing effective ignore rule; use `^$` only if the file defines none |
| `cannot determine conftest configuration` | The provider cannot establish whether a conftest configuration file applies | Check the configuration path and access, select a non-overlapping target, or set `CONFTEST_IGNORE` to the effective rule |
| `invalid CONFTEST_IGNORE expression` | The environment variable contains an invalid regular expression | Correct `CONFTEST_IGNORE` to a valid expression that preserves the intended ignore rule |
| `cannot establish remote clone root` | `scan_path` does not reconcile with the cloned input path | Set `scan_path` to a subdirectory of the cloned repository that matches the cloned input path |
| `directory symlink cycle` or `resolving input path` | An input or remote clone contains a cyclic or unresolved symlink | Repair or remove the link, or select input outside the workspace |
| `cannot establish workspace boundaries` | A configured workspace path cannot be resolved | Check that workspace paths exist and are accessible, then retry |
| `either run Generate first or set opa_bundle_ref variable` | No generated policy configuration or `opa_bundle_ref` is available | Run Generate or add `opa_bundle_ref` to the target's variables in `complytime.yaml` |
| `must use HTTPS scheme` | Target URL uses `http://` or `ssh://` | Change the URL to use `https://` |
| `specify either url or input_path, not both` | Both `url` and `input_path` set on the same target | Use one or the other per target |
| `branch name contains path traversal` | Branch name contains `..` | Use a valid branch name |
| `contains directory traversal` | `input_path` contains `..` | Use a path without traversal sequences |
| `pulling policy bundle` | OCI registry authentication failure | Run `docker login <registry>` before scanning |
| `cloning repository` | Git clone failed (auth, network, or bad URL) | Verify the URL and access token; check network connectivity |

For workspace boundaries, remote-clone checks, traversal, and concurrent-change limitations, see the [OPA man page](../../docs/man/complyctl-provider-opa.md#selecting-scan-input).
