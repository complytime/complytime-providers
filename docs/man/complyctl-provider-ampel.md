% COMPLYCTL-PROVIDER-AMPEL(1) ComplyTime Manual
% Marcus Burghardt <maburgha@redhat.com>
% September 2026

# NAME

complyctl-provider-ampel - Ampel scanning provider for complyctl

# SYNOPSIS

**complyctl-provider-ampel**

# DESCRIPTION

**complyctl-provider-ampel** is a gRPC plugin for **complyctl**(1) that
provides compliance scanning capabilities using the Ampel evaluation
engine. It is not invoked directly by users; **complyctl** discovers and
launches it automatically when the provider is installed.

The provider receives assessment configurations from **complyctl** and
evaluates repository configurations against granular security policies
(e.g., branch protection rules) using the **snappy** and **ampel** CLI
tools. Results include per-tenet messages and remediation guidance.

Communication with **complyctl** uses the hashicorp/go-plugin gRPC
interface, implementing three RPCs: **Describe**, **Generate**, and
**Scan**.

# CONFIGURATION

Configuration is provided through variables defined in assessment plan
targets. These variables are passed to the provider via **complyctl** and
are not set manually by the user.

**url** (required)
:   HTTPS URL of the repository to scan. Must use the HTTPS scheme.
    Example: **https://github.com/org/repo**

**specs** (required)
:   Comma-separated list of spec references identifying the policy files
    to evaluate. Example: **builtin:github/branch-rules.yaml**. Path
    traversal sequences (**../**) are rejected.

**branches** (optional)
:   Comma-separated list of branches to scan. Must match the pattern
    **[a-zA-Z0-9._/-]+**. Path traversal sequences are rejected.
    Default: **main**

**access_token** (optional)
:   Authentication token for accessing private repositories. Rejected if
    it contains newlines or null bytes.

**platform** (optional)
:   Platform hint for self-hosted instances (e.g., **github**, **gitlab**).
    Used when hostname-based detection fails to identify the platform.
    Default: auto-detected from the URL hostname.

**ampel_policy_dir** (optional, global variable)
:   Custom directory containing granular Ampel policy source files. Takes
    second priority after complypack content.
    Default: **.complytime/ampel/granular-policies/**

# ENVIRONMENT

**COMPLYTIME_MACHINE_ID_FILE**
:   Path to the machine-id file used for host identification in
    evidence remarks. The file content is included in each
    Evidence.Source.Remarks field alongside the hostname so auditors
    can identify the host where the scan was executed.
    Default: **/etc/machine-id**

# EXTERNAL TOOLS

Requires **snappy** and **ampel** CLI tools at runtime. These tools are
not currently packaged in Fedora and must be installed separately. See
the Carabiner project for installation instructions:
<https://github.com/carabiner-dev>

# FILES

*.complytime/ampel/*
:   Provider workspace directory. All paths below are relative to the
    current working directory.

*.complytime/ampel/results/\<repo\>-\<branch\>-\<spec\>-snappy.intoto.json*
:   In-toto attestation produced by **snappy snap**. Contains raw API data
    collected from the repository (e.g., branch protection rules). The
    filename encodes the sanitized repository URL, branch, and spec label.

*.complytime/ampel/results/\<repo\>-\<branch\>-\<spec\>-ampel.intoto.json*
:   In-toto attestation produced by **ampel verify**. Contains the policy
    evaluation results against the snappy-collected data. The filename
    follows the same pattern as the snappy attestation.

*.complytime/ampel/results/\<repo\>-\<branch\>.json*
:   Per-repository result JSON summarizing findings for a single
    repository and branch combination.

*.complytime/ampel/policy/scan-config.json*
:   Scan configuration written by the **Generate** RPC. Records the
    matched requirement IDs so the **Scan** RPC can synthesize passing
    assessments for requirements with no findings.

*.complytime/ampel/policy/*
:   Merged AMPEL policy bundle generated from granular policy files.

*.complytime/ampel/granular-policies/*
:   Default directory for granular AMPEL policy source files. Overridden
    by **ampel_policy_dir** or complypack content.

# EVIDENCE

The provider records **Evidence** entries on each **AssessmentLog**
returned by the **Scan** RPC. Two attestation types are produced per
repository/branch/spec combination:

**Snappy attestation** (type: **IntotoAttestation**)
:   The **snappy** tool collects raw API data from the repository platform
    (e.g., GitHub branch protection settings) and produces an in-toto
    attestation. This attestation captures the observed state of the
    repository at scan time.

    - **Source.Coordinate**: path to the snappy attestation file
    - **Source.Digest**: **sha256:\<hex\>** digest of the attestation file
    - **Source.ReferenceID**: **snappy-\<spec-label\>**

**Ampel attestation** (type: **IntotoAttestation**)
:   The **ampel** tool evaluates granular security policies against the
    snappy-collected data and produces an in-toto attestation containing
    the evaluation results. A non-zero exit code indicates policy
    failures, not tool errors.

    - **Source.Coordinate**: path to the ampel result attestation file
    - **Source.Digest**: **sha256:\<hex\>** digest of the attestation file
    - **Source.ReferenceID**: **ampel-\<spec-label\>**

# EXIT CODES

This provider is a gRPC subprocess managed by **complyctl**. Exit codes
follow the hashicorp/go-plugin protocol and are not user-facing.

# SEE ALSO

**complyctl**(1), **complyctl-provider-openscap**(1),
**complyctl-provider-opa**(1)

Project: <https://github.com/complytime/complytime-providers>

# COPYRIGHT

Apache-2.0
