# SPDX-License-Identifier: Apache-2.0

%global goipath github.com/complytime/complytime-providers
%global base_url https://%{goipath}
%global app_dir complytime
%global debug_package %{nil}

Name:           complytime-providers
Version:        0.0.1
Release:        1%{?dist}
Summary:        Compliance scanning providers for complyctl
License:        Apache-2.0
URL:            %{base_url}
Source0:        %{base_url}/archive/refs/tags/v%{version}.tar.gz

BuildRequires:  golang >= 1.25.0
BuildRequires:  go-rpm-macros
ExclusiveArch:  %{go_arches}

%gometa -f

%description
Compliance scanning providers that extend complyctl with support for
different policy validation platforms (PVPs). Each provider communicates
with complyctl via gRPC and follows the complyctl-provider-* discovery
convention. Providers are distributed as separate sub-packages so users
can install only the providers they need.

# --- OpenSCAP provider sub-package ---

%package        openscap
Summary:        OpenSCAP scanning provider for complyctl
Requires:       complyctl >= 0.0.8
Requires:       openscap-scanner
Requires:       scap-security-guide

%description    openscap
OpenSCAP scanning provider that extends complyctl with OpenSCAP evaluation
capabilities. It converts OSCAL assessment plans into SCAP policies,
executes scans via the OpenSCAP engine, and returns structured results
to complyctl. Communicates via gRPC (Describe, Generate, Scan RPCs)
and follows the complyctl-provider-* discovery convention.

# --- Ampel provider sub-package ---

%package        ampel
Summary:        Ampel scanning provider for complyctl
Requires:       complyctl >= 0.0.8

%description    ampel
Ampel scanning provider that extends complyctl with Ampel evaluation
capabilities. It communicates via gRPC and follows the
complyctl-provider-* discovery convention.

NOTE: Requires the 'snappy' and 'ampel' CLI tools at runtime. These are
not currently packaged in Fedora and must be installed separately.

%prep
%goprep -k

%build
# Set up environment variables and flags to build properly and securely
%set_build_flags
export GO111MODULE=on

# Define and create the output directory for binaries
GO_BUILD_BINDIR=./bin
mkdir -p ${GO_BUILD_BINDIR}

# Build both provider binaries
go build -mod=vendor -buildmode=pie -ldflags "${LDFLAGS}" -o ${GO_BUILD_BINDIR}/complyctl-provider-openscap ./cmd/openscap-provider
go build -mod=vendor -buildmode=pie -ldflags "${LDFLAGS}" -o ${GO_BUILD_BINDIR}/complyctl-provider-ampel ./cmd/ampel-provider

%install
install -d -m 0755 %{buildroot}%{_libexecdir}/%{app_dir}/providers

install -p -m 0755 bin/complyctl-provider-openscap %{buildroot}%{_libexecdir}/%{app_dir}/providers/complyctl-provider-openscap
install -p -m 0755 bin/complyctl-provider-ampel %{buildroot}%{_libexecdir}/%{app_dir}/providers/complyctl-provider-ampel

%check
# Run unit tests
go test -mod=vendor -v ./...

# No main files section -- source RPM produces only sub-packages

%files          openscap
%attr(0755, root, root) %{_libexecdir}/%{app_dir}/providers/complyctl-provider-openscap
%license LICENSE
%doc README.md vendor/modules.txt

%files          ampel
%attr(0755, root, root) %{_libexecdir}/%{app_dir}/providers/complyctl-provider-ampel
%license LICENSE
%doc README.md vendor/modules.txt

%changelog
* Fri Apr 24 2026 Marcus Burghardt <maburgha@redhat.com> - 0.0.1-1
- Initial RPM packaging for complytime-providers
- OpenSCAP and Ampel provider sub-packages
