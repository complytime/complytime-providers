## Context

See proposal.md for motivation. The current `complytime-providers.spec`
was written for internal/COPR builds. Fedora Go packages must use the
`go-vendor-tools` ecosystem for vendored license handling, `%gobuild`
for hardened builds, and forge macros for source management. The
`complyctl` spec on `opsx/rpm-spec-modernization` was already modernized
and serves as the reference implementation.

Key constraints:
- complytime-providers builds **three** binaries (not one like complyctl)
- The spec produces only sub-packages (no main binary), but Fedora
  needs a main package to hold shared license files
- Fedora 43 ships Go 1.25; `go.mod` requires Go 1.26.5
- Provider binaries must install to
  `/usr/libexec/complytime/providers/` (owned by `complyctl` RPM)
- The `gopkg.in/yaml.v3` vendored license is a dual MIT+Apache-2.0
  file that `askalono` cannot auto-detect

### Goals

- Pass Fedora package review for Go packaging compliance
- Align with the modernized `complyctl` spec patterns
- Ensure `complyctl` + `complytime-providers` install cleanly
  together on Fedora and provider discovery works end-to-end
- Generate proper debuginfo packages (removing `%{nil}` override)

### Non-Goals

- Packaging `snappy`, `ampel`, or `conftest` for Fedora (external
  runtime deps for ampel/opa providers remain user-managed)
- Changing provider binary behavior or code
- Creating man pages for providers (can be added later)
- Adding `go_vendor_license` checks to GitHub Actions CI
  (`go-vendor-tools` is Fedora-specific tooling not available in
  standard GitHub Actions runners; license drift is caught at RPM
  build time via `%go_vendor_license_check`)

## Decisions

### D1: Main package as license holder (not meta-package)

The main `complytime-providers` package holds shared license files
(via `%go_vendor_license_filelist`) and `README.md`. It does NOT
pull in all sub-packages.

**Why**: Sub-packages have different runtime dependency profiles
(`openscap-scanner + scap-security-guide` vs `git` vs nothing).
A meta-package would force unnecessary dependencies on users who
only want one provider.

**Alternative considered**: Attach `%go_vendor_license_filelist` to
one arbitrary sub-package. Rejected because it creates asymmetry
between sub-packages and the chosen sub-package would need to be
installed even when the user wants a different provider.

### D2: Three `%gobuild` calls for three binaries

Each provider binary gets its own `%gobuild` invocation in `%build`.
The `%gobuild` macro supports `-o <path> <import-path>` syntax,
identical to `go build`, so three calls are straightforward.

**Why**: `%gobuild` is the Fedora-standard macro that applies PIE,
RELRO, FORTIFY, and generates proper DWARF debuginfo. No batching
mechanism exists for multiple binaries.

### D3: Single `GO_LDFLAGS` for version injection

Only `internal/version.version` is injected (unlike complyctl which
also injects `gitTreeState`, `commit`, and `buildDate`). The
providers' `internal/version` package only exposes `version`.

```
export GO_LDFLAGS="-X %{goipath}/internal/version.version=%{version}"
```

### D4: Fedora 43 Go compatibility via conditional sed

Matches the `complyctl` spec approach: conditionally lower the
`go` directive in `go.mod` and `vendor/modules.txt` from 1.26 to
1.25 when building on Fedora 43.

**Why**: Fedora 43 is supported until 2026-12-09. Using a
conditional `%if 0%{?fedora} == 43` block keeps it self-documenting
and easy to remove after EOL.

**Alternative considered**: Carrying a patch file. Rejected because
the `sed` approach is identical to the complyctl reference and
avoids maintaining a separate patch file that would need updating
on every Go version bump.

### D5: No `%gocheck2` exclusions

All 29 test files were analyzed. Every test that touches external
tools (oscap, conftest, snappy, ampel, git) uses mock/fake
implementations. The only integration test
(`cmd/openscap-provider/config/integration_test.go`) is gated by
`//go:build integration` and is automatically excluded from
`go test ./...`. Toolcheck tests use universally available commands
(`go`, `ls`).

### D6: `go-vendor-tools.toml` with yaml.v3 license override

The `askalono` detector correctly identifies all vendored licenses
except `gopkg.in/yaml.v3/LICENSE` (dual MIT + Apache-2.0 in a
single file). A manual `[[licensing.licenses]]` override is
required, matching the same override in `complyctl`'s config.

## Risks / Trade-offs

**[Risk] `%gobuild` external linker requirement** The `%gobuild`
macro uses `-linkmode=external` which requires a C compiler and
linker. This is standard in Fedora build environments but differs
from the current `CGO_ENABLED=0` approach in `.goreleaser.yaml`.
RPM builds and release builds are intentionally different
environments.
Mitigation: Fedora build roots always have `gcc` available.

**[Risk] Fedora 43 Go compat workaround fragility** The `sed`
command pattern-matches `go [0-9].*` in `go.mod`. If the format
changes in future Go versions, it could silently fail.
Mitigation: The block is conditional on `%{?fedora} == 43` only
and has a documented EOL date for removal. A `# TODO(2027-01):
remove F43 workaround` comment should be added in the spec itself
as a sunset trigger.

**[Risk] License expression drift** If vendored dependencies
change, the `License:` field and `go-vendor-tools.toml` overrides
must be updated.
Mitigation: `%go_vendor_license_check` in `%check` will fail the
build if the license config is stale. The `go_vendor_license report
--verify-spec` workflow catches drift. When vendored dependencies
change, maintainers run `go_vendor_license --config
go-vendor-tools.toml report expression` and update the spec's
`License:` field accordingly.

**[Risk] `go-vendor-tools` availability** This change makes RPM
builds dependent on the Fedora-packaged `go-vendor-tools`. This
tool is available in Fedora 43+ but is not available in CentOS
Stream or RHEL build environments.
Mitigation: The RPM spec targets Fedora submission specifically.
Non-Fedora builds (e.g., GoReleaser releases) use a separate
build pipeline that does not depend on `go-vendor-tools`.

**[Note] Provider binary permissions** The `%attr(0755, root, root)`
permission on provider binaries in `%{_libexecdir}` follows the
standard Fedora convention for executable helper programs. While
providers are invoked by `complyctl` via gRPC subprocess (not
directly by users), 0755 is the expected permission for
`%{_libexecdir}` binaries per Fedora packaging guidelines and
enables direct invocation for debugging purposes.
