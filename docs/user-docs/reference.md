# Reference: OPA Provider

Complete, precise description of the OPA provider's interface. For task-oriented
instructions, see the [How-to Guide](how-to.md).

## Provider Identity

| Field | Value |
|:------|:------|
| Plugin name | `opa` |
| Binary name | `complyctl-provider-opa` |
| Version | `0.1.0` |
| Interface | `provider.Provider` (Describe, Generate, Scan) |
| Framework | `hashicorp/go-plugin` (gRPC subprocess) |

## RPCs

### Describe

Returns provider metadata and health status.

**Request:** `*provider.DescribeRequest` (no fields used)

**Response:**

| Field | Type | Description |
|:------|:-----|:------------|
| `Healthy` | `bool` | `true` if `conftest` and `git` are on PATH |
| `Version` | `string` | `"0.1.0"` |
| `ErrorMessage` | `string` | Missing tools list, or empty |
| `RequiredTargetVariables` | `[]string` | `["url", "input_path"]` |

### Generate

Stub. Always returns `{Success: true}`.

### Scan

Evaluates configuration files against OPA policies.

**Request:** `*provider.ScanRequest` with at least one `Target`.

**Response:** `*provider.ScanResponse` containing `AssessmentLog` entries.

## Target Variables

### Required (one of)

| Variable | Type | Description |
|:---------|:-----|:------------|
| `url` | string | HTTPS URL of a git repository. Mutually exclusive with `input_path`. |
| `input_path` | string | Absolute path to a local file or directory. Mutually exclusive with `url`. |

### Required (global)

| Variable | Type | Description |
|:---------|:-----|:------------|
| `opa_bundle_ref` | string | OCI reference for the policy bundle (e.g., `ghcr.io/org/bundle:v1`). Must be set on at least one target. |

### Optional

| Variable | Type | Default | Description |
|:---------|:-----|:--------|:------------|
| `branches` | string | `"main"` | Comma-separated branch names for remote targets. |
| `access_token` | string | (none) | Auth token for private repos. Injected as env var. |
| `scan_path` | string | (none) | Subdirectory to scan within a cloned repository. |

## Workspace Layout

All artifacts are stored under `<complyctl-workspace>/opa/`.

| Path | Purpose |
|:-----|:--------|
| `opa/policy/` | Downloaded Rego policy files from OCI bundle |
| `opa/repos/` | Shallow-cloned git repositories |
| `opa/results/` | Per-target scan result JSON files |

Default workspace: `~/.complytime/workspace/`.

## Result File Schema

Per-target result files are written to `opa/results/<target>[-<branch>].json`.

```json
{
  "target": "string",
  "branch": "string (optional)",
  "scanned_at": "ISO 8601 timestamp",
  "findings": [
    {
      "requirement_id": "string",
      "title": "string",
      "result": "fail",
      "reason": "string",
      "filename": "string"
    }
  ],
  "success_count": 0,
  "status": "scanned | error",
  "error": "string (optional)"
}
```

## Assessment Response Mapping

Findings are grouped by `requirement_id` into `provider.AssessmentLog` entries.

| Finding field | AssessmentLog field |
|:--------------|:--------------------|
| `requirement_id` | `RequirementID` |
| `reason` | `Step.Message` |
| `result: "fail"` | `Step.Result = ResultFailed` |
| `result: "pass"` | `Step.Result = ResultPassed` |
| target+branch | `Step.Name` (format: `target@branch`) |

All assessments use `ConfidenceLevelHigh`.

## Requirement ID Derivation

Requirement IDs are extracted from the conftest result's `metadata.query` field:

| Query value | Derived requirement ID |
|:------------|:-----------------------|
| `data.kubernetes.run_as_root.deny` | `kubernetes.run_as_root` |
| `data.docker.network_encryption.warn` | `docker.network_encryption` |
| `data.terraform.s3_encryption.violation` | `terraform.s3_encryption` |
| (no query metadata) | `unknown` |

The `data.` prefix and rule type suffix (`warn`, `deny`, `violation`) are
stripped.

## Required External Tools

| Tool | Version | Purpose |
|:-----|:--------|:--------|
| `conftest` | any | OCI bundle pull, policy evaluation |
| `git` | any | Repository cloning |

## Input Validation Rules

| Input | Rule | Error message |
|:------|:-----|:--------------|
| `url` + `input_path` | Mutually exclusive | `"specify either url or input_path, not both"` |
| Neither set | At least one required | `"url or input_path is required"` |
| `url` scheme | Must be `https://` | `"url must use HTTPS scheme"` |
| `branches` | No `..` sequences | `"branch name contains path traversal"` |
| `branches` | Match `^[a-zA-Z0-9._/-]+$` | `"branch name contains invalid characters"` |
| `scan_path` | No `..` sequences | `"scan_path contains path traversal"` |
| `access_token` | No `\n`, `\r`, `\x00` | `"access_token contains invalid characters"` |
| `input_path` | No `..` sequences | `"input path contains directory traversal"` |
| `input_path` | Must exist on disk | `"input path does not exist"` |
| `opa_bundle_ref` | Must be set on at least one target | `"opa_bundle_ref variable is required but not set"` |

## File Permissions

| Artifact | Mode |
|:---------|:-----|
| Workspace directories | `0750` |
| Result JSON files | `0600` |

## Platform Detection

| URL hostname contains | Token env var | Git auth |
|:----------------------|:--------------|:---------|
| `github` | `GITHUB_TOKEN` | Token in env |
| `gitlab` | `GITLAB_TOKEN` | Token in env |
| other | `GITHUB_TOKEN` (default) | Token in env |

`GIT_TERMINAL_PROMPT=0` is always set when a token is provided.

## Error Behavior

| Error type | Scan continues? | Appears in |
|:-----------|:----------------|:-----------|
| No targets | No | `Scan()` error return |
| Missing `opa_bundle_ref` | No | `Scan()` error return |
| Missing tools | No | `Scan()` error return |
| Bundle pull failure | No | `Scan()` error return |
| Per-target validation | Yes | Result with `status: "error"` |
| Git clone failure | Yes | Result with `status: "error"` |
| Policy eval failure | Yes | Result with `status: "error"` |
| Parse failure | Yes | Result with `status: "error"` |

## Conftest Flags

The provider invokes conftest with these fixed flags:

| Command | Flags |
|:--------|:------|
| `conftest pull` | `oci://<ref> --policy <dir>` |
| `conftest test` | `<path> --policy <dir> --output json --all-namespaces --no-fail` |
