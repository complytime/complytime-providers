# OPA Provider — Public API Reference

This document covers all exported types, functions, constants, and interfaces
in the `complyctl-provider-opa` plugin. The provider lives under
`cmd/opa-provider/` with six subpackages.

## Entry Point

**File:** `cmd/opa-provider/main.go`

```go
func main() {
    opaProvider := server.New()
    provider.Serve(opaProvider)
}
```

The binary is discovered by complyctl using the `complyctl-provider-` executable
prefix. The complyctl framework manages the gRPC subprocess lifecycle via
`hashicorp/go-plugin`.

---

## Package `server`

**File:** `cmd/opa-provider/server/server.go`

### Types

#### ProviderServer

```go
type ProviderServer struct{}
```

Implements the `provider.Provider` interface from
`github.com/complytime/complyctl/pkg/provider`. This is the core type that
complyctl interacts with over gRPC.

### Functions

#### New

```go
func New() *ProviderServer
```

Returns a new `ProviderServer` instance. Called once in `main()` and passed to
`provider.Serve()`.

#### Describe

```go
func (s *ProviderServer) Describe(
    ctx context.Context,
    req *provider.DescribeRequest,
) (*provider.DescribeResponse, error)
```

Reports provider metadata and health status. Checks that `conftest` and `git`
are available on the system PATH via the `toolcheck` package.

**Response fields:**

| Field | Type | Value |
|:------|:-----|:------|
| `Healthy` | `bool` | `true` if all required tools are found |
| `Version` | `string` | `"0.1.0"` |
| `ErrorMessage` | `string` | Lists missing tools when unhealthy |
| `RequiredTargetVariables` | `[]string` | `["url", "input_path"]` |

#### Generate

```go
func (s *ProviderServer) Generate(
    ctx context.Context,
    req *provider.GenerateRequest,
) (*provider.GenerateResponse, error)
```

Stub implementation. Always returns `&provider.GenerateResponse{Success: true}`.
The Generate phase is deferred to a future iteration.

#### Scan

```go
func (s *ProviderServer) Scan(
    ctx context.Context,
    req *provider.ScanRequest,
) (*provider.ScanResponse, error)
```

Evaluates configuration files against OPA policies using conftest. This is the
primary RPC.

**Preconditions:**

- At least one target must be provided (`req.Targets` non-empty)
- Required tools (`conftest`, `git`) must be available
- `opa_bundle_ref` variable must be set on at least one target

**Behavior:**

1. Validates required tools are present
2. Extracts `opa_bundle_ref` from the first target's variables
3. Creates workspace directories via `config.EnsureDirectories()`
4. Pulls the OPA policy bundle from the OCI registry
5. Iterates over each target:
   - **Remote targets** (have `url`): clones the git repository per branch,
     runs conftest against the cloned files
   - **Local targets** (have `input_path`): runs conftest directly against the
     local path
6. Aggregates all results into a `provider.ScanResponse`

**Error handling:** Per-target errors are captured in the results (scan
continues). Only global errors (no targets, missing bundle ref, tool check
failure, bundle pull failure) return an error from `Scan()` itself.

**Returns:** `*provider.ScanResponse` containing `AssessmentLog` entries grouped
by requirement ID.

### Package-Level Variables

```go
var ScanRunner scan.CommandRunner = scan.ExecRunner{}
var SkipToolCheck bool
var TestWorkspaceDir string
```

These variables exist for testing. `ScanRunner` allows injecting a mock command
runner. `SkipToolCheck` disables tool validation. `TestWorkspaceDir` overrides
the workspace path.

---

## Package `config`

**File:** `cmd/opa-provider/config/config.go`

### Constants

```go
const ProviderDir = "opa"
const PolicyDir   = "policy"
const ReposDir    = "repos"
const ResultsDir  = "results"
```

Subdirectory names within the complyctl workspace for OPA artifacts.

### Types

#### Config

```go
type Config struct {
    WorkspaceDir string
}
```

Holds the provider configuration rooted at the complyctl workspace directory.

### Functions

#### NewConfig

```go
func NewConfig(workspaceDir string) *Config
```

Returns a new `Config` rooted at the given workspace directory.

#### OpaDir

```go
func (c *Config) OpaDir() string
```

Returns `<workspace>/opa`.

#### PolicyDirPath

```go
func (c *Config) PolicyDirPath() string
```

Returns `<workspace>/opa/policy`. This is where downloaded OPA bundles are
stored.

#### ReposDirPath

```go
func (c *Config) ReposDirPath() string
```

Returns `<workspace>/opa/repos`. This is where cloned git repositories are
stored.

#### ResultsDirPath

```go
func (c *Config) ResultsDirPath() string
```

Returns `<workspace>/opa/results`. This is where per-target JSON result files
are written.

#### EnsureDirectories

```go
func (c *Config) EnsureDirectories() error
```

Creates all workspace subdirectories (`opa/`, `opa/policy/`, `opa/repos/`,
`opa/results/`) with mode `0750`. Returns an error if any directory creation
fails.

---

## Package `scan`

**File:** `cmd/opa-provider/scan/scan.go`

### Interfaces

#### CommandRunner

```go
type CommandRunner interface {
    Run(name string, args ...string) ([]byte, error)
    RunWithEnv(env []string, name string, args ...string) ([]byte, error)
}
```

Abstracts command execution for testing. `Run` executes a command with the
default environment. `RunWithEnv` executes with a custom environment (used for
injecting access tokens).

### Types

#### ExecRunner

```go
type ExecRunner struct{}
```

Production implementation of `CommandRunner` using `os/exec`.

### Functions

#### PullBundle

```go
func PullBundle(bundleRef, policyDir string, runner CommandRunner) error
```

Downloads an OPA policy bundle from an OCI registry using `conftest pull`.

**Command constructed:**

```
conftest pull oci://<bundleRef> --policy <policyDir>
```

**Parameters:**

| Parameter | Description |
|:----------|:------------|
| `bundleRef` | OCI reference (e.g., `ghcr.io/org/bundle:dev`) |
| `policyDir` | Local directory to store downloaded policies |
| `runner` | Command executor |

#### CloneRepository

```go
func CloneRepository(url, branch, cloneDir, accessToken string, runner CommandRunner) error
```

Clones a git repository at a specific branch with `--depth 1` (shallow clone).
If `accessToken` is non-empty, it is injected via the platform-specific
environment variable (`GITHUB_TOKEN` or `GITLAB_TOKEN`). The token is never
passed as a command-line argument.

**Command constructed:**

```
git clone --branch <branch> --depth 1 <url> <cloneDir>
```

**Platform detection:** The function inspects the URL hostname. URLs containing
`gitlab` use `GITLAB_TOKEN`; all others use `GITHUB_TOKEN`.

#### EvalPolicy

```go
func EvalPolicy(inputPath, policyDir string, runner CommandRunner) ([]byte, error)
```

Evaluates configuration files against OPA policies using `conftest test`.

**Command constructed:**

```
conftest test <inputPath> --policy <policyDir> --output json --all-namespaces --no-fail
```

**Flags:**

| Flag | Purpose |
|:-----|:--------|
| `--output json` | Machine-parseable structured output |
| `--all-namespaces` | Evaluate policies across all Rego namespaces |
| `--no-fail` | Exit 0 even on policy violations (failures are in JSON) |

**Returns:** Raw JSON output from conftest, or an error if the command fails.

---

## Package `results`

**File:** `cmd/opa-provider/results/results.go`

### Constants

```go
const maxFieldSize = 10 * 1024 // 10KB per field
```

Maximum size for string fields in findings. Longer values are truncated with
a `[truncated]` suffix.

### Types

#### PerTargetResult

```go
type PerTargetResult struct {
    Target       string    `json:"target"`
    Branch       string    `json:"branch,omitempty"`
    ScannedAt    time.Time `json:"scanned_at"`
    Findings     []Finding `json:"findings"`
    SuccessCount int       `json:"success_count"`
    Status       string    `json:"status"`
    Error        string    `json:"error,omitempty"`
}
```

Holds scan findings for a single target evaluation. Written as JSON to the
results directory for audit trail purposes.

#### Finding

```go
type Finding struct {
    RequirementID string `json:"requirement_id"`
    Title         string `json:"title"`
    Result        string `json:"result"`
    Reason        string `json:"reason"`
    Filename      string `json:"filename"`
}
```

Represents an individual policy violation. `RequirementID` is derived from
conftest's query metadata (e.g., `data.kubernetes.run_as_root.deny` becomes
`kubernetes.run_as_root`).

### Functions

#### ParseConftestOutput

```go
func ParseConftestOutput(raw []byte, target, branch string) (*PerTargetResult, error)
```

Unmarshals conftest JSON output and creates `Finding` entries from failures and
warnings. Both `failures` and `warnings` map to `result: "fail"` in findings.
Successes are counted but do not generate individual findings.

**Requirement ID extraction:** Each conftest result includes a `metadata.query`
field (e.g., `data.docker.network_encryption.warn`). The function strips the
`data.` prefix and the rule type suffix (`warn`, `deny`, `violation`) to derive
the requirement ID (`docker.network_encryption`).

**Error conditions:**

- Empty input returns an error
- Invalid JSON returns an error

#### WritePerTargetResult

```go
func WritePerTargetResult(result *PerTargetResult, dir string) error
```

Writes a `PerTargetResult` as indented JSON to the given directory. The filename
is derived from the target name and branch (sanitized for filesystem safety).
Files are written with mode `0600`.

#### ToScanResponse

```go
func ToScanResponse(targetResults []*PerTargetResult) *provider.ScanResponse
```

Maps a slice of `PerTargetResult` to a `provider.ScanResponse`. Groups findings
by requirement ID into `provider.AssessmentLog` entries. Each assessment log
contains steps (one per target/branch combination) and a summary message
indicating the violation count.

**Result mapping:**

| Finding result | Provider result |
|:---------------|:----------------|
| `"fail"` | `provider.ResultFailed` |
| `"pass"` | `provider.ResultPassed` |
| other | `provider.ResultError` |

All assessments use `provider.ConfidenceLevelHigh`.

---

## Package `targets`

**File:** `cmd/opa-provider/targets/targets.go`

### Functions

#### ParseRepoURL

```go
func ParseRepoURL(repoURL, platformHint string) (platform, org, repo string, err error)
```

Extracts the hosting platform, organization, and repository name from a
repository URL. The URL must use HTTPS.

**Platform detection:**

| Hostname contains | Platform |
|:------------------|:---------|
| `github.com` | `"github"` |
| `gitlab.com` | `"gitlab"` |
| other | error (unless `platformHint` is set) |

For GitLab URLs with nested groups (e.g., `gitlab.com/group/subgroup/repo`),
the organization includes the full group path.

#### SanitizeRepoURL

```go
func SanitizeRepoURL(repoURL string) string
```

Converts a repository URL into a filesystem-safe name by stripping the scheme
and replacing `/`, `.`, `:` with hyphens.

#### RepoDisplayName

```go
func RepoDisplayName(repoURL string) string
```

Extracts the `org/repo` portion from a repository URL. Falls back to the raw
URL string if parsing fails.

#### ValidateInputPath

```go
func ValidateInputPath(inputPath string) error
```

Checks that a local input path exists and does not contain directory traversal
sequences (`..`).

---

## Package `toolcheck`

**File:** `cmd/opa-provider/toolcheck/toolcheck.go`

### Variables

```go
var RequiredTools = []string{"conftest", "git"}
```

The external tools the OPA provider depends on.

### Functions

#### CheckTools

```go
func CheckTools() ([]string, error)
```

Verifies that all required tools are available on the system PATH. Returns a
slice of missing tool names (empty if all are found).

#### FormatMissingToolsError

```go
func FormatMissingToolsError(missing []string) error
```

Constructs a human-readable error message listing each missing tool with
installation guidance.
