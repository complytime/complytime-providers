## ADDED Requirements

### Requirement: Version package

The repository SHALL provide an `internal/version` package with a single
exported `Version()` function that returns the build version string. The
canonical version format is without the `v` prefix (e.g., `0.1.0`). The
`Version()` function SHALL strip the `v` prefix if present in the injected
value to ensure consistent output regardless of the injection source.

#### Scenario: Version set via ldflags without prefix

- **WHEN** the binary is built with
  `-ldflags "-X .../internal/version.version=0.1.0"`
- **THEN** `version.Version()` returns `"0.1.0"`

#### Scenario: Version set via ldflags with v prefix

- **WHEN** the binary is built with
  `-ldflags "-X .../internal/version.version=v0.1.0"`
- **THEN** `version.Version()` returns `"0.1.0"` (prefix stripped)

#### Scenario: Version not set

- **WHEN** the binary is built without ldflags (or version is empty)
- **THEN** `version.Version()` returns `"0.0.0-unknown"`

### Requirement: Provider Describe uses injected version

All three provider `Describe` RPC responses SHALL use the version from
the `internal/version` package instead of a hardcoded string.

#### Scenario: OpenSCAP provider version

- **WHEN** the openscap provider binary is built with version `0.2.0`
- **THEN** the `DescribeResponse.Version` field returns `"0.2.0"`

#### Scenario: Ampel provider version

- **WHEN** the ampel provider binary is built with version `0.2.0`
- **THEN** the `DescribeResponse.Version` field returns `"0.2.0"`

#### Scenario: OPA provider version

- **WHEN** the opa provider binary is built with version `0.2.0`
- **THEN** the `DescribeResponse.Version` field returns `"0.2.0"`

### Requirement: Provider Describe tests use version package default

All three provider `server_test.go` files SHALL assert the version
package default value instead of a hardcoded version string, ensuring
tests pass both with and without ldflags injection.

#### Scenario: OpenSCAP Describe test

- **WHEN** `go test` runs without ldflags
- **THEN** the openscap Describe test asserts
  `resp.Version == "0.0.0-unknown"`

#### Scenario: Ampel Describe test

- **WHEN** `go test` runs without ldflags
- **THEN** the ampel Describe test asserts
  `resp.Version == "0.0.0-unknown"`

#### Scenario: OPA Describe test

- **WHEN** `go test` runs without ldflags
- **THEN** the opa Describe test asserts
  `resp.Version == "0.0.0-unknown"`

### Requirement: Makefile version injection

The Makefile build targets SHALL inject the version from the latest git
tag via `-ldflags`. The Makefile SHALL strip the `v` prefix from git
tags before injection.

#### Scenario: Build with existing tags

- **WHEN** the latest git tag is `v0.1.0` and `make build` is run
- **THEN** all three provider binaries report version `0.1.0` through
  `Describe`

#### Scenario: Build with no tags

- **WHEN** no git tags exist and `make build` is run
- **THEN** the version defaults to `0.0.0` (from Makefile fallback)

### Requirement: RPM spec version injection

The RPM spec `%build` section SHALL inject the spec `%{version}` into
the provider binaries via `-ldflags`.

#### Scenario: RPM build injects version

- **WHEN** the RPM spec `Version:` is `0.1.0` and the package is built
- **THEN** all three provider binaries report version `0.1.0` through
  `Describe`

### Requirement: GoReleaser version injection

The GoReleaser configuration SHALL inject the release tag version into
all three provider builds via `-ldflags` using GoReleaser's
`{{ .Version }}` template variable (which strips the `v` prefix from
the tag automatically).

#### Scenario: GoReleaser builds with tag version

- **WHEN** GoReleaser runs with `GORELEASER_CURRENT_TAG=v0.1.0`
- **THEN** all three provider binaries in the release archives report
  version `0.1.0` through `Describe`
