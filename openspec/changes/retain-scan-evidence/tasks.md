# Tasks

These tasks describe future implementation, not work completed by the specification
commit. Begin only after user review and after the spec artifacts are committed and
pushed. Groups 2 through 4 depend on group 1; each provider group carries its own
tests and documentation. Preserve the existing untracked main-spec files until their
ownership and synchronization are resolved before archive.

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

## 1. Shared evaluation storage and context

- [ ] 1.1 Implement the evaluation allocation value and centralized naming/mode constants in `internal/evidence`; verify fixed-clock repeated allocations, concurrent allocations, forced candidate collisions, clock rollback, allocation failure, and retry exhaustion tests preserve existing files.
- [ ] 1.2 Implement bounded readable labels and root-constrained context-directory creation separately from `SanitizeForID`; verify D1's character set, 0/1/79/80/81-byte inputs and non-empty fallback, slash-containing branches, long paths, lossy collisions, URL credential exclusion, and a symlinked context parent pointing outside the root. Cover `.`, `..`, absolute paths, `a/../../b`, NUL/control characters, and empty-after-normalization labels, asserting safe components or rejection without writes outside the results root.
- [ ] 1.3 Implement best-effort versioned metadata persistence with an explicit allowlist and readable target correlation keys; verify the D5 envelope and allowed provider fields, omission of digests/environment/target-variable maps, workspace-relative internal paths and absolute external provenance paths per D8, context surviving label truncation, documented key normalization, credential exclusion, and write/close failures warning without blocking evidence.
- [ ] 1.4 Add shared artifact-instance identity/reference construction and timestamp propagation support; replace superseded `FileURL` usage/tests with workspace-relative URL/escaping coverage and remove the helper if unused. Verify one artifact's repeated references agree, distinct evaluations do not collapse, and escaped workspace-relative URLs and coordinates resolve to the same file against the workspace root; assert `Source.ReferenceID` matches its mapping ID without requiring `Evidence.ID` equality.
- [ ] 1.5 Document helper guarantees and limitations in GoDoc and assess AGENTS impact; verify API comments cover allocation errors, returned paths, collection-start semantics, and filesystem side effects.

## 2. Ampel retention

- [ ] 2.1 Allocate a readable repo/branch/spec evaluation directory per execution and pass its paths into `ScanRepository`; verify three repeated scans, two specs, and multiple branches in one request retain independent attestation pairs with distinct tokens, coordinates, and IDs, including slash-containing branches.
- [ ] 2.2 Finalize timestamps, digests, paths, and instance-qualified references before publishing assessments; attempt metadata persistence and stop writing legacy summaries. Verify each source/reference pair resolves to its stored attestation and multiple assessments share artifacts.
- [ ] 2.3 Add failure regressions for missing/invalid tool output and write/close/digest failures; verify no stale or incomplete evidence is published, metadata failures warn without suppressing complete evidence, and valid failing-compliance results remain retained.
- [ ] 2.4 Update Ampel man-page FILES/EVIDENCE sections with layout, spec-versus-policy context, and growth behavior; replace the legacy summary path entry with a note that existing summaries remain untouched but are no longer written. Regenerate its man page and verify examples match integration-test output.

## 3. OpenSCAP retention

- [ ] 3.1 Replace Scan-time global result-path use with explicitly threaded evaluation ARF/XCCDF paths through command construction, parsing, digesting, and response construction; verify commands and readers use the same allocated paths without mutating package globals.
- [ ] 3.2 Attempt to record request target, observed ARF target, profile, datastream, and tailoring context, and build instance-qualified ARF references; verify a three-scan regression preserves prior ARFs and digests, including valid noncompliant results and an evaluation crossing a timestamp boundary.
- [ ] 3.3 Add operational-error tests for allocation, missing/invalid required outputs, and digest failures; verify no prior fixed-path ARF is reused, incomplete evaluations publish no evidence, and metadata failures warn without suppressing complete evidence.
- [ ] 3.4 Update OpenSCAP man-page FILES/EVIDENCE sections with target/profile grouping and retained ARF semantics; regenerate its man page and verify Generate-time paths remain accurately documented.

## 4. OPA result-file evidence

- [ ] 4.1 Separate conftest stdout from stderr at the runner boundary and retain exact JSON output as `conftest.json`; verify valid stdout with diagnostic stderr, malformed output, tool errors, and `--no-fail` compliance outcomes.
- [ ] 4.2 Allocate readable evaluation directories for remote target/branch and local targets, then parse and hash the retained output; verify remote, local-file, and local-directory evidence coordinates all identify `conftest.json` and carry its SHA256 digest.
- [ ] 4.3 Keep working clones at the existing replaceable location and local inputs in place; verify three-scan tests create no historical clone/input snapshots and earlier result evidence survives clone replacement and local input edits/deletion.
- [ ] 4.4 Replace input-path evidence references with result-file references, attempt supporting metadata persistence after context is finalized, and stop writing legacy summaries; verify metadata includes non-secret source identity, branch/scan path, available policy/configuration context, and consistent collection timestamps.
- [ ] 4.5 Cover repeated targets, multiple branches in one request, different scan paths on the same repo/branch, colliding display labels, and multiple assessments per evaluation; verify separate evaluation tokens, coordinates, IDs, and matching reference URLs without copying artifacts per assessment or modifying the #199 encoding helpers.
- [ ] 4.6 Cover output write/close, parse, digest, and metadata failures; verify incomplete evidence produces operational errors, metadata failures warn without suppressing complete evidence, and previously retained results stay unchanged.
- [ ] 4.7 Update OPA man-page FILES/EVIDENCE sections and README evidence-lifecycle guidance; replace the legacy summary path entry with a note that existing summaries remain untouched but are no longer written. Regenerate the man page and verify documentation distinguishes stored result evidence from replaceable inputs, explains storage growth, and marks coordinate/ID changes.

## 5. Integration verification and release readiness

- [ ] 5.1 Add three-scan regressions in each provider's `server/server_test.go`, using the test harness above with changing and identical fixture outputs. Seed legacy fixed-path artifacts and summaries with known bytes; resolve every `ScanResponse` coordinate and escaped relative URL against an explicit workspace root, recompute each `sha256:` digest, and assert evaluation-qualified IDs and matching reference links. Copy the workspace tree and repeat resolution/digest assertions, confirming earlier artifact bytes and legacy files remain unchanged; keep these contract tests runnable under `make test` without credentials or a complyctl checkout.
- [ ] 5.2 Verify the existing complyctl conversion preserves new coordinates, digests, timestamps, and instance references in serialized Evaluation Logs; verify repeated evaluations do not lose references through deduplication and test documented workspace-relative URL resolution with the integration harness.
- [ ] 5.3 Run `openspec validate retain-scan-evidence --strict`, `make build`, `make test`, `go test -race -count=1 ./...`, `make lint`, `make man`, and `git diff --check`; the race run includes shared concurrent-allocation tests and is an additional local convention-pack check, not currently provided by `ci_local.yml`. Inspect applicable workflows and pinned reusable definitions for additional local CI parity checks, recording external prerequisites rather than claiming unrun checks passed.
- [ ] 5.4 Coordinate a companion complyctl change updating flat-path integration assertions and affected evidence documentation; run `make test-cross-repo COMPLYCTL_DIR=<built-complyctl-checkout>` with that checkout and required external tools and credentials, recording blockers and sequencing.
- [ ] 5.5 Update root `README.md`, `cmd/*-provider/README.md`, and all provider man pages with the workspace root containing `.complytime` as the coordinate and URL base, including how to locate it, URI escaping, and transferring the log with its referenced workspace layout. Explicitly document always-on retention, no automatic cleanup or retention settings, and the preservation guarantee; verify examples resolve against that root rather than the log directory and regenerate affected man pages.
- [ ] 5.6 Document upgrade and rollback behavior in README and provider man pages: fixed-path outputs stop being refreshed, existing files and historical logs remain untouched and are not migrated, previously overwritten evidence cannot be recovered, and older binaries resume overwrite behavior while retained directories remain readable. Verify this guidance against the design's Migration Plan and regenerate affected man pages.
- [ ] 5.7 Add CHANGELOG entries under `Breaking Changes` for OPA result-file evidence, new output paths and instance-qualified IDs without fixed-path aliases, workspace-root-relative coordinates and scheme-less reference URLs, and discontinued Ampel/OPA normalized summaries; record retained evidence under `Added` and updated guidance under `Documentation`. State that legacy files and logs remain untouched, complete the AGENTS structural assessment from 1.5, confirm README and all man pages match the implemented layout, and file the required `unbound-force/website` issue before implementation merge; verify entries and the issue describe shipped behavior and the output-only audit guarantee.
- [ ] 5.8 Run the required review council before implementation PR submission and resolve all REQUEST CHANGES; verify approval remains valid with no intervening code changes, leaving cleanup and the broader #199 encoding work deferred.

When replacing old contracts in groups 2–4, rewrite existing fixed-path ARF,
absolute `file://` URL, and input-path OPA assertions to check the new retained
artifact contract. Remove unused `WritePerRepoResult`/`WritePerTargetResult`
summary writers and replace their failure-propagation tests with required-artifact
write/close failure coverage rather than deleting the error-path protection.

## Workflow follow-up

- Archive this change after implementation, verification, and review. Reconcile the
  existing untracked synchronized specs before updating canonical specs at archive.
- Keep issue #199 separate; link the artifact/logical-identity separation as context
  without claiming the general encoding assessment is complete.
- Address cleanup only after this feature merges, using a separately reviewed change.
