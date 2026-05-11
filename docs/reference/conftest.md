<!-- Source: Official docs + GitHub source | Confidence: HIGH | Fetched: 2026-05-09 -->

# Conftest Reference

> Collected for: opa-provider implementation
> Relevant topics: CLI commands (test, pull), JSON output format, OCI bundle pulling, authentication, --no-fail and --all-namespaces flags
> Latest version: v0.68.2 (2026-04-15)

## JSON Output Format

When `--output json` is used, conftest emits a JSON array of `CheckResult` objects.
Each element represents one file/namespace combination evaluated.

### CheckResult Shape

Source: `output/result.go` in `github.com/open-policy-agent/conftest`

```go
type CheckResult struct {
    FileName   string        `json:"filename"`
    Namespace  string        `json:"namespace"`
    Successes  int           `json:"successes"`
    Skipped    []Result      `json:"skipped,omitempty"`
    Warnings   []Result      `json:"warnings,omitempty"`
    Failures   []Result      `json:"failures,omitempty"`
    Exceptions []Result      `json:"exceptions,omitempty"`
    Queries    []QueryResult `json:"queries,omitempty"`
}
```

Key facts:
- `Successes` is an **integer count**, not an array. It represents the number of rules that passed.
- `Warnings`, `Failures`, `Exceptions`, and `Skipped` are arrays of `Result` objects (empty arrays omitted via `omitempty`).
- `Queries` contains raw query evaluation data (omitted by default).

### Result Shape

```go
type Result struct {
    Message  string         `json:"msg"`
    Location *Location      `json:"loc,omitempty"`
    Metadata map[string]any `json:"metadata,omitempty"`
    Outputs  []string       `json:"outputs,omitempty"`
}
```

Key facts:
- `Message` is serialized as `"msg"` in JSON.
- `Metadata` is a free-form `map[string]any`. The `query` key is always present (e.g., `"data.main.warn"`). Custom metadata from Rego METADATA annotations appears here.
- `Location` is optional (added in v0.64). Present only when the Rego policy includes `_loc` metadata.

### Location Shape

```go
type Location struct {
    File string      `json:"file,omitempty"`
    Line json.Number `json:"line,omitempty"`
}
```

### Example JSON Output

```json
[
  {
    "filename": "deployment.yaml",
    "namespace": "main",
    "successes": 4,
    "warnings": [
      {
        "msg": "Found service hello-kubernetes but services are not allowed",
        "metadata": {
          "query": "data.main.warn"
        }
      }
    ],
    "failures": [
      {
        "msg": "Containers must not run as root",
        "metadata": {
          "query": "data.main.deny",
          "controls": ["ITSS-CH1-ACCESS-005"]
        }
      }
    ]
  },
  {
    "filename": "service.yaml",
    "namespace": "kubernetes",
    "successes": 2
  }
]
```

Notes on the example:
- When `--all-namespaces` is used, results appear for each namespace that contains policy rules.
- When arrays are empty (no failures, no warnings), they are omitted from JSON due to `omitempty`.
- The `metadata.controls` field shown above is custom metadata from Rego METADATA annotations -- it is NOT a built-in conftest field. Its presence depends on the policy bundle's Rego code.

### Go Types for Parsing (recommended for opa-provider)

```go
// conftestFileResult maps to the conftest CheckResult JSON output.
type conftestFileResult struct {
    Filename   string          `json:"filename"`
    Namespace  string          `json:"namespace"`
    Successes  int             `json:"successes"`
    Warnings   []conftestCheck `json:"warnings"`
    Failures   []conftestCheck `json:"failures"`
    Exceptions []conftestCheck `json:"exceptions"`
}

// conftestCheck maps to conftest Result JSON output.
type conftestCheck struct {
    Msg      string                 `json:"msg"`
    Metadata map[string]interface{} `json:"metadata"`
}
```

The `Skipped`, `Queries`, `Location`, and `Outputs` fields can be omitted from the parsing types since the opa-provider does not use them.

## CLI Reference: `conftest test`

### Synopsis

```
conftest test [flags] <file> [file...]
```

Evaluates configuration files against Rego policies. Accepts files or directories as input. Directories are recursed.

### Flags Used by opa-provider

| Flag | Short | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `--policy` | `-p` | `[]string` | `["policy"]` | Path(s) to Rego policy directory. Repeatable. |
| `--output` | `-o` | `string` | `stdout` | Output format. Use `json` for structured output. |
| `--all-namespaces` | | `bool` | `false` | Evaluate policies in all Rego packages, not just `main`. |
| `--no-fail` | | `bool` | `false` | Always return exit code 0, even when policies fail. |

### All Flags

| Flag | Short | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `--policy` | `-p` | `[]string` | `["policy"]` | Path(s) to Rego policy directory. Repeatable. |
| `--output` | `-o` | `string` | `stdout` | Output format: stdout, json, tap, table, junit, github, azuredevops, sarif |
| `--all-namespaces` | | `bool` | `false` | Evaluate all namespaces found in policy files. |
| `--no-fail` | | `bool` | `false` | Return exit code 0 even if policies fail. |
| `--fail-on-warn` | | `bool` | `false` | Return non-zero exit code on warnings (exit 1) or failures (exit 2). |
| `--namespace` | `-n` | `[]string` | `["main"]` | Evaluate policies in specific namespace(s). Repeatable. |
| `--data` | `-d` | `[]string` | `[]` | Paths to external data files (JSON/YAML) for policies. Recursed. |
| `--combine` | | `bool` | `false` | Combine all input files into one data structure (array of {path, contents}). |
| `--parser` | | `string` | auto-detect | Force parser: hcl1, hcl2, cue, ini, toml, dockerfile, etc. |
| `--ignore` | | `string` | `""` | Regex pattern to ignore matching files/directories. |
| `--trace` | | `bool` | `false` | Show Rego evaluation trace. Only works with `--output stdout`. |
| `--quiet` | | `bool` | `false` | Suppress successful test output. |
| `--strict` | | `bool` | `false` | Enable strict mode for Rego policies. |
| `--show-builtin-errors` | | `bool` | `false` | Surface built-in function errors during evaluation. |
| `--suppress-exceptions` | | `bool` | `false` | Exclude exceptions from output. |
| `--rego-version` | | `string` | `v1` | Rego syntax version: v0, v1. |
| `--capabilities` | | `string` | `""` | Path to JSON capabilities file restricting OPA functionality. |
| `--tls` | | `bool` | `true` | Use TLS for registry access. |
| `--update` | `-u` | `[]string` | `[]` | URLs to pull policies from before running tests. |
| `--no-color` | | `bool` | `false` | Disable colored output. |
| `--junit-hide-message` | | `bool` | `false` | Exclude violation messages from JUnit test names. |
| `--proto-file-dirs` | | `[]string` | `[]` | Directories containing Protocol Buffer definitions. |
| `--config-file` | `-c` | `string` | `conftest.toml` | Path to conftest configuration file. |

### Exit Code Behavior

Default (no flags):
- `0` -- all policies passed (no failures)
- `1` -- at least one policy failed

With `--fail-on-warn`:
- `0` -- no failures and no warnings
- `1` -- no failures but at least one warning
- `2` -- at least one failure

With `--no-fail`:
- `0` -- always, regardless of failures or warnings

The `--no-fail` flag takes precedence. When both `--fail-on-warn` and `--no-fail` are set, exit code is always 0.

Important: `--no-fail` only affects the exit code. Failures and warnings are still reported in the output (including JSON output). This is critical for the opa-provider: the JSON output contains all violations even with `--no-fail`.

### `--all-namespaces` Behavior

By default, conftest evaluates only the `main` Rego package. The `--all-namespaces` flag tells conftest to:

1. Open all `.rego` files in the policy directory.
2. Parse the `package` declaration from each file.
3. Evaluate `deny`, `violation`, `warn`, and `exception` rules in every discovered package.

When used with `--output json`, results include a `namespace` field per `CheckResult` showing which package produced each result.

This flag is essential for the opa-provider because OPA bundles use platform-specific package names (not `main`), and a single bundle may contain multiple packages.

## CLI Reference: `conftest pull`

### Synopsis

```
conftest pull [flags] <url>
```

Downloads policies from remote locations. Supports multiple protocols via go-getter.

### Flags

| Flag | Short | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `--policy` | `-p` | `string` | `"policy"` | Directory to download policies into. |
| `--tls` | `-s` | `bool` | `true` | Use TLS for registry access. |
| `--absolute-paths` | | `bool` | `false` | Preserve absolute paths in policy flag. |

### Supported Protocols

| Protocol | URL Format | Example |
| --- | --- | --- |
| OCI Registry | `oci://{host}/{path}:{tag}` | `conftest pull oci://ghcr.io/org/bundle:dev` |
| Git (HTTPS) | `git::https://{host}/{path}.git//sub/folder` | `conftest pull git::https://github.com/org/repo.git//policy` |
| Git (auth) | `git::https://{token}@{host}/{path}.git//sub/folder` | Token embedded in URL |
| HTTPS | `https://{url}` | Direct file download |
| S3 | `s3::{url}` | Amazon S3 bucket |
| GCS | `gcs::{url}` | Google Cloud Storage |

### OCI Pull Example (opa-provider pattern)

```bash
conftest pull oci://ghcr.io/org/policy-bundle:dev --policy /workspace/opa/policy
```

This downloads the OPA bundle from the OCI registry and extracts the Rego files into `/workspace/opa/policy/`.

## OCI Authentication

Conftest uses ORAS (oras-project/oras-go) for OCI registry operations. Authentication relies on the standard Docker credential chain:

### Credential Resolution Order

1. **`DOCKER_CONFIG` environment variable** -- if set, conftest reads credentials from the `config.json` file in that directory.
2. **`~/.docker/config.json`** -- the default Docker configuration file location.
3. **Credential helpers** -- Docker config can reference credential helpers (e.g., `docker-credential-gcr`, `docker-credential-ecr-login`) which are invoked to retrieve tokens.
4. **Anonymous access** -- if no credentials are found for the registry, conftest attempts unauthenticated access.

### Practical Setup

For GitHub Container Registry (GHCR):
```bash
# Option 1: docker login
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Option 2: manual config.json
mkdir -p ~/.docker
echo '{"auths":{"ghcr.io":{"auth":"BASE64_USER_TOKEN"}}}' > ~/.docker/config.json

# Option 3: DOCKER_CONFIG env var (CI/CD)
export DOCKER_CONFIG=/path/to/docker-config-dir
```

Conftest does NOT have a built-in `login` command. All authentication is delegated to existing Docker credentials.

### Important for CI/CD

In containerized CI environments (Docker-in-Docker, Kubernetes pods), the Docker config must be explicitly mounted or configured. The `DOCKER_CONFIG` environment variable is the recommended approach.

## Gotchas

### 1. Successes is an integer, not an array

Unlike `failures`, `warnings`, and `exceptions` (which are arrays of `Result`), `successes` is an integer count. There is no per-rule detail for successful checks. The implementation must handle this asymmetry when parsing JSON output.

### 2. Success count can be inaccurate

The success count is calculated as `ruleCount - (len(failures) + len(warnings) + len(exceptions))`, where `ruleCount` is the number of partial rule definitions. When a single rule returns multiple results for a single input, the success count may be incorrect (negative or inflated). Do not rely on the success count for precise pass/fail accounting. Use the absence of failures/warnings as the signal for "passed."

### 3. Empty arrays are omitted in JSON

Due to `omitempty` on the struct tags, when there are no failures for a file/namespace combination, the `failures` key is absent from the JSON (not an empty array). The Go JSON unmarshaler handles this correctly (nil slice), but code must not assume the key is always present.

### 4. Metadata shape is policy-dependent

The `metadata` field on each `Result` is a free-form map. The only guaranteed key is `query` (e.g., `"data.kubernetes.warn"`). Any other keys (like `controls`, `check`, `family`) depend entirely on the Rego policy's METADATA annotations. The opa-provider must handle the case where expected metadata keys are absent.

### 5. --no-fail still reports violations

The `--no-fail` flag only suppresses the non-zero exit code. All failures and warnings are still present in the JSON output. This is the correct behavior for the opa-provider: use `--no-fail` to prevent conftest from returning a non-zero exit code (which `exec.Command` would interpret as an error), then parse the JSON to extract violations.

### 6. --all-namespaces produces multiple results per file

When `--all-namespaces` is used, the same file can appear in multiple `CheckResult` entries -- one per namespace. The opa-provider must aggregate results across all namespace entries for a given file.

### 7. OCI registry detection heuristics

Conftest uses hostname-based heuristics to detect OCI registries. When using the `oci://` prefix explicitly, this detection is bypassed. Always use the `oci://` prefix for OCI registry URLs in the opa-provider to avoid detection issues with on-premises registries.

### 8. Rego v1 is now the default

As of conftest v0.68.x, the `--rego-version` flag defaults to `v1`. Rego v1 has syntax differences from v0 (e.g., `import rego.v1` is implicit, `if` keyword required in rule bodies). Ensure policy bundles are compatible with Rego v1 or explicitly set `--rego-version v0` if using legacy policies.

### 9. conftest pull overwrites the policy directory

`conftest pull` extracts files into the `--policy` directory, overwriting existing files with the same names. There is no merge behavior. Pull to a clean directory or be aware that successive pulls may overwrite previous policy files.

## Sources

- https://www.conftest.dev/output/ (HIGH)
- https://www.conftest.dev/options/ (HIGH)
- https://www.conftest.dev/sharing/ (HIGH)
- https://github.com/open-policy-agent/conftest/blob/master/output/result.go (HIGH)
- https://github.com/open-policy-agent/conftest/blob/master/internal/commands/test.go (HIGH)
- https://github.com/open-policy-agent/conftest/blob/master/internal/commands/pull.go (HIGH)
- https://github.com/open-policy-agent/conftest/releases (HIGH)
- https://github.com/open-policy-agent/conftest/pull/547 (MEDIUM -- --no-fail PR)
- https://github.com/open-policy-agent/conftest/issues/236 (MEDIUM -- --all-namespaces issue)
- https://github.com/open-policy-agent/conftest/issues/567 (MEDIUM -- OCI auth discussion)

## RESULT: COMPLETE

**Scope:** Reference documentation for conftest
### Files
| File | Action |
|------|--------|
| `docs/reference/conftest.md` | Created |
### Metrics
Confidence: HIGH
### Awaiting
None -- work complete
