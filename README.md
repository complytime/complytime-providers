# complytime-providers

Compliance-scanning provider plugins for the [complyctl](https://github.com/complytime/complyctl) CLI. Each provider implements the complyctl gRPC plugin interface (`hashicorp/go-plugin`) with three RPCs: **Describe**, **Generate**, and **Scan**.

## Providers

| Provider | Binary | Description |
|:---------|:-------|:------------|
| [openscap-provider](cmd/openscap-provider/) | `complyctl-provider-openscap` | XCCDF-based system compliance scanning using OpenSCAP |
| [ampel-provider](cmd/ampel-provider/) | `complyctl-provider-ampel` | In-toto attestation-based policy verification using AMPEL and snappy |
| [opa-provider](cmd/opa-provider/) | `complyctl-provider-opa` | OPA/conftest-based configuration policy evaluation |

## Build

Requires Go 1.26 or higher.

```bash
make build                    # Build all provider binaries to bin/
make build-openscap-provider  # Build openscap provider only
make build-ampel-provider     # Build ampel provider only
make build-opa-provider       # Build opa provider only
```

Binaries are output to `bin/`.

## Install

Copy the built binary to the complyctl providers directory:

```bash
mkdir -p ~/.local/share/complytime/providers
cp bin/complyctl-provider-* ~/.local/share/complytime/providers/
```

Providers are discovered automatically by complyctl using the `complyctl-provider-` naming convention.

## Test

```bash
make test    # Run all unit tests
make lint    # Run golangci-lint
```

## Project Structure

```text
cmd/
├── openscap-provider/   # XCCDF-based system scanning
├── ampel-provider/      # In-toto attestation verification
└── opa-provider/        # OPA/conftest policy evaluation
internal/
├── archive/             # Shared tar.gz extraction with security constraints
└── complytime/testdata/ # Shared XML test fixtures
docs/                    # Documentation
```

Each provider is self-contained under `cmd/<name>-provider/` with its own subpackage hierarchy. Shared utilities live in `internal/`: `internal/archive/` provides secure archive extraction for complypack content, and `internal/version/` provides build-time version injection.

## Documentation

- [Provider Development Guide](docs/provider-guide.md) --
  developing providers.
- [Dev Testing Environment](docs/dev-testing-environment.md) --
  devcontainer setup for interactive testing during PR review.

## License

Apache-2.0. See [LICENSE](LICENSE) for details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. All changes require a feature branch and PR with review from at least two maintainers.
