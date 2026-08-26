## 1. Vendored License Tooling

- [ ] 1.1 Create `go-vendor-tools.toml` with `[archive]` section,
  `[licensing]` section (`detector = "askalono"`), and manual
  `[[licensing.licenses]]` override for `vendor/gopkg.in/yaml.v3/LICENSE`
  (expression: `MIT AND (MIT AND Apache-2.0)`). Verify by running
  `go_vendor_license --config go-vendor-tools.toml report expression`
  and confirming the output is
  `Apache-2.0 AND BSD-3-Clause AND ISC AND MIT AND MPL-2.0`.

## 2. Spec Header Modernization

- [ ] 2.1 Add `%bcond check 1` at the top of the spec. Reorder globals:
  move `%global app_dir complytime` before the goipath block, remove
  `%global base_url`, remove `%global debug_package %{nil}`. Move
  `Version:` before `%gometa -f`. Verify the header follows the pattern
  in the complyctl reference spec.

- [ ] 2.2 Update `Release:` from `1%{?dist}` to `%autorelease`. Update
  `License:` to `Apache-2.0 AND BSD-3-Clause AND ISC AND MIT AND MPL-2.0`
  (must match the `go_vendor_license report expression` output). Update
  `URL:` from `%{base_url}` to `%{gourl}`. Replace `Source0:` with
  `%{gosource}` and add `Source1: %{archivename}-vendor.tar.bz2` and
  `Source2: go-vendor-tools.toml`. Replace `BuildRequires: golang >= 1.26`
  and `BuildRequires: go-rpm-macros` with `BuildRequires: go-vendor-tools`.
  Verify by inspecting the spec header matches the Fedora Go packaging
  pattern.

## 3. Main Package and Sub-package Updates

- [ ] 3.1 Add a main `%files` section for `complytime-providers` with
  `-f %{go_vendor_license_filelist}` and `%doc README.md`. This package
  holds shared license files. Verify the main `%description` already
  exists (it does) and is appropriate.

- [ ] 3.2 Update all three sub-packages (`openscap`, `ampel`, `opa`):
  bump `Requires: complyctl` from `>= 0.0.8` to `>= 1.0.0`, add
  `Requires: %{name} = %{version}-%{release}` to each. Remove
  `%license LICENSE` and `%doc README.md vendor/modules.txt` from each
  sub-package `%files` section (now in main package). Verify each
  sub-package `%files` section contains only the `%attr` line for its
  binary.

## 4. Build and Prep Sections

- [ ] 4.1 Replace `%prep` section: change `%goprep -k` to `%goprep -A`,
  add `%setup -q -T -D -a1 %{forgesetupargs}` and `%autopatch -p1`.
  Add the Fedora 43 Go 1.25 compatibility block (conditional `sed` on
  `go.mod` and `vendor/modules.txt`). Add `%generate_buildrequires`
  section with `%go_vendor_license_buildrequires -c %{S:2}`. Verify by
  comparing with the complyctl reference spec `%prep` section.

- [ ] 4.2 Replace `%build` section: remove `%set_build_flags`,
  `export GO111MODULE=on`, `GO_LD_EXTRAFLAGS`, `GO_BUILD_BINDIR`,
  and raw `go build` commands. Add `%global gomodulesmode GO111MODULE=on`,
  `export GO_LDFLAGS` with version injection, and three `%gobuild` calls
  targeting `%{gobuilddir}/bin/` output paths. Verify the three
  `%gobuild` lines reference the correct import paths
  (`%{goipath}/cmd/openscap-provider`, `%{goipath}/cmd/ampel-provider`,
  `%{goipath}/cmd/opa-provider`).

## 5. Install, Check, and Files

- [ ] 5.1 Update `%install` section: add `%go_vendor_license_install -c %{S:2}`
  at the top. Update binary install source paths from `bin/` to
  `%{gobuilddir}/bin/`. Verify install paths for all three binaries
  target `%{buildroot}%{_libexecdir}/%{app_dir}/providers/`.

- [ ] 5.2 Replace `%check` section: add
  `%go_vendor_license_check -c %{S:2}`, wrap test execution in
  `%if %{with check}` / `%endif`, replace `go test -mod=vendor -v ./...`
  with `%gocheck2`. Verify the check section structure matches the
  complyctl reference.

## 6. Changelog and Verification

- [ ] 6.1 Update `%changelog`: add a new entry at the top documenting
  all modernization changes. Verify the date format and entry style
  match existing changelog entries.

- [ ] 6.2 Run `go_vendor_license --config go-vendor-tools.toml report
  --verify-spec` against the updated spec to confirm the license
  expression is consistent. Verify no errors.

- [ ] 6.3 Run `rpmbuild -bs complytime-providers.spec` (or equivalent
  source RPM build) to verify the spec parses correctly. If the build
  environment supports it, run `rpmbuild -bb` or a mock build to verify
  end-to-end. Document any issues found.
