% COMPLYCTL-PROVIDER-OPA(1) ComplyTime Manual
% Marcus Burghardt <maburgha@redhat.com>
% September 2026

# NAME

complyctl-provider-opa - OPA/Conftest scanning provider for complyctl

# SYNOPSIS

**complyctl-provider-opa**

# DESCRIPTION

**complyctl-provider-opa** is a gRPC plugin for **complyctl**(1) that
provides compliance scanning capabilities using the Open Policy Agent
(OPA) engine via Conftest. It is not invoked directly by users;
**complyctl** discovers and launches it automatically when the provider
is installed.

The provider receives assessment configurations from **complyctl** and
evaluates configuration files and infrastructure-as-code against Rego
policies using the **conftest** CLI tool. It supports both remote git
repositories and local filesystem paths as scan targets, and loads
policy bundles from OCI registries or cached complypack content.

Communication with **complyctl** uses the hashicorp/go-plugin gRPC
interface, implementing three RPCs: **Describe**, **Generate**, and
**Scan**.

# CONFIGURATION

Configuration is provided through variables defined in assessment plan
targets. These variables are passed to the provider via **complyctl** and
are not set manually by the user.

**url** (optional, mutually exclusive with **input_path**)
:   HTTPS URL of a git repository to clone and scan. One of **url** or
    **input_path** must be provided per target.
    Example: **https://github.com/org/repo**

**input_path** (optional, mutually exclusive with **url**)
:   Local filesystem path to scan. Must exist and must not contain path
    traversal sequences (**../**). One of **url** or **input_path** must
    be provided per target.

**branches** (optional)
:   Comma-separated list of branches to scan for remote repositories.
    Must match the pattern **[a-zA-Z0-9._/-]+**. Path traversal
    sequences are rejected.
    Default: **main**

**access_token** (optional)
:   Authentication token for cloning private git repositories. Used as
    the password in git credential helper configuration. Rejected if it
    contains newlines or null bytes.

**scan_path** (optional)
:   Subdirectory within the cloned repository to scan. Path traversal
    sequences (**../**) are rejected.
    Default: root of the cloned repository.

**platform** (optional)
:   Platform hint for self-hosted git instances (e.g., **github**,
    **gitlab**). Affects the credential username (**oauth2** for GitLab,
    **x-access-token** otherwise).
    Default: auto-detected from the URL hostname.

**opa_bundle_ref** (required when no complypack is available)
:   OCI reference for a Conftest policy bundle. Used to pull policies
    via **conftest pull** when no complypack content path is set by
    **complyctl**.
    Example: **oci://ghcr.io/org/policies:v1**

The OCI policy bundle must include a **complytime-mapping.json** file
that maps requirement IDs to Rego namespaces. The provider returns an
error if this file is missing.

# ENVIRONMENT

No environment variables are read directly by this provider.

# EXTERNAL TOOLS

Requires the following tools at runtime:

**conftest**
:   Open Policy Agent test runner for structured data. Not currently
    packaged in Fedora; must be installed separately. See:
    <https://www.conftest.dev>

**git**
:   Required for cloning remote repositories when **url** is specified.
    Provided by the **git** Fedora package.

# FILES

*.complytime/opa/*
:   Provider workspace directory. All paths below are relative to the
    current working directory.

*.complytime/opa/results/\<target\>-\<branch\>.json*
:   Per-target result JSON summarizing conftest findings for a single
    target and branch combination. For local targets, the branch
    component is omitted.

*.complytime/opa/generated/scan-config.json*
:   Scan configuration written by the **Generate** RPC. Records matched
    requirement IDs, the Rego-to-Gemara reverse mapping, and the
    resolved policy directory path.

*.complytime/opa/generated/complytime-mapping.json*
:   Mapping file shipped in the OCI policy bundle or complypack. Maps
    Gemara requirement IDs to Rego policy namespaces. Required for
    requirement-scoped evaluation.

*.complytime/opa/repos/*
:   Directory for cloned git repositories when **url** targets are used.
    Each repository is cloned into a subdirectory named by the sanitized
    URL and branch.

*.complytime/opa/policy/*
:   Directory for downloaded OPA policy bundles (from OCI registries
    or complypack content).

# EVIDENCE

The provider records an **Evidence** entry on each **AssessmentLog**
returned by the **Scan** RPC:

**Type**
:   **ConftestResult** — the conftest evaluation output for the target
    input path.

**Production**
:   The provider invokes **conftest test** with namespace-scoped policies
    against the input path (a cloned repository directory or a local
    filesystem path). The conftest JSON output is parsed into per-target
    result files.

**EvaluationLog representation**
:   Each assessment log carries an Evidence entry with:

    - **Source.Coordinate**: the input path evaluated by conftest
      (e.g., *.complytime/opa/repos/\<repo\>/\<branch\>* or a local
      filesystem path)
    - **Source.ReferenceID**: **conftest-input-\<target\>**

    No digest is recorded because the input is a directory, not a
    single file.

# EXIT CODES

This provider is a gRPC subprocess managed by **complyctl**. Exit codes
follow the hashicorp/go-plugin protocol and are not user-facing.

# SEE ALSO

**complyctl**(1), **complyctl-provider-openscap**(1),
**complyctl-provider-ampel**(1), **conftest**(1)

Project: <https://github.com/complytime/complytime-providers>

# COPYRIGHT

Apache-2.0
