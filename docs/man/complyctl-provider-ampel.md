% COMPLYCTL-PROVIDER-AMPEL(1) ComplyTime Manual
% Marcus Burghardt <maburgha@redhat.com>
% November 2026

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
:   Comma-separated collection spec references for snappy, not policy identities.
    Example: **builtin:github/branch-rules.yaml**. Relative spec file paths
    resolve against the selected workspace. Built-in references pass through to snappy. Path
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

Requires **snappy** and **ampel** CLI tools at runtime. **ampel** version
1.4.2 or later is required for SKIP handling and the **--allow-skip**
flag. The provider verifies the ampel version at **Generate**/**Scan**
time and reports an actionable error if the installed version is below
the minimum.

These tools are not currently packaged in Fedora and must be installed
separately. See the Carabiner project for installation instructions:
<https://github.com/carabiner-dev>

# FILES

*.complytime/ampel/*
:   Provider artifact directory. All paths below are relative to the selected
    workspace root, not the invoking directory or the Evaluation Log's directory.

*.complytime/ampel/results/\<repository\>/\<branch\>/\<spec\>/evaluation-\<time\>-\<token\>/snappy.intoto.json*
:   In-toto attestation produced by **snappy snap**. Contains raw API data
    collected from the repository (e.g., branch protection rules).

*.complytime/ampel/results/\<repository\>/\<branch\>/\<spec\>/evaluation-\<time\>-\<token\>/ampel.intoto.json*
:   In-toto attestation produced by **ampel verify**. Contains the policy
    evaluation results against the snappy-collected data.

*.complytime/ampel/results/\<repository\>/\<branch\>/\<spec\>/evaluation-\<time\>-\<token\>/metadata.json*
:   Best-effort supporting context: source identity, original branch and collection
    spec, available policy/configuration provenance, evaluation token and collection
    start. Metadata omits credentials and evidence digests; it is not evidence authority.

Repository, branch and spec labels are non-empty ASCII components of at most 80 bytes.
Slashes become hyphens, so **release/1** groups under **release-1**. Labels can collide;
the atomically allocated evaluation leaf remains distinct. **time** uses UTC
**YYYYMMDDTHHMMSSZ**; **token** contains 128 bits of cryptographic randomness.

**Breaking change:** former flat attestation paths and normalized per-repository
JSON summaries are no longer written or refreshed. No fixed-path alias replaces
them. Existing legacy files and historical logs remain untouched and are not migrated.

*.complytime/ampel/generated/\<content-key\>/scan-config.json*
:   Scan configuration written by the **Generate** RPC. Records the
    matched requirement IDs so the **Scan** RPC can synthesize passing
    assessments for requirements with no findings.

*.complytime/ampel/generated/\<content-key\>/*
:   Merged AMPEL policy bundle generated from granular policy files.

*.complytime/ampel/generated/generate-index.json*
:   Maps each generated target to its content-keyed scan configuration and policy bundle.

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

    - **Source.Coordinate**: workspace-root-relative path to the retained snappy attestation
    - **Source.Digest**: **sha256:\<hex\>** digest of the attestation file
    - **Source.ReferenceID**: **snappy_\<token\>**, matching its mapping reference

**Ampel attestation** (type: **IntotoAttestation**)
:   The **ampel** tool evaluates granular security policies against the
    snappy-collected data and produces an in-toto attestation containing
    the evaluation results. A non-zero exit code indicates policy
    failures, not tool errors.

    - **Source.Coordinate**: workspace-root-relative path to the retained ampel attestation
    - **Source.Digest**: **sha256:\<hex\>** digest of the attestation file
    - **Source.ReferenceID**: **ampel_\<token\>**, matching its mapping reference

**Breaking change:** evidence and reference IDs identify artifact instances, not
future evaluations of the same target. Evidence IDs use **snappy_\<token\>** and
**ampel_\<token\>**. Both artifacts use the evaluation's UTC collection-start time
for **CollectedAt**. Many assessments share these files rather than creating copies.
Three scans retain three independent attestation pairs per repository/branch/spec,
including valid failing-compliance results and byte-identical repeated outputs.

The Gemara Evaluation Log alone defines the evidence set. Read its evidence mappings,
resolve each coordinate against the selected workspace root, and compare the stored
file's SHA256 digest with the log. Mapping references retain escaped absolute
**file://** URLs identifying the same files, including percent-encoding for spaces,
percent signs and non-ASCII names. A collection spec describes data collection; it
is not a policy identity. Metadata and other unreferenced files are supporting
outputs, not implicit evidence.

**Workspace selection:** unchanged **complyctl** chooses **--workspace**, then
**COMPLYTIME_WORKSPACE**, then its invoking directory. The provider receives that
root in Generate global/Scan target **workspace** variables. Legacy/direct callers
omitting the variable use the provider's current directory. All targets in a request
must agree on the root. Relative provider input/configuration paths now resolve
against that workspace; absolute paths stay absolute. This is a breaking change
for invocation-relative configurations. No process-wide directory change is required.

Local preservation is always on, without retention settings, cleanup, compression
or deduplication. Storage grows with every evaluation's output plus filesystem and
metadata overhead; interrupted attempts can leave unreferenced partial files.
Metadata write/close failures warn without withholding complete, verified evidence.
Required output, parse, digest or ownership/mode failures withhold the affected evidence.

Fresh evaluation directories use **0700** and evidence/supporting files use **0600**,
owned by the provider process's effective UID/GID. Existing parents must permit
traversal and the workspace must be writable by that execution account. Normal
scans are user-owned; direct-root and sudo scans stay root-owned. The original
non-root user needs authorized elevated access to read and hash root-owned files;
there is no automatic ownership handoff or access grant. For permission or disk-space
errors, check execution-account access, parent traversal and available capacity.
Do not widen evidence permissions or alter earlier owners to bypass these checks.

Preserving or uploading the Evaluation Log alone does not preserve or transfer its
referenced bytes. Downstream integrations own storage, transfers, access policy and
longevity: keep the log and its selected files retrievable and verify transferred
bytes against the recorded digests. Collection and optional packaging remain future
complyctl capabilities. Any moved artifacts need mapping to their original references
without rewriting the log. Absolute URLs remain origin-specific after relocation;
workspace-relative coordinates can resolve against a copied, layout-preserving root.

Upgrade stops refreshing fixed-path outputs, leaves legacy files/logs unchanged
without migration and cannot recover evidence overwritten before upgrade.
Rollback to an older provider resumes its overwrite behavior for
new scans; retained evaluation directories remain readable by authorized readers.

# EXIT CODES

This provider is a gRPC subprocess managed by **complyctl**. Exit codes
follow the hashicorp/go-plugin protocol and are not user-facing.

# SEE ALSO

**complyctl**(1), **complyctl-provider-openscap**(1),
**complyctl-provider-opa**(1)

Project: <https://github.com/complytime/complytime-providers>

# COPYRIGHT

Apache-2.0
