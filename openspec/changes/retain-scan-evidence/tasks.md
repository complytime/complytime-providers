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
customer-owned. D8's launcher compatibility work is necessary for the current
runtime mapping contract and is not the future collection feature. The companion
complies with D9's original-caller identity contract as well as D8's workspace
contract; this repository implements provider consumption, not the launcher.

Selected decisions: caller ownership, private 0700/0600 modes, process CWD as the
provider workspace root, absolute file URLs, and no Speckit migration are recorded
in the design. Do not ask the author to choose them again. Separately owned complyctl
follow-up establishes launcher alignment and reliable elevated-caller identity;
record its reference, mechanism, compatible version and sequencing in task 5.4.
The companion implementation must land before end-to-end acceptance and release,
not before publication of this design-feedback draft. The repository instruction
conflicts remain reported for governance reconciliation; no rule is waived here.

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

Use process-isolated provider tests with an explicit child working directory and
injected identities for elevated cases. These tests run under `make test` without a
complyctl checkout, credentials, privileged execution, or parent-process chdir.
Use injected identity/filesystem operations for ownership failures and inspect real
UID/GID results separately in controlled privileged integration tests. Preserve
Ampel's existing RPC error for digest failure; do not misclassify it as a response
error while adding the new finalization errors from D6/D9.

## 1. Shared evaluation storage and context

- [ ] 1.1 Implement evaluation allocation and centralized naming/0700/0600-mode
  constants in focused `internal/evidence` files, deriving the results root from
  `provider.WorkspaceDir` and existing provider constants. Centralize and document
  the bounded retry limit with a test override. Verify exact timestamp/path form,
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
- [ ] 1.7 Implement selected caller-ownership finalization using OS identity for
  normal invocation and verified caller identity for elevated cases. Test normal
  invocation without privileged chown, elevated-on-behalf
  handoff, direct root, missing/malformed/unverified identity, failed chown or owner
  verification, and inaccessible existing parents. Include supporting files present
  after metadata failure. Verify files are finalized before directory handoff, no
  unrelated ownership is changed, and failures return RPC errors without a response.
  Assert directory mode 0700 and file mode 0600 after the reported governance
  conflict has been reconciled; the intended modes are selected, not an open choice.
- [ ] 1.8 Resolve process CWD once as the absolute root. Verify root-resolution
  failure and that inherited workspace variables cannot override the supplied CWD.
  Use the credential-free child-process harness to prove writes, coordinates, and
  URLs agree beneath that root without a new workspace-confirmation message or
  independent application of complyctl's environment/flag precedence.

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
  breaking. Document caller ownership and the supported non-root read/verify
  workflow after elevated scans, including approved modes and parent traversal.
  Regenerate its man page and verify Generate-time paths remain accurately documented.

## 4. OPA result-file evidence

- [ ] 4.1 Separate conftest stdout from stderr at the runner boundary and retain exact JSON output as `conftest.json`; verify valid stdout with diagnostic stderr, malformed output, tool errors, and `--no-fail` compliance outcomes.
- [ ] 4.2 Allocate readable evaluation directories for remote target/branch and local targets, then parse and hash the retained output; verify remote, local-file, and local-directory evidence coordinates all identify `conftest.json` and carry its SHA256 digest.
- [ ] 4.3 Keep working clones at the existing replaceable location and local inputs in place; verify three-scan tests create no historical clone/input snapshots and earlier result evidence survives clone replacement and local input edits/deletion.
- [ ] 4.4 Replace input-path evidence references with result-file references, attempt supporting metadata persistence after context is finalized, and stop writing legacy summaries; verify metadata includes non-secret source identity, branch/scan path, available policy/configuration context, and consistent collection timestamps.
- [ ] 4.5 Cover repeated targets, multiple branches in one request, different scan paths on the same repo/branch, colliding display labels, and multiple assessments per evaluation; verify separate evaluation tokens, coordinates, IDs, and matching reference URLs without copying artifacts per assessment or modifying the #199 encoding helpers.
- [ ] 4.6 Cover output write/close, parse, digest, and metadata failures; verify incomplete evidence produces operational errors, metadata failures warn without suppressing complete evidence, and previously retained results stay unchanged.
- [ ] 4.7 Update OPA man-page FILES/EVIDENCE sections and README evidence-lifecycle guidance; replace the legacy summary path entry with a note that existing summaries remain untouched but are no longer written. Regenerate the man page and verify documentation distinguishes stored result evidence from replaceable inputs, explains storage growth, and marks coordinate/ID changes.

## 5. Integration verification and release readiness

- [ ] 5.1 Add three-scan regressions in each provider's `server/server_test.go`, using the test harness above with changing and identical fixture outputs. Seed legacy fixed-path artifacts and summaries with known bytes; resolve every `ScanResponse` coordinate against an explicit workspace root, verify its escaped absolute file URL identifies the same artifact, recompute each `sha256:` digest, and assert evaluation-qualified IDs and matching reference links. Verify earlier artifacts and legacy files remain unchanged without packaging or storage integration, and that supporting XCCDF/metadata files do not become implicit evidence. Copy the workspace tree and verify coordinates/digests against the copied root while absolute URLs remain unchanged and origin-specific; keep these contract tests runnable under `make test` without credentials or a complyctl checkout.
- [ ] 5.2 Verify the existing complyctl conversion preserves coordinates, digests, timestamps, absolute file URLs, and instance references in serialized Gemara Evaluation Logs. Validate converted logs against the applicable Gemara schema and exercise a downstream consumer; verify repeated evaluations do not lose references through deduplication and every reported evidence entry leads to its original file with the matching digest after later scans, without inferring extra evidence from directory contents.
- [ ] 5.3 Run `openspec validate retain-scan-evidence --strict`, `make build`, `make test`, `go test -race -count=1 ./...`, `make lint`, `make man`, and `git diff --check`; the race run includes shared concurrent-allocation tests and is an additional local convention-pack check, not currently provided by `ci_local.yml`. Inspect applicable workflows and pinned reusable definitions for additional local CI parity checks, recording external prerequisites rather than claiming unrun checks passed.
- [ ] 5.4 Verify the separately owned complyctl compatibility prerequisite; do not
  implement its launcher in this repository. Record its issue/spec, reliable elevated
  caller-identity mechanism, landed version and supported version floor. Confirm workspace precedence,
  child CWD, original caller UID/GID delivery, and caller-relative launch-input
  resolution without parent chdir. Verify default, environment, and flag workspaces,
  including invocation from A with workspace B, Generate/Scan root agreement, no new
  generated artifacts or evidence beneath A, and matching coordinates/URLs/digests
  beneath B. Run `make test-cross-repo COMPLYCTL_DIR=<built-complyctl-checkout>` with
  the compatible checkout and required tools/credentials; record blockers honestly.
  Verify actual elevated-to-user handoff and read/hash as the original user in a
  controlled privileged test. No collection, packaging, or storage feature is part
  of this companion prerequisite or this task.
- [ ] 5.5 Update root `README.md`, `cmd/*-provider/README.md`, and all provider man pages with the selected workspace root as the coordinate base and absolute `file://` URLs as originating-filesystem locators. Document workspace precedence, how to locate the root, URI escaping, and the in-place workflow of reading the log's evidence mappings, resolving files, and checking digests without packaging. Explain that the Gemara Evaluation Log alone defines the evidence set, supporting files are not implicitly evidence, preservation is always on without automatic cleanup or retention settings, and future optional collection/packaging must map moved files without rewriting the log. State that storage backends and longevity remain customer-owned and absolute URLs do not automatically relocate; verify examples and regenerate affected man pages.

  Also document original-caller ownership, direct-root behavior, approved modes,
  directory traversal and workspace writability requirements, and failures that
  leave unreferenced partial outputs. Include troubleshooting for permission and
  disk-space failures, and metadata's best-effort status. Do not promise access to
  arbitrary other users or historical evidence that was already overwritten.
- [ ] 5.6 Document upgrade and rollback behavior in README and provider man pages: fixed-path outputs stop being refreshed, existing files and historical logs remain untouched and are not migrated, previously overwritten evidence cannot be recovered, and older binaries resume overwrite behavior while retained directories remain readable. Verify this guidance against the design's Migration Plan and regenerate affected man pages.
- [ ] 5.7 Add CHANGELOG entries under `Breaking Changes` for OPA result-file evidence, new output paths and instance-qualified IDs without fixed-path aliases, workspace-root-relative coordinates, and discontinued Ampel/OPA normalized summaries; record retained evidence under `Added` and updated guidance under `Documentation`. State that absolute `file://` URL representation is retained and legacy files and logs remain untouched, complete the AGENTS structural assessment from 1.5, confirm README and all man pages match the implemented layout, and file the required `unbound-force/website` issue before implementation merge; verify entries and the issue describe shipped behavior and local preservation/mapping, not an implemented collection, packaging, or managed retention feature.

  Supersede conflicting Unreleased claims about OPA input-path evidence and old ID
  forms so the release does not announce incompatible contracts as current at once.
  Cross-reference both capability specs and record the compatible complyctl version
  required by the workspace/identity contract. Do not mark unrun schema, privileged,
  or cross-repository tests passed.
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
