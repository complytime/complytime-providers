## Why

The complytime-providers package is being proposed for inclusion in Fedora.
The current RPM spec was written for internal/COPR builds and does not
conform to the Fedora Go Packaging Guidelines. Key gaps include: missing
vendored dependency license aggregation (Fedora requires the `License:`
field to enumerate all vendored licenses), raw `go build` commands
instead of `%gobuild` macros (bypasses hardening flags and debuginfo
generation), and manual source/vendor handling instead of the
`go-vendor-tools` workflow that Fedora Go packages use. A parallel
modernization was completed for the `complyctl` spec on the
`opsx/rpm-spec-modernization` branch and serves as the reference
for this work.

## What Changes

- Adopt `go-vendor-tools` for vendored license verification, replacing
  manual `%license` directives with `%go_vendor_license_filelist`
- Create `go-vendor-tools.toml` configuration with license detection
  overrides for ambiguous vendored licenses (e.g., `gopkg.in/yaml.v3`)
- Replace raw `go build` commands with `%gobuild` macro (enables RPM
  hardening flags, proper debuginfo generation, build ID injection)
- Remove `%global debug_package %{nil}` (no longer needed with
  `%gobuild`)
- Switch source handling from raw GitHub URL to forge macros
  (`%{gosource}`) with separate vendor archive (`Source1`)
- Aggregate vendored dependency licenses into the `License:` field
  (`Apache-2.0 AND BSD-3-Clause AND ISC AND MIT AND MPL-2.0`)
- Replace `go test` with `%gocheck2` macro and add `%bcond check`
  toggle
- Switch to `%autorelease` for automatic release numbering
- Introduce a main `complytime-providers` package to hold shared
  license files and documentation; sub-packages require it via
  `Requires: %{name} = %{version}-%{release}`
- Bump `Requires: complyctl` from `>= 0.0.8` to `>= 1.0.0`
- Add Fedora 43 Go 1.25 compatibility workaround (conditional
  `go.mod`/`vendor/modules.txt` patching)
- Add `%generate_buildrequires` section with
  `%go_vendor_license_buildrequires`

## Capabilities

### New Capabilities

None. This change modernizes packaging artifacts only.

### Modified Capabilities

None. No spec-level behavior changes. The `skip_specs: true` marker
is set in `.openspec.yaml` because this is a pure packaging/tooling
change with no behavioral impact.

## Impact

- **Files modified**: `complytime-providers.spec`
- **Files created**: `go-vendor-tools.toml`
- **Dependencies**: Adds build-time dependency on `go-vendor-tools`
  (Fedora-packaged); removes explicit `golang >= 1.26` and
  `go-rpm-macros` build requirements (pulled transitively)
- **Runtime**: No change to binary behavior. Provider discovery,
  install paths, and binary names remain identical.
- **Cross-package**: Requires `complyctl >= 1.0.0` RPM to be
  available (the `complyctl` spec on `opsx/rpm-spec-modernization`
  targets v1.0.0). The `complyctl` RPM owns
  `/usr/libexec/complytime/` and `/usr/libexec/complytime/providers/`;
  provider sub-packages install binaries into that directory.
