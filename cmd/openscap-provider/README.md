# OpenSCAP Provider QuickStart

End-to-end testing of the Gemara-native workflow with the OpenSCAP plugin.

## Prerequisites

| Requirement | Purpose |
|:---|:---|
| Go 1.24+ | Build `complyctl` and `openscap-provider` |
| `openscap-scanner` | Provides the `oscap` CLI used by the provider |
| `scap-security-guide` | Provides SCAP Datastream XML files in `/usr/share/xml/scap/ssg/content/` |
| RHEL, CentOS, or Fedora | SSG datastreams are distribution-specific; the provider auto-detects via `/etc/os-release` |

**Install system dependencies (Fedora/RHEL/CentOS):**

```bash
sudo dnf install -y openscap-scanner scap-security-guide
```

**Verify:**

```bash
oscap --version
ls /usr/share/xml/scap/ssg/content/ssg-*-ds.xml
```

## Sourcing Gemara-Compatible Profile Content

The mock registry in Step 4 requires a Gemara policy layer containing real XCCDF rule short names. These rule-to-control mappings originate from the [ComplianceAsCode/oscal-content](https://github.com/ComplianceAsCode/oscal-content/tree/main) repository, which is the OSCAL-format representation of upstream [ComplianceAsCode/content](https://github.com/ComplianceAsCode/content).

**Repository structure:**

| Directory | Content | Gemara Equivalent |
|:---|:---|:---|
| `catalogs/` | OSCAL control catalogs (CIS, STIG, PCI-DSS, OSPP, etc.) | `ControlCatalog` — the catalog layer (`application/vnd.gemara.catalog.v1+yaml`) |
| `profiles/` | OSCAL profiles scoping a catalog to a platform + level | Determines which controls are in scope for a policy |
| `component-definitions/` | OSCAL component-definitions mapping XCCDF rules to controls | `Policy` — the policy/assessment layer (`application/vnd.gemara.policy.v1+yaml`) |

**Available platforms and profiles (subset):**

| Platform | Example Profiles |
|:---|:---|
| Fedora | `cis_fedora-l1_server`, `cis_fedora-l2_workstation`, `cusp_fedora-default` |
| RHEL 8 | `stig_rhel8`, `cis_rhel8`, `hipaa` |
| RHEL 9 | `stig_rhel9`, `cis_rhel9`, `ccn_rhel9` |
| RHEL 10 | `anssi-*`, `bsi_sys_1_1_rhel10-*`, `cis_rhel10`, `ospp` |
| OCP 4 | `nist_ocp4-*`, `cis_ocp-*`, `stig_ocp4-*`, `pcidss_*` |

**Why this matters:** The component-definitions contain the mapping from compliance control IDs to XCCDF rule short names (e.g., CIS control `1.1.1.1` maps to rule `kernel_module_cramfs_disabled`). These rule short names are what the OpenSCAP plugin uses as `RequirementID` values in the policy layer.

### Conversion workflow (OSCAL to Gemara policy layer)

The OSCAL component-definitions are not directly consumable as Gemara policy layers. The conversion path is:

```
ComplianceAsCode/oscal-content
  └── component-definitions/<platform>/<profile>/component-definition.json
        │
        │  Extract: implemented-requirements[].rules[]
        │  Each rule has an XCCDF rule short name
        │
        ▼
Gemara Policy Layer (YAML)
  - id: <xccdf_rule_short_name>       # from OSCAL implemented-requirement
    evaluator_id: openscap             # routes to the OpenSCAP plugin
    parameters:
      workspace: ./.complytime/scan
      profile: <ssg_profile_short_name>
      policy: tailoring.xml
      arf: arf-results.xml
      results: xccdf-results.xml
```

For testing, manually extract a few rule short names from the OSCAL content (or from `oscap info` output) and use them directly in the policy layer seed data (Step 4). A production workflow would automate this conversion via [complyscribe](https://github.com/complytime/complyscribe).

### Example: extracting rules from OSCAL content

Clone the repository and inspect a Fedora CIS component-definition:

```bash
git clone https://github.com/ComplianceAsCode/oscal-content.git
cat oscal-content/component-definitions/fedora/fedora-cis_fedora-l1_server/component-definition.json \
  | python3 -m json.tool | grep -A2 '"rule-id"'
```

The rule IDs extracted here (stripped of the `xccdf_org.ssgproject.content_rule_` prefix) are the values to use in the Gemara policy layer's `id` field.

## Architecture Overview

```
complytime.yaml
       │
       ▼
   complyctl get ──► Mock OCI Registry (localhost:8765)
       │                  │
       │                  ▼
       │             Policy layer with evaluator_id: openscap
       │             + XCCDF rule short names as requirement IDs
       │
   complyctl generate ──► complyctl-provider-openscap (gRPC)
       │                       │
       │                       ▼
       │                  Reads SSG Datastream
       │                  Generates XCCDF tailoring file
       │                  Generates remediation scripts
       │
   complyctl scan ──► complyctl-provider-openscap (gRPC)
       │                       │
       │                       ▼
       │                  Runs: oscap xccdf eval --tailoring-file ...
       │                  Parses ARF results
       │                  Returns AssessmentLog entries
       ▼
   ./.complytime/scan/
       evaluation-log-*.json
       assessment-results-*.json
```

## Step 1: Build

```bash
make build
```

Produces two binaries:

| Binary | Path |
|:---|:---|
| `complyctl` | `bin/complyctl` |
| `openscap-provider` | `bin/openscap-provider` |

## Step 2: Install the OpenSCAP Provider

The provider discovery mechanism scans `~/.complytime/providers/` for executables matching `complyctl-provider-<evaluator-id>`. The build outputs `openscap-provider` — it must be copied with the correct name.

```bash
mkdir -p ~/.complytime/providers
cp bin/openscap-provider ~/.complytime/providers/complyctl-provider-openscap
chmod +x ~/.complytime/providers/complyctl-provider-openscap
```

**Verify discovery works:**

```bash
ls -la ~/.complytime/providers/complyctl-provider-openscap
```

## Step 3: Identify Available SSG Profile and Rules

The OpenSCAP plugin needs real XCCDF rule short names and a valid profile ID from the installed datastream. Identify what's available on your system.

**Find your datastream:**

```bash
# The provider auto-detects this, but you need it to seed the registry
ls /usr/share/xml/scap/ssg/content/ssg-*-ds.xml
```

Example: `/usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml`

**List available profiles:**

```bash
oscap info /usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml
```

Pick a profile ID (the short name after `xccdf_org.ssgproject.content_profile_`). The pre-seeded policy uses `cis_workstation_l1`:

| Profile Short Name | Description |
|:---|:---|
| `cis_workstation_l1` | CIS Fedora Benchmark - Level 1 Workstation **(used by seed data)** |
| `cis_workstation_l2` | CIS Fedora Benchmark - Level 2 Workstation |
| `cis_server_l1` | CIS Fedora Benchmark - Level 1 Server |
| `standard` | Standard System Security Profile |
| `ospp` | Protection Profile for General Purpose OS |

**Verify the CIS L1 Workstation profile exists:**

```bash
oscap info /usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml 2>&1 \
  | grep cis_workstation_l1
```

**List rules in the profile:**

```bash
oscap xccdf eval --profile cis_workstation_l1 --progress \
  /usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml 2>&1 | head -20
```

Rule short names appear as the text before `:pass` or `:fail` (e.g., `kernel_module_cramfs_disabled`, `package_libselinux_installed`). These match the `id` values in the pre-seeded policy layer.

## Step 4: Mock Registry Seed Data (Pre-configured)

The mock registry ships with a pre-seeded **CIS Fedora Linux Level 1 Workstation** policy derived from the [ComplianceAsCode/oscal-content](https://github.com/ComplianceAsCode/oscal-content/tree/main/component-definitions/fedora/fedora-cis_fedora-l1_workstation) component-definition. No manual edits required.

**Policy ID:** `policies/cis-fedora-l1-workstation`

The seed data lives in `cmd/mock-oci-registry/testdata/` and is embedded at compile time:

| File | Gemara Layer | Content |
|:---|:---|:---|
| `cis-fedora-l1-workstation-catalog.yaml` | Layer 2 — ControlCatalog | 198 CIS controls with titles |
| `cis-fedora-l1-workstation-policy.yaml` | Layer 3 — Policy | 275 XCCDF rules, each with `evaluator_id: openscap` |

**Catalog layer structure (Layer 2):**

```yaml
id: cis-fedora-l1-workstation
title: CIS Fedora Linux - Level 1 Workstation
controls:
  - id: cis_fedora_1-1.1.1
    title: "Ensure Cramfs Kernel Module Is Not Available (Automated)"
  - id: cis_fedora_1-3.1.1
    title: "Ensure SELinux Is Installed (Automated)"
  # ... 219 controls total
```

**Policy layer structure (Layer 3):**

```yaml
- id: kernel_module_cramfs_disabled
  evaluator_id: openscap
  parameters:
    workspace: ./.complytime/scan
    profile: cis_workstation_l1
    policy: tailoring.xml
    arf: arf-results.xml
    results: xccdf-results.xml
    control_id: cis_fedora_1-1.1.1
- id: package_libselinux_installed
  evaluator_id: openscap
  parameters:
    workspace: ./.complytime/scan
    profile: cis_workstation_l1
    policy: tailoring.xml
    arf: arf-results.xml
    results: xccdf-results.xml
    control_id: cis_fedora_1-3.1.1
# ... 315 rules total
```

**Key fields in the policy layer:**

| Field | Value | Purpose |
|:---|:---|:---|
| `id` | XCCDF rule short name (e.g., `kernel_module_cramfs_disabled`) | Used as `RequirementID` — must match a rule in the SSG datastream |
| `evaluator_id` | `openscap` | Routes the request to `complyctl-provider-openscap` |
| `parameters.workspace` | `./.complytime/scan` | Working directory for plugin output |
| `parameters.profile` | `cis_workstation_l1` | SSG profile short name — must exist in the auto-detected datastream |
| `parameters.policy` | `tailoring.xml` | Filename for tailoring XML — written by Generate, consumed by Scan |
| `parameters.arf` | `arf-results.xml` | Filename for ARF results — written by `oscap xccdf eval` |
| `parameters.results` | `xccdf-results.xml` | Filename for XCCDF results — written by `oscap xccdf eval` |
| `parameters.control_id` | CIS control ID (e.g., `cis_fedora_1-1.1.1`) | Maps the rule back to its parent control |

**Important:** All entries share the same `parameters` values (except `control_id`). The provider merges parameters from all assessment configurations, and later entries override earlier ones for the same key.

## Step 5: Start the Mock Registry

```bash
make mock-registry
```

Verify the CIS Fedora policy is served:

```bash
curl -s http://localhost:8765/v2/policies/cis-fedora-l1-workstation/tags/list | python3 -m json.tool
```

Expected:

```json
{
    "name": "policies/cis-fedora-l1-workstation",
    "tags": ["latest", "v1.0.0"]
}
```

## Step 6: Create Workspace Config

A sample config is provided in testdata. Copy it to your working directory:

```bash
cp cmd/mock-oci-registry/testdata/sample-complytime.yaml ./complytime.yaml
```

The config contents (for reference):

```yaml
registry:
  url: http://localhost:8765
policies:
  - id: policies/cis-fedora-l1-workstation
    evaluator_config:
      openscap:
        workspace: ./.complytime/scan
        profile: cis_workstation_l1
        policy: tailoring.xml
        arf: arf-results.xml
        results: xccdf-results.xml
targets:
  - id: local
    policy_ids:
      - policies/cis-fedora-l1-workstation
```

## Step 7: Fetch Policies

```bash
bin/complyctl get
```

**Verify:**

```bash
ls ~/.complytime/policies/policies/cis-fedora-l1-workstation/
cat ~/.complytime/state.json | python3 -m json.tool
```

## Step 8: Generate

```bash
bin/complyctl generate --policy-id policies/cis-fedora-l1-workstation
```

This dispatches the Generate RPC to the OpenSCAP plugin, which:
1. Loads the merged parameters (`workspace`, `profile`, `datastream`, `policy`, `arf`, `results`)
2. Auto-detects the datastream from `/usr/share/xml/scap/ssg/content/` if not set
3. Validates that all rule IDs exist in the datastream
4. Generates a tailoring XML file extending the profile
5. Generates remediation scripts (bash, ansible, blueprint)

**Verify:**

```bash
ls ./.complytime/scan/openscap/policy/
ls ./.complytime/scan/openscap/remediations/
```

Expected: `tailoring.xml` in the policy directory, remediation scripts in the remediations directory.

## Step 9: Scan

Scanning requires root because `oscap xccdf eval` accesses system-level resources.

```bash
sudo bin/complyctl scan --policy-id policies/cis-fedora-l1-workstation
```

**Verify:**

```bash
ls ./.complytime/scan/
cat ./.complytime/scan/evaluation-log-*.json | python3 -m json.tool
```

**Additional output formats:**

```bash
sudo bin/complyctl scan --policy-id policies/cis-fedora-l1-workstation --format oscal
sudo bin/complyctl scan --policy-id policies/cis-fedora-l1-workstation --format pretty
sudo bin/complyctl scan --policy-id policies/cis-fedora-l1-workstation --format sarif
```

## Troubleshooting

| Symptom | Cause | Fix |
|:---|:---|:---|
| `plugin not found for evaluator ID: openscap` | Binary missing or wrong name in `~/.complytime/providers/` | Re-run Step 2; verify the file is named `complyctl-provider-openscap` and is executable |
| `could not determine a datastream file` | `scap-security-guide` not installed or `/etc/os-release` unrecognized | Install `scap-security-guide`; or set `datastream` parameter explicitly in the policy layer |
| `profile not found: xccdf_org.ssgproject.content_profile_cis_workstation_l1` | Profile not available in the detected datastream | Run `oscap info <datastream>` to list available profiles; update the `profile` parameter in the policy seed data |
| `rule(s) not found in datastream ... will be skipped` (WARN) | Rule short names in the policy layer don't exist in the installed datastream version | Informational only — skipped rules won't be evaluated. To resolve, update `scap-security-guide` or remove the rules from the policy layer |
| `no valid rules found in datastream` | Every rule in the policy layer is missing from the datastream | Verify `scap-security-guide` is installed and matches the policy's target platform |
| `absent openscap files ... Did you run the generate command?` | Scan invoked before Generate | Run `complyctl generate` first |
| `oscap error during evaluation: exit status 1` | `oscap` CLI failed (XML validation, missing deps) | Run `oscap` manually to see verbose error; check `--debug` output |
| `command not found: oscap` | `openscap-scanner` package not installed | `sudo dnf install openscap-scanner` |
| Permission denied during scan | `oscap xccdf eval` needs elevated privileges | Run scan with `sudo` |

## Data Flow Reference

```
Policy Layer (OCI Registry)
  │
  ├── id: kernel_module_cramfs_disabled ─── RequirementID (XCCDF rule short name)
  ├── evaluator_id: openscap            ─── Routes to complyctl-provider-openscap
  └── parameters:
        workspace: ./.complytime/scan         ─── Plugin working directory
        profile: cis_workstation_l1      ─── SSG profile short name
        policy: tailoring.xml            ─── Tailoring file name
        arf: arf-results.xml             ─── ARF output name
        results: xccdf-results.xml       ─── XCCDF results name
        control_id: cis_fedora_1-1.1.1   ─── Parent CIS control

Generate RPC
  │
  ├── mergeParameters() ─── Flattens all config parameters into one map
  ├── Config.LoadSettings() ─── Sets workspace, profile, policy, arf, results
  │     └── validate() ─── Auto-detects datastream from /etc/os-release + SSG
  ├── PolicyToXML() ─── Creates tailoring XML extending the SSG profile
  └── OscapGenerateFix() ─── Generates bash/ansible/blueprint remediations

Scan RPC
  │
  ├── ScanSystem() ─── Runs: oscap xccdf eval --profile cis_workstation_l1_complytime
  │                          --tailoring-file <policy> --results-arf <arf>
  ├── Parse ARF XML ─── Matches rule-results to requirement IDs via OVAL checks
  └── Returns AssessmentLog[] ─── One entry per requirement with PASSED/FAILED/SKIPPED
```

## Cleanup

```bash
rm -rf .complytime/scan complytime.yaml
rm -rf ~/.complytime/policies/policies/cis-fedora-l1-workstation
rm ~/.complytime/providers/complyctl-provider-openscap
# Kill mock registry (Ctrl+C in its terminal)
```
