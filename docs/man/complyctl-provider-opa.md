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

**workspace** (injected by complyctl)
:   Base for generated artifacts, retained results and evidence coordinates.
    Generate uses its global variable; Scan uses target variables. Absent or
    empty values use provider process CWD. Invalid or conflicting roots fail
    the RPC before storage. The provider does not reread environment precedence.
    Relative input/configuration paths resolve against this base; absolute
    external paths remain absolute. This is a breaking provider lookup change.

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

# SELECTING SCAN INPUT

Keep target files outside the complyctl workspace so generated artifacts
and prior results do not affect OPA assessments. For a local directory
target that contains a workspace, the provider excludes the workspace
files, including user-owned files. Files outside the excluded directories
remain eligible subject to the link and input checks below.

The provider excludes the selected **workspace** base joined with
**.complytime** and a configured provider artifact directory when different,
including an existing invocation-relative default. An absent old default is not
created in the invoking tree. Output paths and evidence coordinates use the selected base,
not the configured exclusion directory. The provider resolves both boundaries
through symbolic links.
If it cannot resolve a boundary, it rejects the target. Check that both
workspace directories exist and that the provider can access their paths,
then retry the scan.

The provider rejects an explicitly selected local workspace file or
directory. To scan such a file, copy it to a directory outside both
workspace boundaries and set **input_path** to the copy. Use a copy, not
a hard link: the provider checks filesystem identity and rejects a hard
link to workspace content when it cannot exclude it safely. It also
rejects unsafe symbolic links and unresolved paths.

An intentional remote clone remains eligible, even though it is stored
inside the workspace. The provider rejects clone files hard-linked to
other workspace content. It also rejects a remote scan path or traversed
link that reaches other workspace content or matches its file identity.
The provider rejects unresolved paths and link cycles. Non-cyclic links
within the selected clone remain eligible. Fix or remove unsafe links, or
copy the intended input outside the workspace.

Local directories and remote clones or **scan_path** directories with no
confirmed evaluated input produce an operational error and no assessments
for that target. This includes empty, workspace-only, and fully ignored
inputs. Add a supported input file or adjust the effective ignore rule;
other targets retain their results.

For a remote target selecting multiple branches, an input-safety rejection or
lack of confirmed evaluated directory input in any branch suppresses assessments
from every branch of that configured target. This applies in either branch order.
The error identifies the rejected target and branch; other configured targets
retain their assessments. Repair the rejected branch's input or select only the
intended eligible branches. Previously written branch artifacts remain on disk
without published evidence or mapping references; they are not deleted.

Workspace exclusion does not prune directories from traversal. An
unreadable directory or broken link can still cause an operational error;
fix the path or its permissions before retrying. Preflight is not a
sandbox: concurrent filesystem changes between preflight and conftest
traversal can defeat isolation. Identity preflight traverses retained history,
so its work grows with stored files and directories. Inaccessible private history
fails closed; use the authorized account or elevated access without widening old
permissions or treating deletion as an automatic remedy.

## Conftest ignore configuration

For a local directory that contains a workspace, the provider combines a
nonempty **CONFTEST_IGNORE** with the workspace exclusion. If the variable
is empty or unset and an applicable conftest configuration file exists,
or the provider cannot determine whether one exists, it rejects the target
rather than discarding a possible user ignore rule. Non-overlapping local
targets and remote clones do not need this workspace ignore rule.

To resolve this error, select a file or directory outside both workspace
boundaries that does not contain either boundary. Alternatively, set
**CONFTEST_IGNORE** to the effective ignore expression, including the
file's existing **ignore** value. If the file defines no ignore rule, use
a valid expression that matches no file paths:

```bash
export CONFTEST_IGNORE='^$'
```

Other conftest configuration settings continue to apply. The provider
checks configuration and runs conftest in the selected workspace, without changing
process CWD. A relative **CONFTEST_CONFIG_FILE** uses that same workspace.

# ENVIRONMENT

**CONFTEST_IGNORE**
:   Conftest ignore regular expression. For local directory targets that
    contain a workspace, the provider combines a nonempty value with the
    workspace exclusion. See **Conftest ignore configuration** above.

**CONFTEST_CONFIG_FILE**
:   Path to the conftest configuration file. The provider resolves a
    relative path from conftest's working directory when checking for a
    configuration that may define an ignore rule.

**COMPLYTIME_MACHINE_ID_FILE**
:   Path to the machine-id file used for host identification in
    evidence remarks. The file content is included in each
    Evidence.Source.Remarks field alongside the hostname so auditors
    can identify the host where the scan was executed.
    Default: **/etc/machine-id**

# EXTERNAL TOOLS

Requires the following tools at runtime:

**conftest**
:   Open Policy Agent test runner for structured data. Install conftest
    and make it available on **PATH**. The provider uses behavior-based
    compatibility rather than an exact-version runtime check. It does
    not require successful version detection or embedded module metadata;
    a source build reporting **dev** is not rejected for that reason.
    Input-safety, execution, and structured-output validation still apply.

    The real-tool regression suite has been exercised against **0.68.2**
    and **0.71.1**. It runs against any installed executable, without
    skipping based on version. These runs establish tested behavior, not
    compatibility with every future release. Local CI does not install
    conftest, so recurring changing-release coverage remains a tracked
    follow-up: <https://github.com/complytime/complyctl/issues/957>.

    To record the installed version when diagnosing an issue:

    ```bash
    conftest --version
    ```

    Releases: <https://github.com/open-policy-agent/conftest/releases>

**git**
:   Required for cloning remote repositories when **url** is specified.
    Provided by the **git** Fedora package.

# FILES

*.complytime/opa/*
:   Provider artifacts. All paths below are relative to the selected workspace
    root, not the Evaluation Log's directory or an auditor's current directory.

*.complytime/opa/results/\<repository\>/\<branch\>/evaluation-\<UTC-time\>-\<token\>/conftest.json*
:   Exact conftest JSON stdout for one remote target/branch evaluation.
    Time uses **20060102T150405Z** format; a random token distinguishes executions.

*.complytime/opa/results/\<local-target\>/evaluation-\<UTC-time\>-\<token\>/conftest.json*
:   Exact conftest JSON stdout for a local file or directory evaluation.
    Display labels are bounded grouping hints, not unique identities.

*metadata.json* (beside each retained result)
:   Best-effort schema-v1 collection-start, non-secret target/branch/scan-path
    and available policy/namespace/requirement-mapping context. Paths describe
    provenance, not archived policy or input bytes. Metadata is supporting output,
    not an evidence inventory or digest authority.

Legacy normalized per-target summaries are no longer written, including on
errors. Existing summary files remain untouched and are not current results.

*.complytime/opa/generated/\<content-key\>/scan-config.json*
:   Scan configuration written by the **Generate** RPC. Records matched
    requirement IDs, the Rego-to-Gemara reverse mapping, and the
    resolved policy directory path.

*.complytime/opa/generated/generate-index.json*
:   Maps each generated target to its content-keyed scan configuration.

*\<policy-directory\>/complytime-mapping.json*
:   Mapping file shipped in the OCI policy bundle or complypack. Maps
    Gemara requirement IDs to Rego policy namespaces. Required for
    requirement-scoped evaluation.

*.complytime/opa/repos/*
:   Replaceable working clones, not retained evidence or historical snapshots.
    Used when **url** targets are selected.
    Each repository is cloned into a subdirectory named by the sanitized
    URL and branch.

*.complytime/opa/policy/*
:   Directory for downloaded OPA policy bundles (from OCI registries
    or complypack content).

# EVIDENCE

Use the Gemara Evaluation Log to identify and verify each assessment's retained
result file. The log alone defines the evidence set: files beside an artifact,
including metadata, do not become evidence implicitly.

**Type**
:   **ConftestResult** — retained conftest output for remote repositories,
    local files and local directories, uniformly.

**Production**
:   The provider invokes **conftest test** with namespace-scoped policies
    against the input path (a cloned repository directory or a local
    filesystem path). It preserves exact JSON stdout as **conftest.json**,
    parses and hashes those stored bytes and keeps stderr separate. Valid failing
    compliance results remain evidence because conftest runs with **--no-fail**.
    Required allocation/write/close/parse/digest failures publish no affected
    assessments. Metadata failures warn; ownership/mode failures fail the RPC.

**EvaluationLog representation**
:   Each assessment log carries an Evidence entry with:

    - **Source.Coordinate**: workspace-root-relative path to retained **conftest.json**
    - **Source.Digest**: SHA256 (**sha256:\<hex\>**) of the stored output for every input type
    - **ID**: **conftest_\<token\>**
    - **Source.ReferenceID**: **conftest-result_\<token\>**, matching its mapping reference
    - **CollectedAt**: UTC collection start, also used in the directory and metadata

    A mapping reference's escaped absolute **file://** URL identifies the same file.
    Multiple assessments reuse one artifact/reference; different evaluations have
    different IDs even for identical bytes. These coordinate/ID and OPA result-file
    semantics are breaking changes, without moving fixed-path aliases.

Complyctl selects **--workspace**, then **COMPLYTIME_WORKSPACE**, then invocation
directory and supplies that root through existing RPC variables. Establish that
root, read the log's mappings, resolve each coordinate against the root (or decode
the absolute file URL), and compare the file's SHA256 with the recorded digest.
URI escapes such as **%20** and **%25** are not literal filename characters.
No packaging or storage integration is required for in-place verification.

Preservation is always on, with no automatic cleanup, retention settings or
deduplication. Storage grows with each result and supporting metadata, including
identical results and failed attempts. Local inputs stay in place; remote clones
remain replaceable. Embedded conftest input filenames are provenance rather than
retained evidence links. Inputs and policies are not archived; unchanged findings
do not prove unchanged inputs, and historical re-evaluation is not guaranteed.

Evaluation directories use mode **0700** and files **0600**, owned by the provider's
effective execution UID/GID. Sudo and direct-root scans remain root-owned; the
original user needs authorized elevated reading/hashing. Context parents must
permit traversal and the selected workspace must be writable. Permission or
disk-space failures can leave unreferenced partial files. Fix account access or
capacity before retrying; the provider does not transfer owners, widen historical
permissions or promise access to arbitrary other users.

Uploading the Evaluation Log does not transfer its evidence bytes. Customer-owned
storage and transfer integrations must preserve both the log and log-selected files,
keep them retrievable and verify transferred bytes against the logged digests.
Collection, validation of collected copies and optional packaging are separate
future complyctl capabilities, not implemented here. Any relocation must map moved
artifacts to original references without modifying the log. Coordinates work against
a copied workspace preserving the layout; absolute URLs remain specific to the
originating filesystem and do not relocate automatically. Storage backends and
long-term retention remain customer-owned.

After upgrade, legacy summaries/fixed-path artifacts stop being refreshed; existing
files and historical logs remain untouched and are not migrated. Already overwritten
evidence cannot be recovered. Rolling back to an older binary resumes overwrite
behavior for later scans while retained directories remain readable by authorized users.

# EXIT CODES

This provider is a gRPC subprocess managed by **complyctl**. Exit codes
follow the hashicorp/go-plugin protocol and are not user-facing.

# SEE ALSO

**complyctl**(1), **complyctl-provider-openscap**(1),
**complyctl-provider-ampel**(1), **conftest**(1)

Project: <https://github.com/complytime/complytime-providers>

# COPYRIGHT

Apache-2.0
