# ampel-plugin Configuration

## Overview

The ampel-plugin reads its target repository configuration directly from `complytime.yaml`. Each repository to scan is defined as its own target entry with repository details passed as plain string variables. Multi-value fields use comma-separated strings.

## Configuration Reference

### Target variables

Each target in `complytime.yaml` uses the `variables` map to specify repository details:

```yaml
targets:
  - id: <target-id>
    policies:
      - <policy-id>
    variables:
      url: <repository-url>              # required, HTTPS URL to repository
      specs: <spec-refs>                 # required, comma-separated snappy spec references
      branches: <branch-names>           # optional, comma-separated (default: main)
      access_token: <token>              # optional, supports ${VAR} expansion
      platform: <platform>              # optional, github or gitlab (for self-hosted)
```

### Variable reference

| Variable | Required | Description |
|----------|----------|-------------|
| `url` | Yes | HTTPS URL to a repository (e.g., `https://github.com/myorg/repo`) |
| `specs` | Yes | Comma-separated snappy spec file references. Use `builtin:` prefix for embedded specs. |
| `branches` | No | Comma-separated branch names to scan (default: `main`) |
| `access_token` | No | Authentication token for this repository. Supports `${VAR}` env var expansion. |
| `platform` | No | `github` or `gitlab`. Required for self-hosted instances; auto-detected for `github.com` and `gitlab.com`. |

### Granular policy directory

The `ampel_policy_dir` global variable controls where the plugin reads granular AMPEL policy source files:

```yaml
variables:
  ampel_policy_dir: /path/to/custom/policies  # optional
```

- **Default**: `.complytime/ampel/granular-policies/`
- **Generated output**: Always written to `.complytime/ampel/policy/` (unchanged)

## Examples

### Single GitHub repository

```yaml
policies:
  - url: registry.example.com/policies/branch-protection@v1.0
    id: branch-protection

targets:
  - id: myorg-frontend
    policies:
      - branch-protection
    variables:
      url: https://github.com/myorg/frontend
      specs: builtin:github/branch-rules.yaml
      branches: main,develop
```

### Multiple repositories

Each repository is its own target entry:

```yaml
policies:
  - url: registry.example.com/policies/branch-protection@v1.0
    id: branch-protection

targets:
  - id: myorg-frontend
    policies:
      - branch-protection
    variables:
      url: https://github.com/myorg/frontend
      specs: builtin:github/branch-rules.yaml
      branches: main,develop

  - id: myorg-backend
    policies:
      - branch-protection
    variables:
      url: https://github.com/myorg/backend
      specs: builtin:github/branch-rules.yaml
      access_token: ${BACKEND_GITHUB_TOKEN}

  - id: myorg-infra
    policies:
      - branch-protection
    variables:
      url: https://gitlab.com/myorg/infrastructure
      specs: builtin:github/branch-rules.yaml
      branches: main,release
      access_token: ${GITLAB_API_TOKEN}
```

### Self-hosted instance

Use the `platform` variable to specify the platform for self-hosted Git servers:

```yaml
targets:
  - id: corp-repo
    policies:
      - branch-protection
    variables:
      url: https://git.corp.com/myorg/repo
      specs: builtin:github/branch-rules.yaml
      platform: github
      access_token: ${CORP_GIT_TOKEN}
```

## Token Authentication

### When `access_token` is set

The token value is expanded from environment variables at config load time (e.g., `${MY_TOKEN}` reads the `MY_TOKEN` env var). During scanning, the plugin detects the platform and injects the token into the snappy subprocess environment:

- `github` platform: sets `GITHUB_TOKEN=<value>`
- `gitlab` platform: sets `GITLAB_TOKEN=<value>`

### When `access_token` is omitted

Snappy inherits the parent process environment unchanged. It reads `GITHUB_TOKEN` or `GITLAB_TOKEN` directly from the environment. This is sufficient when all repositories use the same token.

### Which env vars are expected per platform

| Platform | Environment Variable |
|----------|---------------------|
| GitHub | `GITHUB_TOKEN` |
| GitLab | `GITLAB_TOKEN` |

### Security considerations

- Tokens are expanded from environment variables at config load time. Never hardcode tokens directly in `complytime.yaml`.
- The `${VAR}` syntax fails with a clear error if the referenced environment variable is not set.
- Tokens are validated to reject newlines and null bytes (prevents header/env injection).
- Tokens are passed via environment variables to subprocess commands, not as command-line arguments.

## Granular Policy Directory

The `ampel_policy_dir` global variable specifies the directory containing granular AMPEL policy source files (one JSON file per control). This is a workspace-scoped setting shared across all targets.

```yaml
variables:
  ampel_policy_dir: /path/to/custom/policies
```

- **Default location**: `.complytime/ampel/granular-policies/`
- **Generated output location**: `.complytime/ampel/policy/` (not configurable, always separate from source policies)
- **Purpose**: Separates user-authored or tool-generated policy source files from the merged policy bundle produced by the plugin's `generate` phase.
