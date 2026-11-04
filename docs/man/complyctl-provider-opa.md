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
:   Workspace base directory. The provider excludes this base joined with
    **.complytime**, as well as its own artifact directory when different.
    When absent, the provider uses its configured artifact directory.

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

The provider resolves the host workspace as the **workspace** base injected
by complyctl joined with **.complytime**. It also excludes its own artifact
directory, normally **.complytime** relative to the provider's working
directory, when that directory differs. For example, a host base of
*/srv/team* and a provider working directory of */srv/scan* create two
boundaries: */srv/team/.complytime* and */srv/scan/.complytime*. If the
host does not provide a workspace base, the provider uses its configured
artifact directory. The provider resolves both paths through symbolic links.
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
intended eligible branches. Previously written per-branch result files are not deleted.

Workspace exclusion does not prune directories from traversal. An
unreadable directory or broken link can still cause an operational error;
fix the path or its permissions before retrying. Preflight is not a
sandbox: concurrent filesystem changes between preflight and conftest
traversal can defeat isolation.

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
checks configuration in conftest's working directory and resolves a
relative **CONFTEST_CONFIG_FILE** from that directory.

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
:   Provider workspace directory. All paths below are relative to the
    current working directory.

*.complytime/opa/results/\<target\>-\<branch\>.json*
:   Per-target result JSON summarizing conftest findings for a single
    target and branch combination. For local targets, the branch
    component is omitted.

*.complytime/opa/generated/\<content-key\>/scan-config.json*
:   Scan configuration written by the **Generate** RPC. Records matched
    requirement IDs, the Rego-to-Gemara reverse mapping, and the
    resolved policy directory path.

*.complytime/opa/generated/generate-index.json*
:   Maps each generated target to its content-keyed scan configuration.

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
    - **Source.ReferenceID**: **conftest-input-\<target\>[-\<branch\>]**

    For regular-file inputs, **Source.Digest** contains the SHA-256
    hash (**sha256:\<hex\>**) of the file content at scan time. For
    directory inputs (cloned repositories), no digest is recorded.

# EXIT CODES

This provider is a gRPC subprocess managed by **complyctl**. Exit codes
follow the hashicorp/go-plugin protocol and are not user-facing.

# SEE ALSO

**complyctl**(1), **complyctl-provider-openscap**(1),
**complyctl-provider-ampel**(1), **conftest**(1)

Project: <https://github.com/complytime/complytime-providers>

# COPYRIGHT

Apache-2.0
