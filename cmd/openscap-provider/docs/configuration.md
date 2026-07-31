# openscap-provider Configuration

## Overview

The openscap-provider scans local systems against SCAP security policies
(CIS, STIG, HIPAA, OSPP, and others) using
[OpenSCAP](https://www.open-scap.org/) and the
[SCAP Security Guide](https://www.open-scap.org/security-policies/scap-security-guide/) (SSG).
It reads target configuration from `complytime.yaml` and auto-detects
the system's SCAP datastream from `/usr/share/xml/scap/ssg/content/`.

## Prerequisites

| Requirement | Purpose |
|-------------|---------|
| `openscap-scanner` | Provides the `oscap` CLI |
| `scap-security-guide` | Provides SCAP datastream XML files |
| Supported Linux distribution | SSG datastreams are distribution-specific (RHEL, CentOS Stream, Fedora, SUSE, openSUSE, Amazon Linux) |

```bash
sudo dnf install -y openscap-scanner scap-security-guide
```

## Configuration Reference

### Target variables

Each target in `complytime.yaml` uses the `variables` map to specify
the SSG profile to evaluate:

```yaml
targets:
  - id: <target-id>
    policies:
      - <policy-id>
    variables:
      profile: <ssg-profile-short-name>
      datastream: <path-to-datastream>     # optional
```

### Variable reference

| Variable | Required | Description |
|----------|----------|-------------|
| `profile` | Yes | SSG profile short name (e.g., `cis_workstation_l1`). The provider prepends `xccdf_org.ssgproject.content_profile_` automatically. |
| `datastream` | No | Absolute path to a SCAP datastream XML file. When omitted, auto-detected from `/usr/share/xml/scap/ssg/content/` based on `/etc/os-release`. |

### Available profiles

List profiles available on your system:

```bash
oscap info /usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml
```

Common Fedora profiles:

| Profile short name | Description |
|--------------------|-------------|
| `cis_workstation_l1` | CIS Fedora Benchmark - Level 1 Workstation |
| `cis_workstation_l2` | CIS Fedora Benchmark - Level 2 Workstation |
| `cis_server_l1` | CIS Fedora Benchmark - Level 1 Server |
| `standard` | Standard System Security Profile |

## Examples

### CIS Fedora L1 Workstation

```yaml
policies:
  - url: quay.io/complytime/policies-cis-fedora-l1-workstation:latest
    id: cis-fedora-l1

targets:
  - id: my-workstation
    policies:
      - cis-fedora-l1
    variables:
      profile: cis_workstation_l1
```

```bash
complyctl get
complyctl scan my-workstation
```

### CIS Fedora L1 Server

```yaml
policies:
  - url: quay.io/complytime/policies-cis-fedora-l1-server:latest
    id: cis-fedora-l1-server

targets:
  - id: my-server
    policies:
      - cis-fedora-l1-server
    variables:
      profile: cis_server_l1
```

### Custom datastream path

```yaml
targets:
  - id: my-system
    policies:
      - cis-fedora-l1
    variables:
      profile: cis_workstation_l1
      datastream: /opt/ssg/custom-fedora-ds.xml
```

## Notes

- Scanning requires elevated privileges (`sudo`) because `oscap xccdf eval`
  accesses system-level resources.
- The provider auto-detects the datastream by matching `/etc/os-release`
  against files in `/usr/share/xml/scap/ssg/content/`. Set `datastream`
  explicitly only when the auto-detection does not match your environment.
- During `generate`, the provider creates a tailoring XML file and
  remediation scripts in `.complytime/scan/openscap/`.
