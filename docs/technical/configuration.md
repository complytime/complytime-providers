# OPA Provider — Configuration Reference

This document describes the configuration surface of the OPA provider:
target variables, workspace directory layout, required tools, and
environment variables.

## Target Variables

Target variables are key-value pairs set on each `provider.Target` in the
`ScanRequest`. They control what the provider scans and how it accesses
resources.

### Required Variables

At least one of `url` or `input_path` must be set per target. Setting both
on the same target is an error.

| Variable | Type | Description |
|:---------|:-----|:------------|
| `url` | string | HTTPS URL of a git repository to clone and scan. Must use the `https://` scheme. |
| `input_path` | string | Absolute path to a local file or directory to scan. Must exist and must not contain `..` traversal. |

### Global Variable

| Variable | Type | Description |
|:---------|:-----|:------------|
| `opa_bundle_ref` | string | **Required.** OCI reference for the policy bundle (e.g., `ghcr.io/org/opa-bundle:latest`). Must be set on at least one target. The bundle is pulled once and shared across all targets. |

### Optional Variables

| Variable | Type | Default | Description |
|:---------|:-----|:--------|:------------|
| `branches` | string | `"main"` | Comma-separated list of branch names to scan. Each branch is cloned and evaluated separately. Only used with `url` targets. |
| `access_token` | string | (none) | Authentication token for private repositories. Injected as `GITHUB_TOKEN` or `GITLAB_TOKEN` depending on the URL hostname. Never passed as a CLI argument. |
| `scan_path` | string | (none) | Subdirectory within the cloned repository to scan. If empty, the repository root is scanned. Must not contain `..` traversal. Only used with `url` targets. |

### Variable Examples

**Local scan:**

```json
{
  "target_id": "my-configs",
  "variables": {
    "input_path": "/home/user/k8s-manifests",
    "opa_bundle_ref": "ghcr.io/myorg/opa-policies:v1.0"
  }
}
```

**Remote scan (public repo, multiple branches):**

```json
{
  "target_id": "infra-repo",
  "variables": {
    "url": "https://github.com/myorg/infrastructure",
    "opa_bundle_ref": "ghcr.io/myorg/opa-policies:v1.0",
    "branches": "main, staging, production"
  }
}
```

**Remote scan (private repo with subdirectory):**

```json
{
  "target_id": "private-k8s",
  "variables": {
    "url": "https://github.com/myorg/private-infra",
    "opa_bundle_ref": "ghcr.io/myorg/opa-policies:v1.0",
    "branches": "main",
    "access_token": "ghp_xxxxxxxxxxxxx",
    "scan_path": "deploy/k8s"
  }
}
```

## Workspace Directory Layout

The OPA provider creates a workspace directory structure under the complyctl
workspace (`~/.complytime/workspace/` by default). All directories are created
automatically on the first scan with mode `0750`.

```
<workspace>/
└── opa/
    ├── policy/        # Downloaded OPA policy bundles
    ├── repos/         # Cloned git repositories
    │   ├── github-com-org-repo-main/
    │   └── github-com-org-repo-staging/
    └── results/       # Per-target scan result JSON files
        ├── org-repo-main.json
        └── my-configs.json
```

### Directory Descriptions

| Directory | Constant | Purpose |
|:----------|:---------|:--------|
| `opa/` | `config.ProviderDir` | Root directory for all OPA provider artifacts |
| `opa/policy/` | `config.PolicyDir` | Stores Rego policy files downloaded via `conftest pull` |
| `opa/repos/` | `config.ReposDir` | Stores shallow-cloned git repositories, one per URL+branch combination |
| `opa/results/` | `config.ResultsDir` | Stores per-target scan results as JSON files for audit trail |

### Result File Format

Each target evaluation produces a JSON file in `opa/results/`:

```json
{
  "target": "myorg/infrastructure",
  "branch": "main",
  "scanned_at": "2026-05-09T14:30:00Z",
  "findings": [
    {
      "requirement_id": "kubernetes.run_as_root",
      "title": "Container must not run as root",
      "result": "fail",
      "reason": "Container must not run as root",
      "filename": "deployment.yaml"
    }
  ],
  "success_count": 3,
  "status": "scanned"
}
```

Result files are written with mode `0600`. The filename is derived from the
target name and branch, with special characters replaced by hyphens.

## Required External Tools

The OPA provider depends on two external command-line tools. Both must be
available on the system `PATH`.

| Tool | Purpose | Installation |
|:-----|:--------|:-------------|
| `conftest` | Pulls OPA policy bundles from OCI registries and evaluates configuration files against Rego policies | [conftest.dev](https://www.conftest.dev/) |
| `git` | Clones remote repositories for scanning | System package manager |

Tool availability is checked during both `Describe` (health check) and `Scan`
(prerequisite validation). If tools are missing, `Describe` reports the provider
as unhealthy and `Scan` returns an error listing the missing tools.

## Environment Variables

The OPA provider uses environment variables for git authentication. These are
set programmatically during remote target scanning — they are not user-configured.

| Variable | When Set | Purpose |
|:---------|:---------|:--------|
| `GITHUB_TOKEN` | When `access_token` is provided and URL contains `github` | Authenticates git clone operations against GitHub |
| `GITLAB_TOKEN` | When `access_token` is provided and URL contains `gitlab` | Authenticates git clone operations against GitLab |
| `GIT_TERMINAL_PROMPT` | Always set to `0` when `access_token` is provided | Prevents interactive auth prompts that could hang the subprocess |

## OCI Bundle Authentication

The `conftest pull` command uses Docker's credential store for OCI registry
authentication. If the OCI registry hosting the policy bundle requires
authentication, configure Docker credentials before running a scan:

```bash
docker login ghcr.io
```

The provider does not handle OCI authentication directly — it delegates to
conftest, which delegates to the Docker credential chain.

## Validation Rules

The provider applies defense-in-depth validation on all target variables:

| Rule | Error |
|:-----|:------|
| Both `url` and `input_path` set | `"specify either url or input_path, not both"` |
| Neither `url` nor `input_path` set | `"url or input_path is required"` |
| `url` not HTTPS | `"url \"<url>\" must use HTTPS scheme"` |
| Branch name contains `..` | `"branch name contains path traversal: \"<branch>\""` |
| Branch name has invalid characters | `"branch name contains invalid characters"` |
| `scan_path` contains `..` | `"scan_path contains path traversal"` |
| `access_token` contains `\n`, `\r`, `\x00` | `"access_token contains invalid characters"` |
| `input_path` contains `..` | `"input path contains directory traversal"` |
| `input_path` does not exist | `"input path does not exist"` |
| `opa_bundle_ref` not set on any target | `"opa_bundle_ref variable is required but not set"` |
