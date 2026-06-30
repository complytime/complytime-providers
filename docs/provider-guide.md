# Provider Development Guide

Providers extend `complyctl` by implementing the gRPC interface defined in
`github.com/complytime/complyctl/pkg/provider`. Each provider is a standalone
binary discovered by complyctl at runtime using the `complyctl-provider-`
executable prefix.

## How Providers Work

Providers communicate with complyctl via gRPC using the
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) subprocess model.
When a complyctl command runs, it:

1. Discovers provider binaries in `~/.complytime/providers/` (prefix: `complyctl-provider-`)
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

### Complypack Content Path

The `GenerateRequest` includes an optional `ComplypackContentPath` field. When
complyctl has a cached complypack for the provider's evaluator ID, it sets this
field to the local cache path (typically a `content.tar.gz` archive under
`~/.complytime/complypacks/<evaluator-id>/<version>/`).

Providers that consume `ComplypackContentPath` should follow this resolution
order in their `Generate` implementation:

1. **ComplypackContentPath** (if non-empty) -- use the provided path directly.
   The path may be a directory or a tar.gz archive; providers that receive an
   archive must extract it before reading content files.
2. **Provider-specific fallback** (e.g., `opa_bundle_ref` + pull) -- use when
   no complypack is available.
3. **Error** -- when neither source is available.

This field is additive: existing provider-specific workflows (such as the OPA
provider's `opa_bundle_ref` + `conftest pull`) remain unchanged when no
complypack is provided.

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
