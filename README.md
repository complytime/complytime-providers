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

## OPA input safety

OPA directory scans exclude the host workspace base joined with `.complytime` and, when different,
the provider's artifact directory. This changes enclosing-directory results, including for user-owned
workspace files. Explicit local workspace targets fail; copy intended input outside both directories
rather than using a hard link. Empty or fully ignored local directories and remote clones or scan
paths with no confirmed evaluated input produce errors, not passing assessments.
For a remote target selecting multiple branches, one input-safety or empty-input
rejection suppresses assessments for that entire target; other targets retain their results.

Install conftest on `PATH`. Compatibility is tested by behavior, not an exact runtime version check.
For workspace boundaries, link handling, conftest configuration remedies, and traversal
limitations, see the [OPA provider man page](docs/man/complyctl-provider-opa.md#selecting-scan-input).

## Preserve and verify scan evidence

Use the Gemara Evaluation Log to locate and verify evidence from an earlier scan.
Providers always preserve each evaluation's outputs in a fresh directory; later
scans do not overwrite those files, even when the output bytes are identical.
The log alone defines the evidence set through its evidence entries and linked
mapping references. Nearby supporting files do not become evidence implicitly.

| Provider | Layout beneath the selected workspace | Reported evidence |
| --- | --- | --- |
| Ampel | `.complytime/ampel/results/<repository>/<branch>/<spec>/evaluation-<time>-<token>/` | `snappy.intoto.json` and `ampel.intoto.json` |
| OpenSCAP | `.complytime/openscap/results/<target>/<profile>/evaluation-<time>-<token>/` | `arf.xml`; `results.xml` is supporting output |
| OPA (remote) | `.complytime/opa/results/<repository>/<branch>/evaluation-<time>-<token>/` | `conftest.json` |
| OPA (local) | `.complytime/opa/results/<target>/evaluation-<time>-<token>/` | `conftest.json` |

Readable context labels are bounded grouping hints, not unique identities. The UTC
time uses `YYYYMMDDTHHMMSSZ` and records collection start, as does `CollectedAt`;
a random token distinguishes evaluations. Evidence/reference IDs identify those
artifact instances rather than future scans of the same target. Multiple
assessments can share an artifact without additional copies. Each directory may
also contain best-effort `metadata.json` for non-secret target/configuration context;
metadata is not the authority for evidence identity or digests.

### Locate the workspace and check the files

1. Establish the workspace used for the scan: complyctl selects `--workspace`, then
   `COMPLYTIME_WORKSPACE`, then its invoking directory. Record the selected root
   alongside the log; it is not the log's directory or the auditor's current directory.
   Complyctl delivers it through existing Generate global/Scan target `workspace`
   variables. Providers use CWD only when the variable is missing or empty, reject
   invalid/conflicting roots, and resolve workspace-root symlinks.
2. Read each log evidence entry's source coordinate, digest and reference ID. Resolve
   the coordinate against that workspace root. Follow the matching mapping reference
   for an escaped absolute `file://` URL identifying the same original file. Decode
   URI escapes (`%20` for a space, `%25` for a percent sign, and UTF-8 escapes for
   non-ASCII names); do not use the encoded URL as a literal filesystem path.
3. Read the file as an authorized account and compare its SHA256 with the log's
   `sha256:<hex>` digest. No collection, packaging or storage integration is needed
   for this in-place workflow. Use only log-selected evidence, not a directory listing.

**Breaking input lookup:** relative provider inputs/configuration now resolve against
the selected workspace, not invocation CWD. Migrate invocation-relative configurations
when those directories differ. For example, workspace `/srv/compliance` and
`input_path: manifests/` select `/srv/compliance/manifests/`, irrespective of where
complyctl was invoked. Absolute external inputs stay absolute. Tool execution and
OPA conftest discovery, including relative `CONFTEST_CONFIG_FILE`, use the workspace
without process-wide chdir. OPA retains the host/provider input-exclusion boundaries
when different, including an existing invocation-relative artifact directory.

OPA evidence is the retained raw conftest JSON result, not its input file or clone;
every target type has a result-file digest. Embedded conftest filenames are input
provenance, not retained evidence links. Remote clones remain replaceable under
`.complytime/opa/repos/`; local inputs stay in place. Inputs, policies and historical
clones are not archived. Retention does not guarantee historical re-evaluation, and
unchanged findings do not prove unchanged inputs.

### Access, capacity and transfer

New evaluation directories use `0700` and evidence/supporting files use `0600`, owned
by the provider's effective execution UID/GID. Normal user scans remain user-owned;
sudo/direct-root scans remain root-owned. The original non-root user needs authorized
elevated access to read and hash root-run evidence: there is no ownership handoff or
automatic access for other users. The execution account needs a writable workspace
and traversal access through every parent; new context parents permit that access
without changing unrelated existing directories.

For permission errors, check the execution account and parent traversal. For disk-space
errors, provide sufficient writable capacity before retrying. Failures or interrupted
evaluations can leave unreferenced partial outputs; their presence does not prove
publication succeeded. Metadata write/close failures warn without suppressing complete,
verified evidence. Ownership/mode verification failures fail the RPC without a response.
Do not widen private historical permissions or change earlier owners to bypass checks.

Storage grows with every evaluation, including identical outputs and failed attempts.
There is no automatic cleanup, retention setting, compression or deduplication. OPA's
input-safety preflight also traverses retained history, so inspection work grows with
history. Inaccessible private history can cause fail-closed input errors: use the
authorized execution account or authorized elevated access, rather than widening
permissions or deleting evidence as an automatic remedy.

Preserve the Evaluation Log **and** its log-selected evidence files. Uploading only
the log does not transfer artifact bytes. Downstream collection must keep both
retrievable, verify transferred bytes against the recorded digests, and map moved
files to their original references without modifying the log. Collection, validation
of collected copies and optional packaging are separate future complyctl capabilities,
not implemented here. Storage backends, transfers, access policy and longevity remain
customer-owned. Coordinates can resolve against a copied workspace with its layout
preserved; absolute URLs still identify the originating filesystem and do not relocate
automatically. Preserve historical logs separately too: provider artifact preservation
does not prevent the host from replacing a same-name log during rapid scans.

### Upgrade and rollback

**Breaking changes:** follow returned evidence coordinates and instance-qualified IDs,
not fixed filenames or moving aliases. Ampel/OPA normalized summaries are no longer
written; use the Evaluation Log for normalized assessments. Fixed-path outputs stop
being refreshed. Existing files and historical logs remain untouched and are not
migrated; old summaries are not current results, and previously overwritten evidence
cannot be recovered. Rolling back to older binaries resumes their overwrite behavior
for subsequent scans while retained evaluation directories remain readable by authorized
users. Preserve logs and referenced files before deliberately removing a workspace.

See the provider-specific [Ampel](cmd/ampel-provider/README.md),
[OpenSCAP](cmd/openscap-provider/README.md#retained-evidence-lifecycle) and
[OPA](cmd/opa-provider/README.md#locate-and-verify-retained-evidence) guidance and
their [man pages](docs/man/) for artifact details.

## Test

```bash
make test    # Run all unit tests
make lint    # Run golangci-lint
```

For cross-repository checks with a pinned canonical harness and prebuilt released
complyctl binaries, see [released-host integration](docs/dev-testing-environment.md#released-host-integration).

## Project Structure

```text
cmd/
├── openscap-provider/   # XCCDF-based system scanning
├── ampel-provider/      # In-toto attestation verification
└── opa-provider/        # OPA/conftest policy evaluation
internal/
├── archive/             # Shared tar.gz extraction with security constraints
├── complytime/testdata/ # Shared XML test fixtures
├── evidence/            # Workspace resolution, retained artifacts and evidence mappings
├── genindex/            # Generated-artifact index validation and resolution
└── version/             # Build-time version injection
docs/                    # Documentation
```

Each provider is self-contained under `cmd/<name>-provider/` with its own subpackage hierarchy. Shared utilities live in `internal/`: `internal/archive/` provides secure archive extraction for complypack content, `internal/evidence/` provides workspace resolution, constrained evaluation storage, SHA256 digests and instance-qualified evidence mappings, `internal/genindex/` resolves generated artifacts, and `internal/version/` provides build-time version injection.

## Documentation

- [Provider Development Guide](docs/provider-guide.md) --
  developing providers.
- [Dev Testing Environment](docs/dev-testing-environment.md) --
  devcontainer setup for interactive testing during PR review.

## License

Apache-2.0. See [LICENSE](LICENSE) for details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. All changes require a feature branch and PR with review from at least two maintainers.
