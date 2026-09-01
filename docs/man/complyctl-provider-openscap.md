% COMPLYCTL-PROVIDER-OPENSCAP(1) ComplyTime Manual
% Marcus Burghardt <maburgha@redhat.com>
% September 2026

# NAME

complyctl-provider-openscap - OpenSCAP scanning provider for complyctl

# SYNOPSIS

**complyctl-provider-openscap**

# DESCRIPTION

**complyctl-provider-openscap** is a gRPC plugin for **complyctl**(1) that
provides compliance scanning capabilities using the OpenSCAP engine. It is
not invoked directly by users; **complyctl** discovers and launches it
automatically when the provider is installed.

The provider receives assessment configurations from **complyctl**,
generates XCCDF tailoring files matching the requested profiles, executes
scans via the **oscap** command-line tool, and returns structured results.
It supports automatic datastream selection via CPE-based matching against
the host system.

Communication with **complyctl** uses the hashicorp/go-plugin gRPC
interface, implementing three RPCs: **Describe**, **Generate**, and
**Scan**.

# CONFIGURATION

Configuration is provided through variables defined in assessment plan
targets. These variables are passed to the provider via **complyctl** and
are not set manually by the user.

**profile** (required)
:   XCCDF profile ID to evaluate. Must match the pattern
    **[a-zA-Z0-9-_.]+**. Examples: **cis_server_l1**,
    **xccdf_org.ssgproject.content_profile_cis**.

**datastream** (optional)
:   Absolute path to an SSG XCCDF datastream XML file. When omitted, the
    provider auto-detects the appropriate datastream by matching the
    system's CPE (from the os-release file) against CPE metadata embedded
    in the SSG datastream files.

# ENVIRONMENT

**SSG_CONTENT_DIR**
:   Directory containing SSG datastream XML files. Used during
    auto-detection when **datastream** is not specified.
    Default: **/usr/share/xml/scap/ssg/content**

**OS_RELEASE_FILE**
:   Path to the os-release file used for CPE extraction during
    datastream auto-detection.
    Default: **/etc/os-release**

# EXTERNAL TOOLS

Requires **oscap** (provided by the **openscap-scanner** package) at
runtime. The provider locates **oscap** via **PATH** lookup.

The **scap-security-guide** package provides the SSG datastream files
used for evaluation.

# FILES

*~/.local/share/complytime/openscap/*
:   Provider workspace directory for scan artifacts and tailoring files.

# EXIT CODES

This provider is a gRPC subprocess managed by **complyctl**. Exit codes
follow the hashicorp/go-plugin protocol and are not user-facing.

# SEE ALSO

**complyctl**(1), **complyctl-provider-ampel**(1),
**complyctl-provider-opa**(1), **oscap**(8)

Project: <https://github.com/complytime/complytime-providers>

# COPYRIGHT

Apache-2.0
