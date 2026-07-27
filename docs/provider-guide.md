# Provider Development Guide

Providers extend `complyctl` by implementing the gRPC interface defined in
`github.com/complytime/complyctl/pkg/provider`. Each provider is a standalone
binary discovered by complyctl at runtime using the `complyctl-provider-`
executable prefix.

## How Providers Work

Providers communicate with complyctl via gRPC using the
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) subprocess model.
When a complyctl command runs, it:

1. Discovers provider binaries in `~/.local/share/complytime/providers/` (prefix: `complyctl-provider-`)
2. Reads each provider's manifest (`c2p-<name>-manifest.json`) for metadata
3. Launches the provider binary as a subprocess
4. Communicates via gRPC over a local socket managed by go-plugin

## Provider Interface

Every provider must implement the `provider.Provider` interface from
`github.com/complytime/complyctl/pkg/provider`:

```go
type Provider interface {
    Describe(ctx context.Context, req *DescribeRequest) (*DescribeResponse, error)
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
}
```

The `Describe` RPC reports the provider's identity, health, version, and
declared variable requirements. `Generate` converts the OSCAL assessment plan
into provider-specific policy artifacts. `Scan` invokes the underlying policy
engine and returns assessment results.

### Result Semantics

The `Scan` RPC returns assessment steps, each carrying a `Result` value.
Providers MUST map their engine-specific outcomes to the correct `Result`
constant so that complyctl can produce accurate Gemara and SARIF reports.

| Result | Proto value | Meaning | Gemara mapping |
|:---|:---|:---|:---|
| `ResultPassed` | `RESULT_PASSED` | Control requirement satisfied | `Passed` |
| `ResultFailed` | `RESULT_FAILED` | Control requirement violated | `Failed` |
| `ResultSkipped` | `RESULT_SKIPPED` | Rule evaluated but does not apply to the target | `NotApplicable` |
| `ResultError` | `RESULT_ERROR` | Evaluation could not complete (tool error, timeout) | `Unknown` |

**NotApplicable vs NotRun** — `RESULT_SKIPPED` means the rule was evaluated
and found not applicable to the target environment. It is distinct from a rule
that was never evaluated at all. Rules that were not selected for evaluation
(e.g. XCCDF `notselected`) should be omitted from scan results entirely, not
reported as `RESULT_SKIPPED`.

For XCCDF-based providers, the mapping from XCCDF result strings is:

| XCCDF result | Provider Result | Notes |
|:---|:---|:---|
| `pass`, `fixed` | `ResultPassed` | Requirement met |
| `fail` | `ResultFailed` | Requirement violated |
| `notapplicable` | `ResultSkipped` | Rule does not apply to this target |
| `error`, `unknown` | `ResultError` | Evaluation failure |
| `notselected` | *(omit)* | Rule not selected in profile; no assessment emitted |

### GenerateRequest

The `GenerateRequest` struct carries configuration from complyctl to your
provider's `Generate` RPC:

```go
type GenerateRequest struct {
    GlobalVariables       map[string]string            // user-defined key/value pairs from the complyctl config
    Configuration         []AssessmentConfiguration    // assessment plan entries selected for this provider
    TargetVariables       map[string]string            // target-scoped overrides for provider variables
    ComplypackContentPath string                       // path to cached complypack content (see below)
}
```

## Complypack Support

Complypacks are opaque content bundles (`content.tar.gz`) identified by
evaluator ID and fetched by `complyctl get`. They allow content authors to
package provider-specific policy files, data, and configuration into a single
distributable archive.

When a complypack exists for your provider's evaluator ID, complyctl sets
`GenerateRequest.ComplypackContentPath` to the local cache path (typically
`~/.cache/complytime/complypacks/<evaluator-id>/<version>/content.tar.gz`). When no
complypack is available the field is an empty string.

### Adoption pattern

Check `ComplypackContentPath` first, fall back to your existing content
source, and error when neither is available:

```go
func (s *MyProvider) resolvePolicyDir(
	req *provider.GenerateRequest,
) (string, error) {
	// 1. Complypack content (highest priority)
	if req.ComplypackContentPath != "" {
		return resolveComplypackPath(req.ComplypackContentPath)
	}

	// 2. Provider-specific fallback (e.g. variable-driven bundle pull)
	if ref := req.GlobalVariables["my_bundle_ref"]; ref != "" {
		return pullBundle(ref)
	}

	// 3. No content available
	return "", fmt.Errorf(
		"either a complypack or my_bundle_ref variable is required")
}
```

### Provider responsibilities

- **Extraction**: the path may point to a `tar.gz` archive or an already-
  extracted directory. If it is an archive, extract it before reading files.
- **Zip-slip protection**: validate that extracted file paths do not escape the
  target directory. Use `filepath.Rel` or equivalent to reject paths containing
  `..` components.
- **Content format**: the archive layout is provider-defined. Document what
  your provider expects (e.g. a `policies/` directory, Rego files, XCCDF
  tailoring).
- **Backward compatibility**: always check for an empty string before using the
  path. Providers must continue to work without a complypack. For example, the
  OPA provider's `opa_bundle_ref` + `conftest pull` workflow remains unchanged
  when no complypack is provided.

## Entry Point

Each provider binary calls `provider.Serve(impl)` in `main()`:

```go
package main

import (
    "github.com/complytime/complyctl/pkg/provider"
    "github.com/example/myprovider/server"
)

func main() {
    provider.Serve(&server.MyProvider{})
}
```

## Manifest File

Each provider ships a JSON manifest file that complyctl reads before launching
the provider subprocess. The manifest declares the provider ID, version, binary
name, and supported configuration parameters.

Example (`c2p-openscap-manifest.json`):

```json
{
  "metadata": {
    "id": "openscap",
    "description": "OpenSCAP provider for complyctl",
    "version": "0.1.0",
    "types": ["pvp"]
  },
  "executablePath": "complyctl-provider-openscap",
  "sha256": "<sha256-of-binary>",
  "configuration": [
    {
      "name": "workspace",
      "description": "Directory for writing provider artifacts",
      "required": true
    }
  ]
}
```

## Providers in This Repository

| Provider | Binary | Description |
|:---|:---|:---|
| `cmd/openscap-provider` | `complyctl-provider-openscap` | OpenSCAP-based compliance scanning |
| `cmd/ampel-provider` | `complyctl-provider-ampel` | AMPEL-based policy evaluation |
| `cmd/opa-provider` | `complyctl-provider-opa` | OPA/conftest-based policy evaluation |

## Building Providers

```bash
make build
```

This produces both provider binaries in `bin/`.

## See Also

- [complyctl](https://github.com/complytime/complyctl) — the CLI that discovers and invokes providers
- [compliance-to-policy-go](https://github.com/oscal-compass/compliance-to-policy-go) — upstream OSCAL framework
