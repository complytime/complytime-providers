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

No environment variables are read directly by this provider.

# EXTERNAL TOOLS

Requires **snappy** and **ampel** CLI tools at runtime. These tools are
not currently packaged in Fedora and must be installed separately. See
the Carabiner project for installation instructions:
<https://github.com/carabiner-dev>

# FILES

*~/.local/share/complytime/ampel/*
:   Provider workspace directory for scan configuration and policy files.

# EXIT CODES

This provider is a gRPC subprocess managed by **complyctl**. Exit codes
follow the hashicorp/go-plugin protocol and are not user-facing.

# SEE ALSO

**complyctl**(1), **complyctl-provider-openscap**(1),
**complyctl-provider-opa**(1)

Project: <https://github.com/complytime/complytime-providers>

# COPYRIGHT

Apache-2.0
