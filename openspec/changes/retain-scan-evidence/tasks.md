# Tasks

These tasks describe future implementation, not work completed by the specification
commit. Begin only after user review and after the spec artifacts are committed and
pushed. Groups 2 through 4 depend on group 1; each provider group carries its own
tests and documentation. Preserve the existing untracked main-spec files until their
ownership and synchronization are resolved before archive.

Scope boundary: implement preservation and explicit provider-file mapping into the
Gemara Evaluation Log, the source of truth for the evidence set. Keep unreferenced
supporting outputs distinct from reported evidence. Collection, validation of a
collected set, and optional packaging are a separate future complyctl capability,
not tasks or release dependencies here. That future feature must map moved files
without modifying the log; storage backends and long-term retention remain
customer-owned. Use existing complyctl workspace-variable delivery and evidence
conversion; no launcher, caller-identity, protocol, or release-pairing change is
required in complyctl. Integration tests verify the unchanged consumer.

Selected decisions: execution-account ownership, private 0700/0600 modes, existing
workspace variables with CWD fallback, workspace-relative input/configuration paths,
and absolute file URLs. Root-run evidence stays root-owned; original-user handoff
is deferred. Provider-only implementation retains the single-repository OpenSpec
tier. The permission-rule conflict remains reported; no governance rule is waived.

Test harness: use isolated Go tests alongside the shared helpers and each provider's
server, scan, and results packages, runnable by `make test`. Reuse the existing XML
and attestation fixtures and fake runners; OpenSCAP can use a temporary fake `oscap`
executable on a test-scoped PATH to write fixture bytes to the supplied ARF/XCCDF
paths, without requiring a system OpenSCAP installation. Do not run PATH-mutating
tests in parallel. Use deterministic injected failures at the new helper boundaries
for allocation, write, close, and digest errors, not chmod-based permission failures
that disappear when tests run as root. Preserve each provider's established RPC
versus `ScanResponse.Errors` channel and assert that exact channel alongside the
absence of assessments and references for the failed evaluation.

Use process-isolated provider tests with distinct invocation/workspace directories
and injected OS identities for root/non-root cases. These tests run under `make test`
without a complyctl checkout, credentials, privileged execution, or parent-process chdir.
Use injected identity/filesystem operations for ownership failures and inspect real
UID/GID results separately in controlled root-run integration tests. Preserve
Ampel's existing RPC error for digest failure; do not misclassify it as a response
error while adding the new finalization errors from D6/D9.

## 0. Implementation-readiness prerequisites

- [x] 0.1 Before implementing private modes, obtain maintainer reconciliation of
  `.opencode/uf/packs/go.md` SC-005 with the selected 0700/0600 evidence modes.
  Record the authorized decision, scope, venue/link, and applicable instruction in
  the design without modifying convention packs in this change. Keep this task
  incomplete until the decision is recorded; no companion complyctl contract is
  required to satisfy it.

## 1. Shared evaluation storage and context

- [ ] 1.1 Implement evaluation allocation and centralized naming/0700/0600-mode
  constants in focused `internal/evidence` files. Provider configuration derives
  absolute workspace/results roots using `provider.WorkspaceDir` and its existing
  constants; OS identity supplies the execution account for ownership verification.
  The allocator accepts these explicit values, validates confinement, and returns the
  evaluation directory, token, collection time, and constrained artifact access.
  It must not import `cmd/*`, rediscover CWD, select workspace precedence, or
  interpret identity transport. Share root-validation and ownership/mode checks
  across provider adapters; test each adapter's wiring and common validation.
  Test all three adapters without mutating global result paths or duplicating provider
  directory constants. Centralize the bounded retry limit with a test override.
  Verify exact timestamp/path form,
  token entropy source, fixed-clock repeated allocations, concurrent allocations,
  forced collisions, clock rollback, allocation failure, and retry exhaustion.
  Assert atomic no-replace behavior deterministically, not only under `-race`.
- [ ] 1.2 Implement bounded readable labels and root-constrained context-directory creation separately from `SanitizeForID`; verify D1's character set, 0/1/79/80/81-byte inputs and non-empty fallback, slash-containing branches, long paths, lossy collisions, URL credential exclusion, and a symlinked context parent pointing outside the root. Cover `.`, `..`, absolute paths, `a/../../b`, NUL/control characters, and empty-after-normalization labels, asserting safe components or rejection without writes outside the results root.
- [ ] 1.3 Implement best-effort versioned metadata persistence with an explicit allowlist and readable target correlation keys; verify the D5 envelope and allowed provider fields, omission of digests/environment/target-variable maps, workspace-relative internal paths and absolute external provenance paths per D8, context surviving label truncation, documented key normalization, credential exclusion, and write/close failures warning without blocking evidence.
- [ ] 1.4 Add shared artifact-instance identity/reference construction and timestamp
  propagation support. Pass only validated, already-absolute artifact paths to
  `FileURL`; do not rely on its relative-input/failure fallback. Cover spaces,
  percent signs, and non-ASCII names. Verify repeated references agree, distinct
  evaluations do not collapse, and URLs identify the same files as coordinates
  resolved against the supplied workspace root. Assert `Source.ReferenceID` matches
  its mapping ID without requiring `Evidence.ID` equality.
- [ ] 1.5 Document helper guarantees and limitations in GoDoc and assess AGENTS impact; verify API comments cover allocation errors, returned paths, collection-start semantics, and filesystem side effects.
- [ ] 1.6 Implement root-constrained artifact access through allocation, write,
  parse, hash, and finalization. Test symlinked context parents, artifact symlink
  substitution, non-regular outputs, workspace-root symlink resolution, and
  validated/hashed file agreement without external reads or ownership changes.
- [ ] 1.7 Implement execution-account ownership/mode verification using effective
  OS UID/GID without original-user handoff. Test normal and root execution,
  unexpected ownership, identity-lookup and mode-verification failures, and
  inaccessible existing parents. Ignore SUDO_UID/SUDO_GID/USER and RPC owner claims
  when selecting ownership; their presence must not reject an ordinary sudo scan.
  Include supporting files present after metadata failure. Verify
  tools stop writing before final checks, no ownership transfer occurs, and
  failures return RPC errors without a response.
  Assert directory mode 0700 and file mode 0600 after the reported governance
  conflict has been reconciled; the intended modes are selected, not an open choice.
  Verify newly created context parents allow the selected owner to traverse the full
  path without widening the leaf or changing unrelated existing directories.
- [ ] 1.8 Resolve Generate global/Scan target `workspace` variables per D8 with
  CWD fallback only for absent/empty values. Test relative and symlinked workspace
  roots, invalid non-empty roots, conflicting Scan target roots and conflicting
  Generate target/global roots before storage access. Preserve OPA host/provider
  exclusion boundaries. With invocation A and workspace B, prove Generate/Scan
  paths, coordinates, URLs, and relative inputs/configuration resolve beneath B;
  absolute external inputs remain absolute. Place conflicting fixtures in A and B,
  verify conftest discovery and relative CONFTEST_CONFIG_FILE use B with matching
  ignore analysis/execution, and assert no process-wide chdir or writes beneath A.
  Do not reapply complyctl's flag/environment precedence in providers.

## 2. Ampel retention

- [ ] 2.1 Allocate a readable repo/branch/spec evaluation directory per execution and pass its paths into `ScanRepository`; verify three repeated scans, two specs, and multiple branches in one request retain independent attestation pairs with distinct tokens, coordinates, and IDs, including slash-containing branches.
- [ ] 2.2 Finalize timestamps, digests, paths, and instance-qualified references before publishing assessments; attempt metadata persistence and stop writing legacy summaries. Verify each source/reference pair resolves to its stored attestation and multiple assessments share artifacts.
- [ ] 2.3 Add failure regressions for missing/invalid tool output and write/close/digest failures; verify no stale or incomplete evidence is published, metadata failures warn without suppressing complete evidence, and valid failing-compliance results remain retained.
- [ ] 2.4 Update Ampel man-page FILES/EVIDENCE sections with layout, changed
  coordinate/ID forms marked as breaking, spec-versus-policy context, and growth
  behavior. In the man page and provider README, replace legacy summary entries
  with the note that existing summaries remain untouched but are no longer written.
  Regenerate its man page and verify examples match integration-test output.

## 3. OpenSCAP retention

- [ ] 3.1 Replace Scan-time global result-path use with explicitly threaded evaluation ARF/XCCDF paths through command construction, parsing, digesting, and response construction; verify commands and readers use the same allocated paths without mutating package globals.
- [ ] 3.2 Attempt to record request target, observed ARF target, profile, datastream, and tailoring context, and build instance-qualified ARF references; verify a three-scan regression preserves prior ARFs and digests, including valid noncompliant results and an evaluation crossing a timestamp boundary.
- [ ] 3.3 Add operational-error tests for allocation, missing/invalid required outputs, and digest failures; verify no prior fixed-path ARF is reused, incomplete evaluations publish no evidence, and metadata failures warn without suppressing complete evidence.
- [ ] 3.4 Update OpenSCAP man-page FILES/EVIDENCE sections with target/profile
  grouping, retained ARF semantics, and changed coordinate/ID forms marked as
  breaking. Document execution-account ownership, approved modes and parent
  traversal, and authorized elevated reading/hashing after root-run scans.
  Regenerate its man page and verify Generate-time paths remain accurately documented.

## 4. OPA result-file evidence

- [ ] 4.1 Separate conftest stdout from stderr at the runner boundary and retain exact JSON output as `conftest.json`; verify valid stdout with diagnostic stderr, malformed output, tool errors, and `--no-fail` compliance outcomes.
  Preserve `RunWithEnv`'s captured environment and composed ignore rule. Classify
  empty-directory diagnostics from stderr without putting them in retained JSON;
  keep unrelated execution failures distinct and preserve structured evaluated-input
  validation for `[]`, `null`, missing filename/namespace, and valid no-finding output.
- [ ] 4.2 Allocate readable evaluation directories for remote target/branch and local targets, then parse and hash the retained output; verify remote, local-file, and local-directory evidence coordinates all identify `conftest.json` and carry its SHA256 digest.
- [ ] 4.3 Keep working clones at the existing replaceable location and local inputs in place; verify three-scan tests create no historical clone/input snapshots and earlier result evidence survives clone replacement and local input edits/deletion.
  Preserve PR #233's `opa-input-selection` contracts with the new layout: enclosing
  scans exclude current/prior evaluation outputs, metadata, partial files, and other
  providers' artifacts; explicit workspace inputs and symlink/hard-link aliases into
  retained evidence are rejected. Ordinary selected clones remain eligible; aliases
  into retained results outside the clone do not. Preserve cyclic-clone isolation
  across independent targets and repeated scans. Measure representative accumulated
  histories using the existing workspace-preflight benchmarks, and cover inaccessible
  private history without skipping identity checks or changing earlier permissions.
- [ ] 4.4 Replace input-path evidence references with result-file references, attempt supporting metadata persistence after context is finalized, and stop writing legacy summaries; verify metadata includes non-secret source identity, branch/scan path, available policy/configuration context, and consistent collection timestamps.
- [ ] 4.5 Cover repeated targets, multiple branches in one request, different scan paths on the same repo/branch, colliding display labels, and multiple assessments per evaluation; verify separate evaluation tokens, coordinates, IDs, and matching reference URLs without copying artifacts per assessment or modifying the #199 encoding helpers.
- [ ] 4.6 Cover output write/close, parse, digest, and metadata failures; verify incomplete evidence produces operational errors, metadata failures warn without suppressing complete evidence, and previously retained results stay unchanged.
  Extend `workspace_branches_test.go` for both branch orders: input-safety rejection,
  preflight cancellation, empty/unconfirmed directory input, and invalid directory
  output withhold all assessments/evidence/mapping references from that configured
  target. Preserve its operational errors and other targets' valid retained evidence.
  Already written branch artifacts remain unreferenced on disk; ordinary branch-scoped
  clone/tool/storage failures do not acquire target-wide rejection semantics.
- [ ] 4.7 Update OPA man-page FILES/EVIDENCE sections and README evidence-lifecycle guidance; replace the legacy summary path entry with a note that existing summaries remain untouched but are no longer written. Regenerate the man page and verify documentation distinguishes stored result evidence from replaceable inputs, explains storage growth, and marks coordinate/ID changes.

## 5. Integration verification and release readiness

- [ ] 5.1 Add three-scan regressions in each provider's `server/server_test.go`, using the test harness above with changing and identical fixture outputs. Seed legacy fixed-path artifacts and summaries with known bytes; resolve every `ScanResponse` coordinate against an explicit workspace root, verify its escaped absolute file URL identifies the same artifact, recompute each `sha256:` digest, and assert evaluation-qualified IDs and matching reference links. Verify earlier artifacts and legacy files remain unchanged without packaging or storage integration, and that supporting XCCDF/metadata files do not become implicit evidence. Copy the workspace tree and verify coordinates/digests against the copied root while absolute URLs remain unchanged and origin-specific; keep these contract tests runnable under `make test` without credentials or a complyctl checkout.
- [ ] 5.2 Verify the existing complyctl conversion preserves coordinates, digests, timestamps, absolute file URLs, and instance references in serialized Gemara Evaluation Logs. Validate converted logs against the applicable Gemara schema and exercise a downstream consumer; verify repeated evaluations do not lose references through deduplication and every reported evidence entry leads to its original file with the matching digest after later scans, without inferring extra evidence from directory contents.
- [ ] 5.3 Run `openspec validate retain-scan-evidence --strict`, `make build`, `make test`, `go test -race -count=1 ./...`, `make lint`, `make man`, and `git diff --check`; the race run includes shared concurrent-allocation tests and is an additional local convention-pack check, not currently provided by `ci_local.yml`. Inspect applicable workflows and pinned reusable definitions for additional local CI parity checks, recording external prerequisites rather than claiming unrun checks passed.
  Also run PR #233's installed-conftest isolation and offline CLI regressions with
  rebuilt binaries, using the commands and prerequisites in `AGENTS.md`. Record
  actual tested versions; do not introduce exact-version gates, metadata prerequisites,
  or version-based skips. A missing-conftest skip is not behavior verification, and
  local CI does not install conftest. Keep the existing behavior assertions while
  replacing input-path evidence assertions with retained-result coordinates/digests.
- [ ] 5.4 Verify integration with unchanged complyctl and record the tested
  revision/version. Confirm existing workspace-variable delivery for Generate and
  Scan under default, environment, and flag selection, including flag precedence
  and invocation A with workspace B. Verify generated artifacts/evidence beneath B,
  matching coordinates/URLs/digests, and no new provider artifacts beneath A without
  changing launcher CWD. Exercise workspace-relative local input, conftest discovery,
  explicit relative configuration, ignore precedence and fail-closed ambiguity.
  Preserve host/provider exclusion boundaries when different. Run
  `make test-cross-repo COMPLYCTL_DIR=<built-complyctl-checkout>` with required
  tools/credentials; record blockers honestly. In a controlled root-run test, verify
  root execution ownership/private modes and authorized elevated read/hash; normal
  user scans remain user-owned. No original-user handoff or companion release is
  required, and no skipped privileged test counts as verification.
- [ ] 5.5 Update root `README.md`, `cmd/*-provider/README.md`, and all provider man pages with the selected workspace root as the coordinate base and absolute `file://` URLs as originating-filesystem locators. Document workspace precedence, how to locate the root, URI escaping, and the in-place workflow of reading the log's evidence mappings, resolving files, and checking digests without packaging. Explain that the Gemara Evaluation Log alone defines the evidence set, supporting files are not implicitly evidence, preservation is always on without automatic cleanup or retention settings, and future optional collection/packaging must map moved files without rewriting the log. State that storage backends and longevity remain customer-owned and absolute URLs do not automatically relocate; verify examples and regenerate affected man pages.

  Explicitly distinguish preserving or uploading the Evaluation Log from preserving
  and transferring its referenced files: a log-only upload does not transfer the
  artifact bytes. Document that downstream collection preserves both the log and
  its log-selected evidence files, keeps those files retrievable, verifies
  transferred bytes against the recorded digests, and maps transferred artifacts to
  the original references without modifying the log. Keep collection implementation
  and storage integrations outside this provider-local change.

  Also document execution-account ownership, root-run/sudo behavior, approved modes,
  directory traversal and workspace writability requirements, and failures that
  leave unreferenced partial outputs. Include troubleshooting for permission and
  disk-space failures, and metadata's best-effort status. Do not promise access to
  the original non-root user after sudo, arbitrary other users, or historical evidence
  that was already overwritten. Explain workspace-relative inputs/configuration,
  CWD fallback for missing workspace variables, and unchanged absolute input paths.
  Explain that OPA's workspace identity preflight traverses retained history, so
  both storage and preflight work grow. Inaccessible private historical directories
  can cause actionable fail-closed input errors; do not suggest widening permissions
  or deleting evidence as an automatic remedy.
- [ ] 5.6 Document upgrade and rollback behavior in README and provider man pages: fixed-path outputs stop being refreshed, existing files and historical logs remain untouched and are not migrated, previously overwritten evidence cannot be recovered, and older binaries resume overwrite behavior while retained directories remain readable. Verify this guidance against the design's Migration Plan and regenerate affected man pages.
- [ ] 5.7 Add CHANGELOG entries under `Breaking Changes` for OPA result-file evidence, new output paths and instance-qualified IDs without fixed-path aliases, workspace-root-relative coordinates, and discontinued Ampel/OPA normalized summaries; record retained evidence under `Added` and updated guidance under `Documentation`. State that absolute `file://` URL representation is retained and legacy files and logs remain untouched, complete the AGENTS structural assessment from 1.5, confirm README and all man pages match the implemented layout, and file the required `unbound-force/website` issue before implementation merge; verify entries and the issue describe shipped behavior and local preservation/mapping, not an implemented collection, packaging, or managed retention feature.

  Supersede conflicting Unreleased claims about OPA input-path evidence and old ID
  forms so the release does not announce incompatible contracts as current at once.
  Cross-reference both capability specs and include workspace-relative input/config
  lookup as a breaking provider behavior; invocation-relative configurations must
  migrate. Record the tested unchanged complyctl revision without inventing a new
  API/version requirement. Do not mark unrun schema, privileged, or integration tests passed.
- [ ] 5.8 Run the required review council before implementation PR submission and resolve all REQUEST CHANGES; verify approval remains valid with no intervening code changes, leaving cleanup and the broader #199 encoding work deferred.

When replacing old contracts in groups 2–4, rewrite existing fixed-path ARF and
input-path OPA assertions to check the new retained artifact contract. Preserve
absolute `file://` URL coverage and update expected paths to the allocated files.
Remove unused `WritePerRepoResult`/`WritePerTargetResult`
summary writers and replace their failure-propagation tests with required-artifact
write/close failure coverage rather than deleting the error-path protection.

## Workflow follow-up

- Archive this change after implementation, verification, and review. Reconcile the
  existing untracked synchronized specs before updating canonical specs at archive.
- Keep issue #199 separate; link the artifact/logical-identity separation as context
  without claiming the general encoding assessment is complete.
- Address cleanup only after this feature merges, using a separately reviewed change.
