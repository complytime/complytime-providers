# How-to Guide: OPA Provider

Task-oriented instructions for common OPA provider operations. Each section
solves a specific problem. See the [Reference](reference.md) for the complete
list of variables and configuration options.

## Scan a remote git repository

**Prerequisites:** `complyctl`, `conftest`, and `git` installed. The repository
must be accessible via HTTPS.

1. Run complyctl with the `url` variable pointing to the repository:

   ```bash
   complyctl scan \
     --provider opa \
     --target url=https://github.com/myorg/infrastructure \
     --var opa_bundle_ref=ghcr.io/myorg/opa-policies:v1.0
   ```

2. The provider clones the repository (shallow, `--depth 1`), evaluates all
   configuration files in the repository root against the policy bundle, and
   reports findings.

## Scan multiple branches of a repository

**Prerequisites:** Same as remote scanning.

1. Set the `branches` variable to a comma-separated list of branch names:

   ```bash
   complyctl scan \
     --provider opa \
     --target url=https://github.com/myorg/infrastructure \
     --var opa_bundle_ref=ghcr.io/myorg/opa-policies:v1.0 \
     --var branches="main,staging,production"
   ```

2. The provider clones each branch separately and evaluates policies against
   each one. Results include the branch name so you can see which branch has
   which violations.

## Scan a subdirectory within a repository

**Prerequisites:** Same as remote scanning.

1. Set the `scan_path` variable to the subdirectory containing your
   configuration files:

   ```bash
   complyctl scan \
     --provider opa \
     --target url=https://github.com/myorg/infrastructure \
     --var opa_bundle_ref=ghcr.io/myorg/opa-policies:v1.0 \
     --var scan_path=deploy/kubernetes
   ```

2. The provider clones the full repository but only evaluates policies against
   the files in `deploy/kubernetes/`.

## Scan a private repository

**Prerequisites:** A personal access token with read access to the repository.
For GitHub, create a token with `repo` scope. For GitLab, create a token with
`read_repository` scope.

1. Set the `access_token` variable:

   ```bash
   complyctl scan \
     --provider opa \
     --target url=https://github.com/myorg/private-repo \
     --var opa_bundle_ref=ghcr.io/myorg/opa-policies:v1.0 \
     --var access_token=ghp_xxxxxxxxxxxxxxxxxxxx
   ```

2. The provider injects the token as `GITHUB_TOKEN` (or `GITLAB_TOKEN` for
   GitLab URLs) in the environment. The token is never passed as a command-line
   argument.

## Authenticate with a private OCI registry

**Prerequisites:** Docker CLI installed and the registry accessible.

1. Log in to the OCI registry hosting your policy bundles:

   ```bash
   docker login ghcr.io
   ```

2. Run the scan as usual. The `conftest pull` command uses Docker's credential
   store automatically:

   ```bash
   complyctl scan \
     --provider opa \
     --target input_path=/path/to/configs \
     --var opa_bundle_ref=ghcr.io/myorg/opa-policies:v1.0
   ```

## Scan multiple targets in one request

**Prerequisites:** Same as local or remote scanning.

1. Provide multiple `--target` flags. All targets share the same policy bundle
   (set `opa_bundle_ref` on at least one):

   ```bash
   complyctl scan \
     --provider opa \
     --target input_path=/path/to/k8s-configs \
     --target url=https://github.com/myorg/terraform-modules \
     --var opa_bundle_ref=ghcr.io/myorg/opa-policies:v1.0
   ```

2. The provider processes each target independently. If one target fails, the
   others still complete. Per-target errors appear in the results.

## View per-target result files

**Prerequisites:** A completed scan.

1. List the result files in the workspace:

   ```bash
   ls ~/.complytime/workspace/opa/results/
   ```

2. Read a specific result file:

   ```bash
   cat ~/.complytime/workspace/opa/results/myorg-infrastructure-main.json | jq .
   ```

   Each file contains the target name, branch (if applicable), scan timestamp,
   individual findings with requirement IDs, and success count.

## Check provider health

1. Ask complyctl to describe the provider:

   ```bash
   complyctl provider describe opa
   ```

2. The response shows:
   - `Healthy: true/false` — whether `conftest` and `git` are on PATH
   - `Version` — the provider version (`0.1.0`)
   - `RequiredTargetVariables` — variables the provider expects (`url`,
     `input_path`)
   - `ErrorMessage` — lists missing tools if unhealthy

## Troubleshoot common errors

### "required tools not found: conftest"

Install conftest:

```bash
# macOS
brew install conftest

# Linux (binary)
curl -sL https://github.com/open-policy-agent/conftest/releases/latest/download/conftest_Linux_x86_64.tar.gz | tar xz
sudo mv conftest /usr/local/bin/
```

### "opa_bundle_ref variable is required but not set"

Add `--var opa_bundle_ref=<oci-reference>` to your scan command. The OCI
reference points to the registry, repository, and tag of your OPA policy bundle.

### "url must use HTTPS scheme"

Change your repository URL from `http://` to `https://`. The provider only
supports HTTPS for security.

### "specify either url or input_path, not both"

Each target must use either a remote URL or a local path, not both. Split them
into separate `--target` flags.

### "branch name contains path traversal"

Branch names must not contain `..` sequences. Verify that your `branches`
variable contains valid branch names.
