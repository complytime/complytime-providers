# OPA Provider Plugin — Technical Specification

> **Date**: 2026-05-09 | **Author**: Claude Code Interview
> **Status**: Ready for implementation
> **Implementation method**: TDD

---

## 1. Overview

Build a new `complyctl-provider-opa` plugin binary that implements the complyctl gRPC
`Provider` interface (Describe, Generate, Scan) using Open Policy Agent via `conftest`
as the policy evaluation engine. The provider pulls OPA policy bundles from OCI
registries and evaluates configuration files (Kubernetes manifests, Terraform HCL,
Dockerfiles, CI YAML, Ansible playbooks) against them. It supports both local file
paths and remote git repositories as input sources.

The Generate phase is deferred as a follow-up item. This spec covers Describe and Scan,
with Generate as a pass-through stub returning success.

## 2. Context

The ComplyTime organization ships compliance-scanning provider plugins for the `complyctl`
CLI. Two providers exist today: `openscap-provider` (XCCDF-based system scanning) and
`ampel-provider` (in-toto attestation-based policy verification). A third provider is
needed to evaluate configuration files against OPA/Rego policies using `conftest`.

The OPA policy bundles are built and published to OCI registries by a separate repository
(`opa-policies-poc`). That repo generates platform-specific bundles (CI, Kubernetes,
Terraform, Docker, Ansible) from CUE-defined compliance frameworks (NIST 800-53,
CIS, and organization-specific standards) and Rego templates. Each bundle contains Rego policies with METADATA
blocks that declare which compliance control IDs they enforce.

The OPA provider bridges complyctl's assessment model with conftest's policy evaluation:
complyctl tells the provider what to scan (targets), the provider pulls the right policy
bundle and runs conftest against the target's configuration files, then maps conftest's
structured output back to complyctl's `AssessmentLog` format using control IDs from Rego
METADATA blocks.

### Why not reuse ampel?

Ampel's architecture is attestation-centric: snappy collects in-toto attestations, ampel
verifies them against JSON policy bundles. OPA/conftest evaluates configuration files
directly against Rego policies — no attestations, no DSSE, no snappy. The data flow is
fundamentally different. A dedicated provider keeps both implementations clean and
focused (Constitution Principle II: Simplicity & Isolation).

## 3. References & Documentation Consulted

| Source | Type | Relevance |
|--------|------|-----------|
| `cmd/ampel-provider/` (all subpackages) | LOCAL | Reference implementation — same plugin pattern |
| `cmd/openscap-provider/server/server.go` | LOCAL | Second reference for Provider interface usage |
| `vendor/github.com/complytime/complyctl/pkg/provider/` | LOCAL | Provider interface contracts and type definitions |
| `docs/provider-guide.md` | LOCAL | Provider development guide (manifest, entry point) |
| `opa-policies-poc/` (local clone) | LOCAL | OPA policy bundle structure, Rego output format, manifest schema |
| [SecurityCon workflow](https://github.com/jpower432/opensource-securitycon-2025-oscal-in-action/blob/cc0facf/.github/workflows/collect.yml) | WEB | Reference workflow showing conftest pull + test pattern |

## 4. Architecture & Design

### 4.1 Plugin Pattern

Follows the identical pattern as ampel and openscap:

```
main.go → server.New() → provider.Serve()
```

The complyctl framework manages gRPC subprocess lifecycle via hashicorp/go-plugin.
The provider is self-contained under `cmd/opa-provider/` with its own subpackage
hierarchy. No shared library code with other providers.

### 4.2 Package Structure

```text
cmd/opa-provider/
├── main.go                  # Entry point
├── config/
│   ├── config.go            # Workspace directory management
│   └── config_test.go
├── scan/
│   ├── scan.go              # Scan orchestration (git clone, conftest)
│   └── scan_test.go
├── results/
│   ├── results.go           # Parse conftest JSON → complyctl AssessmentLog
│   └── results_test.go
├── targets/
│   ├── targets.go           # URL parsing, path validation, repo display
│   └── targets_test.go
├── toolcheck/
│   ├── toolcheck.go         # Check conftest + git on PATH
│   └── toolcheck_test.go
└── server/
    ├── server.go            # gRPC Provider implementation
    └── server_test.go
```

### 4.3 Data Flow

```
complyctl (ScanRequest with targets)
  ↓
server.Scan()
  ├─ validate targets, check tools, ensure directories
  ├─ conftest pull oci://{bundle_ref} → {workspace}/opa/policy/
  ├─ for each target:
  │   ├─ if remote URL:
  │   │   ├─ git clone --branch {branch} --depth 1 {url} → {workspace}/opa/repos/{sanitized}/
  │   │   └─ resolve scan dir: {clone_dir}/{scan_path} or {clone_dir}
  │   ├─ if local path:
  │   │   └─ validate path exists, no traversal
  │   ├─ conftest test {input} --policy {policy_dir} --output json --all-namespaces --no-fail
  │   ├─ parse conftest JSON → PerTargetResult with Findings
  │   └─ log errors but continue (graceful degradation)
  ├─ aggregate results → ToScanResponse()
  └─ return ScanResponse with assessments
```

### 4.4 Design Decisions

| # | Decision | Chosen | Alternatives Considered | Rationale |
|---|----------|--------|------------------------|-----------|
| D1 | Provider name | `opa-provider` (binary: `complyctl-provider-opa`) | `conftest-provider` | OPA is the broader ecosystem brand. The provider could evolve to support other OPA-based tools beyond conftest. Naming after the ecosystem, not the CLI tool. |
| D2 | Policy source | OPA bundles from OCI registry via single `opa_bundle_ref` global variable | Per-target bundle refs, pre-built local bundles | Single bundle per assessment is simplest for v1. The policy bundle applies to all targets in an assessment. Can expand to per-target bundles later. |
| D3 | Evaluation tool | `conftest` CLI via `CommandRunner` interface | `opa eval` CLI, OPA as Go library (`github.com/open-policy-agent/opa/rego`) | conftest natively handles multi-format inputs (YAML, HCL, Dockerfile, JSON), OCI bundle pulling, and structured JSON output. Using CLI via CommandRunner matches ampel's proven pattern and enables test mocking. OPA Go library would tightly couple the provider to OPA internals. |
| D4 | Data collection | No snappy — conftest evaluates config files directly | Snappy-based collection (like ampel) | OPA/Rego policies evaluate configuration files directly — Kubernetes manifests, Terraform HCL, Dockerfiles, CI YAML. No attestation layer needed. Simpler architecture than ampel. |
| D5 | Input sources | Both local paths (`input_path`) and remote git repos (`url`) | Local-only, remote-only | Supports CI pipelines (local files in workspace), GitHub Actions (remote repos), and developer workstations (either). Composable per Unix philosophy. |
| D6 | Git authentication | Optional `access_token`, unauthenticated if not provided | Always require token | Supports public repos (GitHub/GitLab public projects), internal repos behind VPN (corporate GitLab accessible without tokens on same network), and authenticated private repos. Unauthenticated clone attempt with clear error message on failure covers all cases. |
| D7 | Git clone method | Shell out to `git` CLI via `CommandRunner` | `go-git` library for in-process cloning | Consistent with external-tool pattern used throughout the project. Git CLI handles credential helpers, SSH config, proxy settings, and partial clones natively. go-git would add a large dependency for marginal benefit. |
| D8 | Cloned repo cleanup | Keep in workspace (`{workspace}/opa/repos/`) | Delete after scan | Allows inspection and debugging of scanned files. Consistent with ampel keeping attestation files in workspace. Users can clean up manually or via complyctl workspace commands. |
| D9 | OCI registry auth | Standard Docker config (`~/.docker/config.json` / `DOCKER_CONFIG` env var) | Custom `OPA_REGISTRY_TOKEN` env var | conftest respects Docker credential helpers natively — zero custom code needed. Standard Docker auth works with GitHub Container Registry, GitLab Container Registry, Quay.io, ECR, GCR, and any OCI-compliant registry. |
| D10 | Conftest namespace | `--all-namespaces` flag | Explicit `--namespace {platform}`, auto-detect from manifest | Bundles are platform-specific and contain only relevant policies. `--all-namespaces` ensures all packages are evaluated without needing platform detection logic. No risk of evaluating wrong policies since the bundle already scopes them. Simplest correct approach. |
| D11 | Bundle manifest | Download and parse `.manifest.json` alongside bundle | Ignore manifest | Manifest provides platform, version, control count, and bundle SHA256 for logging and health checks. Minimal code cost for significant observability benefit. |
| D12 | Result mapping | `warn` → `ResultFailed`, `deny` → `ResultFailed`, `successes` → `ResultPassed` | `warn` → softer result, only `deny` → `ResultFailed` | Both warn and deny represent compliance violations that must be reported. Current Rego policies use `warn` rules exclusively. Mapping both to `ResultFailed` is future-proof for when `deny` rules are added. Consistent treatment avoids confusion. |
| D13 | Error vs violation | Violations → `ResultFailed`, conftest errors → `ResultError` | Treat all as failures | Same pattern as ampel. Distinguishes policy violations (actionable by the user — fix the config) from tool failures (actionable by the operator — fix the toolchain). Different remediation paths warrant different result types. |
| D14 | Generate phase | Deferred — stub returns success | Full implementation | Generate's role for OPA is unclear — policies come from OCI bundles, not generated from assessment configs. Needs further design after v1 Scan is working. Stub avoids blocking the useful work (Scan) on an uncertain design. |
| D15 | Results format | Parse conftest standard `--output json` | Custom conftest output plugin, parse text output | conftest's JSON output includes `successes`, `failures`, `warnings` arrays with `msg` and `metadata` fields. The metadata contains control IDs from Rego METADATA blocks. Standard format, well-documented, stable across versions. |
| D16 | Requirement ID mapping | Control IDs from conftest metadata → complyctl `RequirementID` | Parse control ID from violation message string, use policy package name | Rego METADATA blocks declare `controls: [ITSS-CH1-ACCESS-005, ...]`. conftest exposes these in structured JSON output metadata. Clean, reliable, and decoupled from message formatting. Falls back to package-derived ID when metadata is absent. |
| D17 | Scan path for remote repos | Optional `scan_path` target variable, defaults to repo root | Scan entire repo always, auto-detect from bundle platform | Gives user explicit control over which subdirectory to scan (e.g., `k8s/` for Kubernetes manifests, `.github/workflows/` for CI configs). Default to root for simple cases. Explicit is better than magical auto-detection. |
| D18 | Multiple input files per target | One `input_path` per target, can be a directory | Multiple comma-separated paths | conftest already recurses directories and handles multiple files. One path per target is simple and composable — use multiple targets for multiple paths. Avoids CSV parsing complexity. |
| D19 | Version | `0.1.0` | Higher version | Consistent with openscap and ampel providers, both at `0.1.0`. Signals alpha maturity appropriately. |
| D20 | Required tools | `conftest` + `git` | `conftest` + `oras`, just `conftest` | conftest handles OCI bundle operations natively. `git` is needed for remote repo cloning. `oras` is unnecessary since conftest has built-in OCI support. |

## 5. Detailed Implementation

### 5.1 Entry Point

**File**: `cmd/opa-provider/main.go` | **Action**: CREATE

```go
// SPDX-License-Identifier: Apache-2.0

package main

import (
    "github.com/complytime/complytime-providers/cmd/opa-provider/server"
    "github.com/complytime/complyctl/pkg/provider"
)

func main() {
    opaProvider := server.New()
    provider.Serve(opaProvider)
}
```

### 5.2 Server Package — gRPC Provider Implementation

**File**: `cmd/opa-provider/server/server.go` | **Action**: CREATE

**Types:**
- `ProviderServer` struct — implements `provider.Provider`
- Global `ScanRunner` variable of type `scan.CommandRunner` (injectable for tests, defaults to `scan.ExecRunner{}`)
- Global `SkipToolCheck` bool (test-only flag)

**Functions:**

- `New() *ProviderServer` — constructor returning empty struct
- `Describe(ctx, req) (*DescribeResponse, error)` — health check + variable declarations
- `Generate(ctx, req) (*GenerateResponse, error)` — stub returning success
- `Scan(ctx, req) (*ScanResponse, error)` — main scan orchestration
- `validateTargetVariables(url, inputPath, branches, scanPath, accessToken string) error` — defense-in-depth validation
- `splitCSV(s string) []string` — split comma-separated values
- `checkRequiredTools(logger) error` — tool availability check

**Scan logic:**
1. Validate at least one target
2. Check tools (conftest, git)
3. Ensure workspace directories
4. Extract `opa_bundle_ref` from `req.Targets[0].Variables` or global variables
5. Pull bundle: `conftest pull oci://{ref} --policy {policyDir}`
6. For each target:
   - Extract variables: `url`, `input_path`, `branches`, `access_token`, `platform`, `scan_path`
   - Validate: exactly one of `url` or `input_path` must be set
   - If `url`: for each branch, clone repo and scan
   - If `input_path`: scan directly
   - Run `conftest test` and parse results
   - On per-target error: log and continue
7. Aggregate results via `results.ToScanResponse()`

### 5.3 Config Package — Workspace Directories

**File**: `cmd/opa-provider/config/config.go` | **Action**: CREATE

**Constants:**
- `ProviderDir = "opa"`
- `PolicyDir = "policy"`
- `ReposDir = "repos"`
- `ResultsDir = "results"`

**Functions:**
- `NewConfig() *Config`
- `opaDir() string` — returns `{workspace}/opa`
- `PolicyDirPath() string` — returns `{workspace}/opa/policy`
- `ReposDirPath() string` — returns `{workspace}/opa/repos`
- `ResultsDirPath() string` — returns `{workspace}/opa/results`
- `EnsureDirectories() error` — creates all directories with mode 0750

### 5.4 Scan Package — Scan Orchestration

**File**: `cmd/opa-provider/scan/scan.go` | **Action**: CREATE

**Types:**
- `RepoTarget` struct: `URL`, `AccessToken` (tagged `json:"-"`), `Platform`
- `ScanConfig` struct: `PolicyPath`, `OutputDir`
- `RawScanResult` struct: `Output []byte`
- `CommandRunner` interface: `Run(name, args...) ([]byte, error)`, `RunWithEnv(env[], name, args...) ([]byte, error)`
- `ExecRunner` struct implementing `CommandRunner` via `os/exec`

**Functions:**
- `PullBundle(bundleRef, policyDir string, runner CommandRunner) error`
  - Constructs: `conftest pull oci://{bundleRef} --policy {policyDir}`
  - Returns error on failure with conftest stderr
- `CloneRepository(repo RepoTarget, branch, cloneDir string, runner CommandRunner) error`
  - Constructs: `git clone --branch {branch} --depth 1 {url} {cloneDir}`
  - If `AccessToken` provided: injects via environment (`GIT_ASKPASS` or URL-embedded token)
  - Returns clear error message on failure
- `EvalPolicy(inputPath, policyDir string, runner CommandRunner) (*RawScanResult, error)`
  - Constructs: `conftest test {inputPath} --policy {policyDir} --output json --all-namespaces --no-fail`
  - Returns raw JSON output
- `constructConftestPullCommand(bundleRef, policyDir string) (string, []string)`
- `constructConftestTestCommand(inputPath, policyDir string) (string, []string)`
- `constructGitCloneCommand(url, branch, cloneDir string) (string, []string)`
- `buildTokenEnv(repo RepoTarget) []string` — builds environment with platform-specific token

### 5.5 Results Package — Conftest Output Parsing

**File**: `cmd/opa-provider/results/results.go` | **Action**: CREATE

**Constants:**
- `maxFieldSize = 10 * 1024` (10KB field size limit)

**Conftest output types (internal):**
```go
type conftestFileResult struct {
    Filename  string           `json:"filename"`
    Successes []conftestCheck  `json:"successes"`
    Failures  []conftestCheck  `json:"failures"`
    Warnings  []conftestCheck  `json:"warnings"`
}

type conftestCheck struct {
    Msg      string                 `json:"msg"`
    Metadata map[string]interface{} `json:"metadata"`
}
```

**Public types:**
- `PerTargetResult` struct: `Target`, `Branch`, `ScannedAt`, `Findings []Finding`, `Status`, `Error`
- `Finding` struct: `ControlID`, `Title`, `Result` ("pass"/"fail"), `Reason`

**Functions:**
- `ParseConftestOutput(raw []byte, target, branch string) (*PerTargetResult, error)`
  - Unmarshals conftest JSON (array of `conftestFileResult`)
  - For each file result:
    - Successes → `Finding{Result: "pass"}`
    - Failures → `Finding{Result: "fail"}`
    - Warnings → `Finding{Result: "fail"}` (D12: both warn and deny → failed)
  - Extracts control IDs from metadata `controls` field
  - If multiple control IDs per check, fans out to one Finding per control ID
  - Falls back to package-derived ID if no metadata
  - Validates field sizes, strips control characters
  - Sets overall status based on presence of failures
- `WritePerTargetResult(result *PerTargetResult, dir string) error`
  - Writes JSON to `{dir}/{sanitized-target}.json` with mode 0600
- `ToScanResponse(results []*PerTargetResult) *provider.ScanResponse`
  - Groups findings by control ID (requirement ID)
  - For each requirement: creates `AssessmentLog` with steps from all targets
  - Counts pass/total, sets message
  - Error targets with no findings → error step under "scan-error" requirement
  - Returns deterministic-order response

**Helpers:**
- `extractControlIDs(metadata map[string]interface{}) []string`
- `stripControlChars(s string) string`
- `isPrintableASCII(s string) bool`
- `mapResult(findingResult string) provider.Result`

### 5.6 Targets Package — URL Parsing & Validation

**File**: `cmd/opa-provider/targets/targets.go` | **Action**: CREATE

**Functions:**
- `ParseRepoURL(repoURL, platformHint string) (platform, org, repo string, err error)`
  - HTTPS-only validation
  - Platform detection from hostname (github.com, gitlab.com) or hint
  - GitHub: first two path segments
  - GitLab: nested group support
- `SanitizeRepoURL(repoURL string) string`
  - Strips scheme, replaces `/`, `.`, `:` with `-`
- `RepoDisplayName(repoURL string) string`
  - Returns `{org}/{repo}` for human output
- `ValidateInputPath(inputPath string) error`
  - Checks path exists
  - Rejects `..` traversal
  - Validates within expected boundaries

### 5.7 Toolcheck Package

**File**: `cmd/opa-provider/toolcheck/toolcheck.go` | **Action**: CREATE

**Constants:**
- `RequiredTools = []string{"conftest", "git"}`

**Functions:**
- `CheckTools() ([]string, error)` — checks each tool via `exec.LookPath()`
- `FormatMissingToolsError(missing []string) error` — formats user-facing error

### 5.8 Makefile Update

**File**: `Makefile` | **Action**: MODIFY

Add:
```makefile
build-opa-provider:
	go build -o $(BINARY_DIR)/complyctl-provider-opa ./cmd/opa-provider
```

Update `build` target to include `build-opa-provider`.
Update `.PHONY` to include `build-opa-provider`.

### 5.9 Provider Guide Update

**File**: `docs/provider-guide.md` | **Action**: MODIFY

Add OPA provider to the providers table:

| Provider | Binary | Description |
|:---|:---|:---|
| `cmd/opa-provider` | `complyctl-provider-opa` | OPA/conftest-based configuration policy evaluation |

## 6. Error Handling

| Error scenario | Detection | Response | Test |
|---------------|-----------|----------|------|
| No targets provided | `len(req.Targets) == 0` | Return error: "at least one target is required" | `TestScan_NoTargets` |
| Missing conftest on PATH | `exec.LookPath("conftest")` fails | Describe: `Healthy: false`; Scan: error listing missing tools | `TestDescribe_MissingTools` |
| Missing git on PATH | `exec.LookPath("git")` fails | Describe: `Healthy: false`; Scan: error listing missing tools | `TestDescribe_MissingTools` |
| Missing `opa_bundle_ref` | Global variable not set | Return error: "opa_bundle_ref global variable is required" | `TestScan_MissingBundleRef` |
| Invalid OCI bundle ref | conftest pull exits non-zero | Return error with conftest stderr | `TestScan_ConftestPullFailure` |
| OCI auth failure | conftest pull exits non-zero | Return error suggesting Docker config check | `TestScan_ConftestPullFailure` |
| Target has neither url nor input_path | Both variables empty | Error for that target, continue others | `TestScan_NeitherURLNorInputPath` |
| Target has both url and input_path | Both variables set | Error: "specify either url or input_path, not both" | `TestScan_BothURLAndInputPath` |
| URL not HTTPS | URL scheme != https | Error: "only HTTPS URLs are supported" | `TestScan_InvalidURL` |
| Path traversal in input_path | Contains `..` | Error: "path traversal not allowed" | `TestScan_PathTraversal` |
| Path traversal in branch name | Contains `..` | Error: "invalid branch name" | `TestScan_PathTraversal` |
| Unauthenticated clone fails | git clone exits non-zero | Error: "clone failed — if repo is private, provide access_token" | `TestScan_GitCloneFailure` |
| conftest test exits non-zero | Non-zero exit (shouldn't with --no-fail) | Capture stderr, return `ResultError` step | `TestScan_ConftestTestFailure` |
| Conftest JSON parse failure | json.Unmarshal error | Return `ResultError` step with parse error | `TestParseConftestOutput_InvalidJSON` |
| No metadata in Rego policy | metadata field absent | Fall back to package-derived requirement ID | `TestParseConftestOutput_NoMetadata` |
| Multiple control IDs per policy | metadata.controls has multiple entries | Fan out to one Finding per control ID | `TestParseConftestOutput_MultipleControlIDs` |
| Field exceeds 10KB | len(field) > maxFieldSize | Truncate with `[truncated]` marker | `TestParseConftestOutput_FieldSizeLimit` |
| Non-printable chars in IDs | !isPrintableASCII(id) | Strip control characters | `TestParseConftestOutput_FieldSizeLimit` |
| conftest finds no results | Empty successes/failures/warnings | Empty assessment for target (no violations = pass) | `TestParseConftestOutput_EmptyInput` |
| Local input_path doesn't exist | os.Stat fails | Error for that target, continue others | `TestScan_LocalPath_NotFound` |

## 7. Concurrency

Not applicable for v1. Targets are processed sequentially within the Scan RPC (same as
ampel). The gRPC framework handles concurrent request isolation at the subprocess level
via hashicorp/go-plugin. No mutexes, channels, or goroutines needed in provider code.

## 8. Configuration

### 8.1 Workspace Directories

All artifacts stored under `{WorkspaceDir}/opa/`:

| Directory | Purpose | Created by |
|-----------|---------|------------|
| `{workspace}/opa/policy/` | Pulled conftest policy bundle | `config.EnsureDirectories()` |
| `{workspace}/opa/repos/` | Cloned remote repositories | `config.EnsureDirectories()` |
| `{workspace}/opa/results/` | Conftest output JSON files | `config.EnsureDirectories()` |

Directory permissions: 0750 (rwxr-x---).

### 8.2 Global Variables

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `opa_bundle_ref` | Yes | OCI reference for the policy bundle | `ghcr.io/org/policy-bundle:dev` |

### 8.3 Target Variables

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `url` | One of url/input_path | HTTPS git repository URL (remote scanning) | `https://github.com/org/repo` |
| `input_path` | One of url/input_path | Local path to config files/directory | `/path/to/k8s-manifests/` |
| `branches` | No (default: "main") | CSV of branches to scan (remote only) | `main,develop` |
| `access_token` | No | Git auth token for private repos (remote only) | `ghp_xxxx` |
| `platform` | No | Platform hint for URL parsing (remote only) | `github`, `gitlab` |
| `scan_path` | No (default: repo root) | Subdirectory within repo to scan (remote only) | `k8s/`, `.github/workflows/` |

### 8.4 Validation Rules

- `url`: HTTPS only, valid URL structure, safe characters
- `input_path`: must exist, no `..` traversal, no absolute path escape
- `branches`: safe characters (`[a-zA-Z0-9._/-]`), no `..`
- `access_token`: no newlines or null bytes (never logged)
- `scan_path`: no `..` traversal, relative path only
- Exactly one of `url` or `input_path` must be provided per target

## 9. Test Plan (TDD)

### 9.1 Unit Tests

Write test FIRST, then implementation:

| # | Test name | Description | Assertions |
|---|-----------|-------------|------------|
| 1 | `TestNew_ReturnsProviderServer` | Constructor returns valid server | `require.NotNil(t, server.New())` |
| 2 | `TestDescribe_Healthy` | Returns healthy with version and variables | `assert.True(t, resp.Healthy)`, `assert.Equal(t, "0.1.0", resp.Version)`, `assert.Contains(t, resp.RequiredTargetVariables, "url")` |
| 3 | `TestDescribe_MissingTools` | Returns unhealthy when conftest/git missing | `assert.False(t, resp.Healthy)`, `assert.Contains(t, resp.ErrorMessage, "conftest")` |
| 4 | `TestGenerate_ReturnsSuccess` | Stub returns success | `assert.True(t, resp.Success)` |
| 5 | `TestScan_LocalPath_HappyPath` | Scans local dir, parses results | Mock runner returns conftest JSON, verify AssessmentLog entries |
| 6 | `TestScan_RemoteURL_HappyPath` | Clones repo, scans, parses results | Mock runner captures git clone + conftest commands, verify results |
| 7 | `TestScan_RemoteURL_WithBranches` | Scans multiple branches | Two branches → two clone+scan invocations, results aggregated |
| 8 | `TestScan_RemoteURL_WithScanPath` | Scans subdirectory within clone | Verify conftest test path includes scan_path |
| 9 | `TestScan_RemoteURL_WithAccessToken` | Injects token for authenticated clone | Verify env contains token, token not in logs |
| 10 | `TestScan_RemoteURL_UnauthenticatedClone` | Clones without token | Verify no token in env |
| 11 | `TestScan_NoTargets` | Returns error | `assert.Error`, message contains "at least one target" |
| 12 | `TestScan_MissingBundleRef` | Returns error for missing global var | `assert.Error`, message contains "opa_bundle_ref" |
| 13 | `TestScan_BothURLAndInputPath` | Returns error | Message contains "specify either url or input_path" |
| 14 | `TestScan_NeitherURLNorInputPath` | Returns error | Message contains "url or input_path" |
| 15 | `TestScan_InvalidURL` | Rejects non-HTTPS URLs | `assert.Error`, message contains "HTTPS" |
| 16 | `TestScan_PathTraversal` | Rejects `..` in paths and branches | `assert.Error` for each case |
| 17 | `TestScan_ConftestPullFailure` | Handles bundle pull error | Error propagated, scan does not proceed |
| 18 | `TestScan_ConftestTestFailure` | Handles evaluation error | `ResultError` step in response |
| 19 | `TestScan_GitCloneFailure` | Handles clone failure | Error message suggests access_token |
| 20 | `TestScan_MultipleTargets_PartialFailure` | Continues on per-target failure | First target fails, second succeeds, both in response |
| 21 | `TestParseConftestOutput_HappyPath` | Parses successes, failures, warnings | Correct Finding count and Result values |
| 22 | `TestParseConftestOutput_WithMetadata` | Extracts control IDs from metadata | ControlID matches metadata.controls |
| 23 | `TestParseConftestOutput_NoMetadata` | Falls back to package-derived ID | ControlID derived from package name |
| 24 | `TestParseConftestOutput_MultipleControlIDs` | Fans out to multiple findings | One check with 2 controls → 2 Findings |
| 25 | `TestParseConftestOutput_EmptyInput` | Handles empty/nil input | Returns error |
| 26 | `TestParseConftestOutput_InvalidJSON` | Returns parse error | `assert.Error` |
| 27 | `TestParseConftestOutput_FieldSizeLimit` | Truncates oversized fields | Field ends with `[truncated]` |
| 28 | `TestToScanResponse_Aggregation` | Groups findings by requirement ID | Multiple targets → grouped AssessmentLog entries |
| 29 | `TestToScanResponse_WarnAndDeny` | Both map to ResultFailed | Both types produce `ResultFailed` steps |
| 30 | `TestParseRepoURL_GitHub` | Extracts org/repo from GitHub URL | `platform == "github"`, correct org/repo |
| 31 | `TestParseRepoURL_GitLab` | Extracts group/project from GitLab URL | `platform == "gitlab"`, nested groups |
| 32 | `TestParseRepoURL_InvalidScheme` | Rejects non-HTTPS | `assert.Error` |
| 33 | `TestSanitizeRepoURL` | Produces filesystem-safe strings | No `/`, `.`, `:` in output |
| 34 | `TestValidateInputPath_Valid` | Accepts valid local paths | `assert.NoError` |
| 35 | `TestValidateInputPath_Traversal` | Rejects `..` paths | `assert.Error` |
| 36 | `TestCheckTools_AllPresent` | Returns empty list when all present | `assert.Empty(t, missing)` |
| 37 | `TestCheckTools_Missing` | Returns missing tool names | `assert.Contains(t, missing, "conftest")` |
| 38 | `TestEnsureDirectories` | Creates workspace subdirs | All directories exist after call |
| 39 | `TestConstructConftestPullCommand` | Correct args for OCI pull | Verify command name and args slice |
| 40 | `TestConstructConftestTestCommand` | Correct args with --all-namespaces | Verify `--all-namespaces` and `--no-fail` present |
| 41 | `TestConstructGitCloneCommand` | Correct args with branch and depth | Verify `--branch`, `--depth 1` |

### 9.2 Integration Tests

| # | Test name | Description | Dependencies |
|---|-----------|-------------|--------------|
| 1 | `TestScan_EndToEnd_LocalPath` | Full scan of local fixtures | conftest on PATH, test fixture files |
| 2 | `TestScan_EndToEnd_RemoteRepo` | Full scan of public repo | conftest + git on PATH, network access |

Integration tests guarded with `testing.Short()` skip.

## 10. Acceptance Criteria

- [ ] AC-1: `complyctl-provider-opa` binary builds successfully via `make build-opa-provider`
- [ ] AC-2: Describe RPC returns healthy status, version `0.1.0`, correct required variables
- [ ] AC-3: Describe RPC returns unhealthy when `conftest` or `git` not on PATH
- [ ] AC-4: Generate RPC returns success (stub)
- [ ] AC-5: Scan evaluates local config files against OCI policy bundle via conftest
- [ ] AC-6: Scan clones remote git repos and evaluates config files within them
- [ ] AC-7: Scan supports authenticated and unauthenticated git clone
- [ ] AC-8: Scan supports multiple branches for remote targets
- [ ] AC-9: Scan supports `scan_path` for subdirectory scanning in remote repos
- [ ] AC-10: Conftest JSON output correctly mapped to complyctl AssessmentLog with control IDs as requirement IDs
- [ ] AC-11: Both `warn` and `deny` violations map to `ResultFailed`
- [ ] AC-12: Graceful degradation — per-target failures do not abort the entire scan
- [ ] AC-13: Input validation: HTTPS-only URLs, no path traversal, no invalid branch names
- [ ] AC-14: All tests pass with `go test -race -count=1 ./cmd/opa-provider/...`
- [ ] AC-15: `make lint` passes with zero issues
- [ ] AC-16: Code follows project conventions (SPDX headers, go-hclog, testify, goimports)
- [ ] AC-FINAL: Code compiles, vet clean, all tests pass

## 11. Implementation Order

1. Create `cmd/opa-provider/` directory structure
2. Create test file `toolcheck/toolcheck_test.go` with test stubs → implement `toolcheck/toolcheck.go`
3. Create test file `config/config_test.go` with test stubs → implement `config/config.go`
4. Create test file `targets/targets_test.go` with test stubs → implement `targets/targets.go`
5. Create test file `scan/scan_test.go` with test stubs → implement `scan/scan.go`
6. Create test file `results/results_test.go` with test stubs → implement `results/results.go`
7. Create test file `server/server_test.go` with test stubs → implement `server/server.go`
8. Create `main.go` entry point
9. Update `Makefile` with `build-opa-provider` target
10. Update `docs/provider-guide.md` with OPA provider entry
11. Run full test suite: `go test -race -count=1 ./cmd/opa-provider/...`
12. Run linter: `golangci-lint run ./cmd/opa-provider/...`
13. Verify build: `make build-opa-provider`
14. Verify all acceptance criteria

## 12. Follow-Up Items

- [ ] **Generate phase design**: Determine if/how Generate should prepare policy artifacts for OPA. Options include downloading bundles during Generate (not Scan), mapping complyctl requirement IDs to Rego rule selectors, or remaining a no-op.
- [ ] **Manifest JSON**: Create `c2p-opa-manifest.json` for complyctl discovery.
- [ ] **README**: Create `cmd/opa-provider/README.md` with setup and usage instructions.
- [ ] **AGENTS.md update**: Add OPA provider to project structure section.
- [ ] **CI workflow update**: Ensure CI builds and tests the OPA provider.
