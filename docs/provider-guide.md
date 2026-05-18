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

## Export Interface (Optional)

Providers that support evidence export implement the optional
`provider.Exporter` interface:

```go
type Exporter interface {
    Export(ctx context.Context, req *ExportRequest) (*ExportResponse, error)
}
```

Export is triggered when a user runs `complyctl scan --format otel`. After the
scan phase completes, complyctl calls `Export` on each provider that declared
support.

### Declaring Export Support

Set `SupportsExport: true` in your `DescribeResponse` and add a compile-time
interface assertion:

```go
var _ provider.Exporter = (*ProviderServer)(nil)
```

### ExportRequest and ExportResponse

The `ExportRequest` carries a `CollectorConfig` with the OTLP gRPC endpoint
and a pre-resolved bearer token (complyctl handles OIDC):

```go
type ExportRequest struct {
    Collector CollectorConfig
}

type CollectorConfig struct {
    Endpoint  string // host:port for the OTLP gRPC collector
    AuthToken string // Bearer token (resolved by complyctl)
}

type ExportResponse struct {
    Success       bool
    ExportedCount int32
    FailedCount   int32
    ErrorMessage  string
}
```

### Implementation Pattern

Both providers in this repository follow the same pattern:

1. **Read scan results from disk** — the `Export` method reads the results
   written by `Scan` from the workspace directory
2. **Convert to GemaraEvidence** — each finding is mapped to a
   `proofwatch.GemaraEvidence` struct with Gemara metadata and assessment log
   fields
3. **Emit via ProofWatch** — an OTLP gRPC log exporter, `sdklog.LoggerProvider`,
   and `ProofWatch` instance are created from the `CollectorConfig`, evidence
   records are emitted, and the provider is shut down (flushing buffered logs)
   before returning

Each provider has an `export/` package with two files:

- `export.go` — OTEL SDK setup (`NewEmitter` creates the exporter,
  LoggerProvider, and ProofWatch; `Shutdown` flushes and tears down)
- `convert.go` — provider-specific result-to-GemaraEvidence conversion

Key dependencies for export:

- `github.com/complytime/complybeacon/proofwatch` — GemaraEvidence type and
  OTEL log emission
- `github.com/gemaraproj/go-gemara` — Gemara metadata and assessment log types
- `go.opentelemetry.io/otel/sdk/log` — OTEL LoggerProvider
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc` — OTLP gRPC
  exporter

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
| `cmd/openscap-provider` | `complyctl-provider-openscap` | OpenSCAP-based compliance scanning (with OTLP export) |
| `cmd/ampel-provider` | `complyctl-provider-ampel` | AMPEL-based policy evaluation (with OTLP export) |
| `cmd/opa-provider` | `complyctl-provider-opa` | OPA/conftest-based policy evaluation |

## Building Providers

```bash
make build
```

This produces both provider binaries in `bin/`.

## See Also

- [complyctl](https://github.com/complytime/complyctl) — the CLI that discovers and invokes providers
- [compliance-to-policy-go](https://github.com/oscal-compass/compliance-to-policy-go) — upstream OSCAL framework
