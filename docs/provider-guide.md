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

## Building Providers

```bash
make build
```

This produces both provider binaries in `bin/`.

## See Also

- [complyctl](https://github.com/complytime/complyctl) — the CLI that discovers and invokes providers
- [compliance-to-policy-go](https://github.com/oscal-compass/compliance-to-policy-go) — upstream OSCAL framework
