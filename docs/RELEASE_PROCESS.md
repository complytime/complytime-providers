# Release Process for complytime-providers

The release process values simplicity and automation in order to provide better predictability and low cost for maintainers.

## Process Description

Release artifacts are orchestrated by [GoReleaser](https://goreleaser.com/), which is configured in [.goreleaser.yaml](https://github.com/complytime/complytime-providers/blob/main/.goreleaser.yaml)

There is a [Workflow](https://github.com/complytime/complytime-providers/blob/main/.github/workflows/release.yml) created specifically for releases. This workflow is triggered manually by a project maintainer when a new release is ready to be published.

### Cutting a Release

Trigger the release workflow, providing the desired tag:

```bash
gh workflow run release.yml -f tag=v0.1.0
```

The workflow performs automated preflight validation before releasing:
- **Tag format**: must match `vMAJOR.MINOR.PATCH`
- **Semver ordering**: must be greater than the latest existing tag
- **CI verification**: confirms all required checks passed on `main`
- **Unreleased commits**: ensures there are changes to release
- **Tag creation**: creates an annotated tag automatically (no manual tagging needed)

#### Override Inputs

For exceptional situations (e.g., hotfix releases), the workflow supports skip overrides:

```bash
# Skip semver ordering check (e.g., backport release)
gh workflow run release.yml -f tag=v0.1.1 -f skip_semver_check=true

# Skip CI verification (e.g., CI infrastructure outage)
gh workflow run release.yml -f tag=v0.1.0 -f skip_ci_checks=true

# Skip unreleased commits check
gh workflow run release.yml -f tag=v0.1.0 -f skip_unreleased_check=true
```

If preflight passes, GoReleaser builds all three provider binaries, generates per-provider archives, signs checksums with [cosign](https://github.com/sigstore/cosign) (Sigstore keyless), and produces [SPDX](https://spdx.dev/) SBOMs via [syft](https://github.com/anchore/syft).

Once the workflow completes, the release is available on the [releases page](https://github.com/complytime/complytime-providers/releases)

### Release Artifacts

Each release produces the following artifacts:

| Artifact | Description |
|----------|-------------|
| `complyctl-provider-openscap_linux_x86_64.tar.gz` | OpenSCAP provider binary |
| `complyctl-provider-ampel_linux_x86_64.tar.gz` | Ampel provider binary |
| `complyctl-provider-opa_linux_x86_64.tar.gz` | OPA/Conftest provider binary |
| `checksums.txt` | SHA256 checksums for all archives |
| `checksums.txt.sigstore.json` | Cosign keyless signature bundle |
| `*.sbom.json` | SPDX JSON SBOMs (one per archive + source) |

### Re-running a Failed Release

If the release job fails after the preflight creates the tag (e.g., due to transient infrastructure issues), simply re-run the same command:

```bash
gh workflow run release.yml -f tag=v0.1.0
```

The preflight detects that the tag already exists at HEAD and safely skips the validation steps that would otherwise block the re-run.

## Tests

Tests relevant for releases are incorporated in CI tests for every PR.

## Cadence

Releases are discussed and agreed upon by project maintainers. The release cadence follows the project needs and may be aligned with complyctl releases when coordinated updates are required.

## Fedora Packages

After the repository split, complyctl and complytime-providers are independent Fedora packages with separate release cycles.

> **Note:** There are not yet packages for complytime-providers in Fedora. The Fedora package will be introduced after a few upstream releases stabilize the release process. The RPM spec and Packit configuration are already in place to support this when the time comes.

The [complytime-providers](https://github.com/complytime/complytime-providers) repository produces three sub-packages:
- `complytime-providers-openscap` -- OpenSCAP scanning provider
- `complytime-providers-ampel` -- Ampel scanning provider
- `complytime-providers-opa` -- OPA/Conftest scanning provider

### Automated Process (Packit)

Once the Fedora package is approved, the process will be automated by [Packit](https://packit.dev/docs/fedora-releases-guide) according to [.packit.yaml](https://github.com/complytime/complytime-providers/blob/main/.packit.yaml) configuration file and should only demand a PR review from a Fedora package maintainer.

Once a new GitHub Release is created, Packit will automatically:
1. Propose PRs to dist-git for the configured branches (rawhide, f44, f43)
2. After PR merge, trigger Koji builds
3. Submit Bodhi updates for released Fedora versions

### Preparation (only necessary for Manual Process)

To update a Fedora package, it is ultimately necessary to be a member of Fedora Packager group.
Here is the main documentation on how to become a Fedora Packager:
- [Joining the Package Maintainers](https://docs.fedoraproject.org/en-US/package-maintainers/Joining_the_Package_Maintainers/)

However, if you are not yet a Fedora Packager, it is still possible to propose a PR.
In this case, a package maintainer will review it and help on the process.

### Requirements

#### Install the required tools

```bash
sudo dnf install fedora-packager fedora-review
```
- Ensure your system user is included in the `mock` group. This is useful when testing the package changes.
```bash
sudo usermod -a -G mock $USER
```

#### Token for authenticated commands

Make sure you have a valid kerberos token. It will be necessary for commands that require authentication:
```bash
fkinit -u <your_fas_id>
```

#### Fork the repository

Create a fork from https://src.fedoraproject.org/rpms/complytime-providers

```bash
fedpkg clone --anonymous forks/<your fedora id>/rpms/complytime-providers
cd complytime-providers
```

### Update the spec file and sources

Usually it is only necessary to update the `Version:` line and include a `%changelog` entry.

`rpmdev-bumpspec` command can be used to automate this process. e.g.:
```bash
rpmdev-bumpspec -n 0.2.0 -c "Bump to upstream version v0.2.0" complytime-providers.spec
```

Ensure the sources are downloaded locally:
```bash
fedpkg sources
```

To ensure the `scratch build` doesn't fail due to an "Invalid Source", ensure the new sources are uploaded to the [lookaside_cache](https://docs.fedoraproject.org/en-US/package-maintainers/Package_Maintenance_Guide/#upload_new_source_files):
```bash
fedpkg new-sources
```

### Package Tests

Check if the changes work as expected before proceeding to the next step:
```bash
fedpkg diff
fedpkg lint
fedpkg mockbuild
```
> **_NOTE:_** Alternatively one can test the package build in Koji with `fedpkg scratch-build --srpm`.

### Propose the updates

After confirming that everything is fine, create a new branch to use in the Pull Request. e.g.:
```bash
git checkout -b release-0.2.0_rawhide
git status
git add -u
git commit -s
git push -u origin release-0.2.0_rawhide
```
Continue the steps via src.fedoraproject.org web UI.

Repeat this process for all other relevant branches.

```bash
fedpkg switch-branch f44
```

### Create the new Builds

Once the PRs are merged, it is time to create the new builds.

```bash
fedpkg switch-branch rawhide
fedpkg build
```
- Follow the builds status in the following links:
    - [Builds Status](https://koji.fedoraproject.org/koji/packageinfo?packageID=44960)

### Submit Fedora updates

After the build is done, an update must be submitted to [Bodhi](https://bodhi.fedoraproject.org).

Updates for `rawhide` builds are submitted automatically, but updates for any branched version needs to be submitted manually.
```bash
fedpkg update
```
Or via web interface on [Bodhi](https://bodhi.fedoraproject.org).

The new updates enter in `testing` state and are moved to stable after 7 days, or sooner if it receives 3 positive "karmas".
After moving to `stable` state, the update is signed and awaits to be pushed to the repositories by the Release Engineering Team.

Check the package update status in the following links:
  - [Updates Status](https://bodhi.fedoraproject.org/updates/?packages=complytime-providers)
  - [Package Overview](https://src.fedoraproject.org/rpms/complytime-providers)

#### Troubleshooting

If tests fail due to external issues, they can be restarted once the external issues are solved.
For example, if some tests in a Bodhi update failed due to infrastructure issues, they could be restarted by the following command:
```bash
bodhi updates trigger-tests <UPDATE_ID>
```

### More information
- [Fedora Package Guidelines](https://docs.fedoraproject.org/en-US/packaging-guidelines/)
- [Package Maintenance Guide](https://docs.fedoraproject.org/en-US/package-maintainers/Package_Maintenance_Guide)
- [Package Update Guide](https://docs.fedoraproject.org/en-US/package-maintainers/Package_Update_Guide/)
