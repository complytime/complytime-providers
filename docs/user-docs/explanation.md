# Explanation: OPA Provider Architecture

This document explains why the OPA provider exists, how it fits into the
complyctl ecosystem, and the design trade-offs behind its implementation.

## Why a dedicated OPA provider?

The complyctl CLI ships with two existing providers for compliance scanning:

- **openscap** evaluates system-level security configuration (XCCDF datastreams
  against live hosts)
- **ampel** verifies in-toto attestations against JSON policy bundles (supply
  chain provenance)

Neither can evaluate configuration files — Kubernetes manifests, Terraform
modules, Dockerfiles, CI pipelines — against OPA/Rego policies. This is a
fundamentally different data flow:

| Aspect | openscap | ampel | opa |
|:-------|:---------|:------|:----|
| Input | Live system state | In-toto attestations | Configuration files |
| Engine | OpenSCAP | snappy + ampel | conftest (OPA) |
| Policy format | XCCDF/OVAL | JSON bundles | OCI-hosted Rego bundles |
| Evaluation | System scan | Attestation verification | Policy-as-code |

Building a third provider keeps each implementation focused on its domain.
This follows the ComplyTime Constitution's Simplicity & Isolation principle:
each provider does one thing well.

## How the OPA provider fits into complyctl

Complyctl uses a plugin architecture based on `hashicorp/go-plugin`. Each
provider is a standalone binary that complyctl discovers, launches as a
subprocess, and communicates with over gRPC.

```
complyctl (host process)
  ├── discovers providers in ~/.complytime/providers/
  ├── reads provider manifests (c2p-<name>-manifest.json)
  └── launches provider binaries as gRPC subprocesses
       ├── complyctl-provider-openscap
       ├── complyctl-provider-ampel
       └── complyctl-provider-opa  ← this provider
```

The three RPCs in the provider interface map to the compliance assessment
lifecycle:

1. **Describe** — "Are you healthy? What do you need?" The provider checks its
   dependencies (conftest, git) and reports what target variables it requires.

2. **Generate** — "Convert assessment plans into your native format." The OPA
   provider defers this phase — it works with pre-built OCI bundles rather than
   generating policy files from OSCAL assessment plans.

3. **Scan** — "Evaluate these targets and report findings." The provider pulls
   the policy bundle, processes each target, and maps results back to complyctl's
   assessment format.

## Why conftest instead of the OPA library?

The provider shells out to `conftest` as a subprocess rather than importing
OPA's Go library directly. This was a deliberate choice:

**Conftest advantages:**

- Built-in support for parsing 15+ configuration formats (YAML, JSON, HCL,
  Dockerfile, INI, XML, and more) — the provider does not need format-specific
  parsers
- Native OCI bundle support via `conftest pull` — no OCI client implementation
  needed
- Well-tested, actively maintained tool with a large user community
- The `--no-fail` flag makes conftest report violations in structured JSON
  rather than via exit codes, simplifying error handling

**Trade-off:** The provider depends on conftest being installed on the system.
If conftest is missing, the provider reports as unhealthy and scans fail. This
is acceptable because the provider is a compliance tool that runs in controlled
environments (CI pipelines, developer workstations) where tool installation is
managed.

## How requirement IDs are mapped

The bridge between conftest's output and complyctl's assessment model is the
requirement ID. Here is how the mapping works:

1. OPA policies are written in Rego with rule names like `deny` or `warn` in
   namespaces like `kubernetes.run_as_root`

2. When conftest evaluates a policy, each result includes a `metadata.query`
   field identifying the rule: `data.kubernetes.run_as_root.deny`

3. The provider strips the `data.` prefix and the rule type suffix (`deny`,
   `warn`, `violation`) to derive a requirement ID: `kubernetes.run_as_root`

4. Findings with the same requirement ID from different targets are grouped
   into a single `AssessmentLog` with multiple steps

This v1 approach derives IDs from the Rego namespace structure. A future
enhancement will read explicit control IDs from Rego METADATA annotations,
enabling direct mapping to compliance framework controls (e.g., NIST AC-6,
CIS-K8S-5.2.1).

## Why errors are handled per-target

When scanning multiple targets, a failure in one target (clone error, parse
error, evaluation error) does not stop the scan. The provider captures the
error in the results and continues processing the remaining targets.

This design serves two use cases:

- **CI pipelines** scanning multiple repositories: one unreachable repo should
  not block compliance reporting for the others
- **Multi-branch scanning**: a broken branch should not prevent results from
  other branches

Only truly global errors — no targets provided, missing policy bundle, missing
tools — cause the scan to fail entirely.

## How authentication works

The provider handles two separate authentication concerns:

### Git repository authentication

For private repositories, the `access_token` variable is injected as an
environment variable (`GITHUB_TOKEN` or `GITLAB_TOKEN`). The platform is
auto-detected from the URL hostname. The token is never passed as a
command-line argument, preventing exposure in process listings.

The provider also sets `GIT_TERMINAL_PROMPT=0` to prevent git from prompting
for credentials interactively, which would hang the subprocess.

### OCI registry authentication

Policy bundle authentication is handled entirely by conftest, which delegates
to Docker's credential store. If a registry requires authentication, users log
in with `docker login` before running the scan. The provider does not touch OCI
credentials directly.

## The Generate phase gap

The Generate RPC is a stub that returns success. In the full complyctl
lifecycle, Generate converts an OSCAL assessment plan into provider-specific
policy artifacts. For the OPA provider, this would mean:

1. Reading the assessment plan's control selection
2. Mapping controls to OPA bundle contents
3. Generating a tailored conftest configuration

This is deferred because the current workflow uses pre-built OCI bundles that
already contain the complete policy set. A future iteration will implement
Generate to support assessment-plan-driven policy selection.

## See Also

- [Tutorial](tutorial.md) — Step-by-step guide to your first scan
- [How-to Guide](how-to.md) — Task-oriented instructions for specific scenarios
- [Reference](reference.md) — Complete variable and configuration reference
- [Provider Development Guide](../provider-guide.md) — How to build a new
  complyctl provider
