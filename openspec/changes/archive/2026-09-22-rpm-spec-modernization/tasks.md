**Prerequisites**: These tasks require a Fedora build environment
with `go-vendor-tools` and `rpm-build` installed (available in
the project devcontainer). The `complyctl` RPM spec modernization
(`opsx/rpm-spec-modernization` in the complyctl repo) MUST be
merged and `complyctl >= 1.0.0` MUST be available in the target
build repository before submitting `complytime-providers` for
Fedora review.

## 1. Vendored License Tooling

- [x] 1.1 First, run `go_vendor_license report all` without any
  config to identify which licenses `askalono` auto-detects and
  which it cannot. Confirm that `vendor/gopkg.in/yaml.v3/LICENSE`
  is the only undetectable license. Then create
  `go-vendor-tools.toml` with `[archive]` section, `[licensing]`
  section (`detector = "askalono"`), and manual
  `[[licensing.licenses]]` override for
  `vendor/gopkg.in/yaml.v3/LICENSE` (expression:
  `MIT AND (MIT AND Apache-2.0)` — copied verbatim from
  the complyctl reference `go-vendor-tools.toml`; the redundant
  `MIT` is intentional to match the upstream override format).
  Verify by running `go_vendor_license --config
  go-vendor-tools.toml report expression` and confirming the
  output is
  `Apache-2.0 AND BSD-3-Clause AND ISC AND MIT AND MPL-2.0`.

## 2. Spec Header Modernization

- [x] 2.1 Add `%bcond check 1` at the top of the spec. Reorder globals:
  move `%global app_dir complytime` before the goipath block, remove
  `%global base_url`, remove `%global debug_package %{nil}`. Move
  `Version:` before `%gometa -f`. Verify the header follows the pattern
  in the complyctl reference spec.

- [x] 2.2 Update `Release:` from `1%{?dist}` to `%autorelease`. Update
  `License:` to `Apache-2.0 AND BSD-3-Clause AND ISC AND MIT AND MPL-2.0`
  (must match the `go_vendor_license report expression` output). Update
  `URL:` from `%{base_url}` to `%{gourl}`. Replace `Source0:` with
  `%{gosource}` and add `Source1: %{archivename}-vendor.tar.bz2` and
  `Source2: go-vendor-tools.toml`. Replace `BuildRequires: golang >= 1.26`
  and `BuildRequires: go-rpm-macros` with `BuildRequires: go-vendor-tools`.
  Verify by inspecting the spec header matches the Fedora Go packaging
  pattern.

## 3. Main Package and Sub-package Updates

- [x] 3.1 Add a main `%files` section for `complytime-providers` with
  `-f %{go_vendor_license_filelist}` and `%doc README.md`. This package
  holds shared license files. Verify the main `%description` already
  exists (it does) and is appropriate.

- [x] 3.2 Update all three sub-packages (`openscap`, `ampel`, `opa`):
  bump `Requires: complyctl` from `>= 0.0.8` to `>= 1.0.0`
  (confirm `go.mod` requires `github.com/complytime/complyctl v1.0.0`
  to align the Go module dependency with the RPM Requires), add
  `Requires: %{name} = %{version}-%{release}` to each. Remove
  `%license LICENSE` and `%doc README.md vendor/modules.txt` from each
  sub-package `%files` section (now in main package). Verify each
  sub-package `%files` section contains only the `%attr` line for its
  binary. Verify that NO `%files` section includes `%dir` for
  `/usr/libexec/complytime/` or `/usr/libexec/complytime/providers/`
  (these directories are owned by the `complyctl` RPM).

## 4. Build and Prep Sections

- [x] 4.1 Replace `%prep` section: change `%goprep -k` to `%goprep -A`,
  add `%setup -q -T -D -a1 %{forgesetupargs}` and `%autopatch -p1`.
  Add the Fedora 43 Go 1.25 compatibility block (conditional `sed` on
  `go.mod` and `vendor/modules.txt`) with a
  `# TODO(2027-01): remove F43 workaround` comment. Add
  `%generate_buildrequires` section with
  `%go_vendor_license_buildrequires -c %{S:2}`. Verify the sed
  pattern works by running
  `sed -n 's/^go [0-9].*/go 1.25/p' go.mod` and confirming it
  outputs `go 1.25` (proving the pattern matches `go 1.26.5`).

- [x] 4.2 Replace `%build` section: remove `%set_build_flags`,
  `export GO111MODULE=on`, `GO_LD_EXTRAFLAGS`, `GO_BUILD_BINDIR`,
  and raw `go build` commands. Add `%global gomodulesmode GO111MODULE=on`,
  `export GO_LDFLAGS` with version injection, and three `%gobuild`
  calls (the `%gobuild` macro wraps `go build` with Fedora hardening
  flags, PIE, and debuginfo; it passes `-o` and import path arguments
  through to `go build`):
  ```
  %gobuild -o %{gobuilddir}/bin/complyctl-provider-openscap %{goipath}/cmd/openscap-provider
  %gobuild -o %{gobuilddir}/bin/complyctl-provider-ampel %{goipath}/cmd/ampel-provider
  %gobuild -o %{gobuilddir}/bin/complyctl-provider-opa %{goipath}/cmd/opa-provider
  ```
  Note: `%gocheck2` uses `go test` with vendor mode under the hood;
  key differences from the current raw `go test -mod=vendor -v ./...`
  include automatic build tag handling and integration with the
  `%bcond check` toggle. Verify the three `%gobuild` lines reference
  the correct import paths.

## 5. Install, Check, and Files

- [x] 5.1 Update `%install` section: add `%go_vendor_license_install -c %{S:2}`
  at the top. Update binary install source paths from `bin/` to
  `%{gobuilddir}/bin/`. Verify install paths for all three binaries
  target `%{buildroot}%{_libexecdir}/%{app_dir}/providers/`.

- [x] 5.2 Replace `%check` section: add
  `%go_vendor_license_check -c %{S:2}`, wrap test execution in
  `%if %{with check}` / `%endif`, replace `go test -mod=vendor -v ./...`
  with `%gocheck2`. Verify the check section structure matches the
  complyctl reference.

## 6. Changelog and Verification

- [x] 6.1 Update `%changelog`: add a new entry at the top documenting
  all modernization changes. Verify the date format and entry style
  match existing changelog entries.

- [x] 6.2 Run `go_vendor_license --config go-vendor-tools.toml report
  --verify-spec` against the updated spec to confirm the license
  expression is consistent. Verify no errors.

- [x] 6.3 Run `rpmbuild -bs complytime-providers.spec` to verify the
  spec parses correctly into a source RPM. This MUST pass.

- [x] 6.4 If the build environment supports it, run `rpmbuild -bb`
  or a mock build to verify end-to-end binary RPM generation. If
  attempted and it fails due to spec errors (not missing external
  deps), the task is blocked until resolved. Run `rpmlint` on the
  built SRPM and binary RPMs to catch common Fedora packaging
  errors (unowned directories, permission issues, license tag
  mismatches). Document any deliberately suppressed warnings.

- [x] 6.5 Update `CHANGELOG.md` under `## Unreleased` /
  `### Infrastructure` with an entry documenting the RPM spec
  modernization for Fedora Go packaging compliance. Verify the
  entry is consistent with existing changelog style.

- [x] 6.6 Add `go-vendor-tools.toml` to the project structure in
  `AGENTS.md` alongside `.goreleaser.yaml` (e.g.,
  `├── go-vendor-tools.toml  # Vendored license detection config
  (Fedora RPM)`). Verify the TMT test plan at
  `plans/test-RPM-providers.fmf` remains compatible with the
  updated spec (same binary names and install paths).

<!-- spec-review: passed -->
<!-- code-review: passed -->
