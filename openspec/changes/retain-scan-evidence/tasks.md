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

- [x] 1.1 Implement evaluation allocation and centralized naming/0700/0600-mode
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
- [x] 1.2 Implement bounded readable labels and root-constrained context-directory creation separately from `SanitizeForID`; verify D1's character set, 0/1/79/80/81-byte inputs and non-empty fallback, slash-containing branches, long paths, lossy collisions, URL credential exclusion, and a symlinked context parent pointing outside the root. Cover `.`, `..`, absolute paths, `a/../../b`, NUL/control characters, and empty-after-normalization labels, asserting safe components or rejection without writes outside the results root.
- [x] 1.3 Implement best-effort versioned metadata persistence with an explicit allowlist and readable target correlation keys; verify the D5 envelope and allowed provider fields, omission of digests/environment/target-variable maps, workspace-relative internal paths and absolute external provenance paths per D8, context surviving label truncation, documented key normalization, credential exclusion, and write/close failures warning without blocking evidence.
- [x] 1.4 Add shared artifact-instance identity/reference construction and timestamp
  propagation support. Pass only validated, already-absolute artifact paths to
  `FileURL`; do not rely on its relative-input/failure fallback. Cover spaces,
  percent signs, and non-ASCII names. Verify repeated references agree, distinct
  evaluations do not collapse, and URLs identify the same files as coordinates
  resolved against the supplied workspace root. Assert `Source.ReferenceID` matches
  its mapping ID without requiring `Evidence.ID` equality.
- [x] 1.5 Document helper guarantees and limitations in GoDoc and assess AGENTS impact; verify API comments cover allocation errors, returned paths, collection-start semantics, and filesystem side effects.
- [x] 1.6 Implement root-constrained artifact access through allocation, write,
  parse, hash, and finalization. Test symlinked context parents, artifact symlink
  substitution, non-regular outputs, workspace-root symlink resolution, and
  validated/hashed file agreement without external reads or ownership changes.
- [x] 1.7 Implement execution-account ownership/mode verification using effective
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
- [x] 1.8 Resolve Generate global/Scan target `workspace` variables per D8 with
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

- [x] 2.1 Allocate a readable repo/branch/spec evaluation directory per execution and pass its paths into `ScanRepository`; verify three repeated scans, two specs, and multiple branches in one request retain independent attestation pairs with distinct tokens, coordinates, and IDs, including slash-containing branches.
- [x] 2.2 Finalize timestamps, digests, paths, and instance-qualified references before publishing assessments; attempt metadata persistence and stop writing legacy summaries. Verify each source/reference pair resolves to its stored attestation and multiple assessments share artifacts.
- [x] 2.3 Add failure regressions for missing/invalid tool output and write/close/digest failures; verify no stale or incomplete evidence is published, metadata failures warn without suppressing complete evidence, and valid failing-compliance results remain retained.
- [x] 2.4 Update Ampel man-page FILES/EVIDENCE sections with layout, changed
  coordinate/ID forms marked as breaking, spec-versus-policy context, and growth
  behavior. In the man page and provider README, replace legacy summary entries
  with the note that existing summaries remain untouched but are no longer written.
  Regenerate its man page and verify examples match integration-test output.

## 3. OpenSCAP retention

- [x] 3.1 Replace Scan-time global result-path use with explicitly threaded evaluation ARF/XCCDF paths through command construction, parsing, digesting, and response construction; verify commands and readers use the same allocated paths without mutating package globals.
- [x] 3.2 Attempt to record request target, observed ARF target, profile, datastream, and tailoring context, and build instance-qualified ARF references; verify a three-scan regression preserves prior ARFs and digests, including valid noncompliant results and an evaluation crossing a timestamp boundary.
- [x] 3.3 Add operational-error tests for allocation, missing/invalid required outputs, and digest failures; verify no prior fixed-path ARF is reused, incomplete evaluations publish no evidence, and metadata failures warn without suppressing complete evidence.
- [x] 3.4 Update OpenSCAP man-page FILES/EVIDENCE sections with target/profile
  grouping, retained ARF semantics, and changed coordinate/ID forms marked as
  breaking. Document execution-account ownership, approved modes and parent
  traversal, and authorized elevated reading/hashing after root-run scans.
  Regenerate its man page and verify Generate-time paths remain accurately documented.

## 4. OPA result-file evidence

- [x] 4.1 Separate conftest stdout from stderr at the runner boundary and retain exact JSON output as `conftest.json`; verify valid stdout with diagnostic stderr, malformed output, tool errors, and `--no-fail` compliance outcomes.
  Preserve `RunWithEnv`'s captured environment and composed ignore rule. Classify
  empty-directory diagnostics from stderr without putting them in retained JSON;
  keep unrelated execution failures distinct and preserve structured evaluated-input
  validation for `[]`, `null`, missing filename/namespace, and valid no-finding output.
- [x] 4.2 Allocate readable evaluation directories for remote target/branch and local targets, then parse and hash the retained output; verify remote, local-file, and local-directory evidence coordinates all identify `conftest.json` and carry its SHA256 digest.
- [x] 4.3 Keep working clones at the existing replaceable location and local inputs in place; verify three-scan tests create no historical clone/input snapshots and earlier result evidence survives clone replacement and local input edits/deletion.
  Preserve PR #233's `opa-input-selection` contracts with the new layout: enclosing
  scans exclude current/prior evaluation outputs, metadata, partial files, and other
  providers' artifacts; explicit workspace inputs and symlink/hard-link aliases into
  retained evidence are rejected. Ordinary selected clones remain eligible; aliases
  into retained results outside the clone do not. Preserve cyclic-clone isolation
  across independent targets and repeated scans. Measure representative accumulated
  histories using the existing workspace-preflight benchmarks, and cover inaccessible
  private history without skipping identity checks or changing earlier permissions.
- [x] 4.4 Replace input-path evidence references with result-file references, attempt supporting metadata persistence after context is finalized, and stop writing legacy summaries; verify metadata includes non-secret source identity, branch/scan path, available policy/configuration context, and consistent collection timestamps.
- [x] 4.5 Cover repeated targets, multiple branches in one request, different scan paths on the same repo/branch, colliding display labels, and multiple assessments per evaluation; verify separate evaluation tokens, coordinates, IDs, and matching reference URLs without copying artifacts per assessment or modifying the #199 encoding helpers.
- [x] 4.6 Cover output write/close, parse, digest, and metadata failures; verify incomplete evidence produces operational errors, metadata failures warn without suppressing complete evidence, and previously retained results stay unchanged.
  Extend `workspace_branches_test.go` for both branch orders: input-safety rejection,
  preflight cancellation, empty/unconfirmed directory input, and invalid directory
  output withhold all assessments/evidence/mapping references from that configured
  target. Preserve its operational errors and other targets' valid retained evidence.
  Already written branch artifacts remain unreferenced on disk; ordinary branch-scoped
  clone/tool/storage failures do not acquire target-wide rejection semantics.
- [x] 4.7 Update OPA man-page FILES/EVIDENCE sections and README evidence-lifecycle guidance; replace the legacy summary path entry with a note that existing summaries remain untouched but are no longer written. Regenerate the man page and verify documentation distinguishes stored result evidence from replaceable inputs, explains storage growth, and marks coordinate/ID changes.

## 5. Integration verification and release readiness

- [x] 5.1 Add three-scan regressions in each provider's `server/server_test.go`, using the test harness above with changing and identical fixture outputs. Seed legacy fixed-path artifacts and summaries with known bytes; resolve every `ScanResponse` coordinate against an explicit workspace root, verify its escaped absolute file URL identifies the same artifact, recompute each `sha256:` digest, and assert evaluation-qualified IDs and matching reference links. Verify earlier artifacts and legacy files remain unchanged without packaging or storage integration, and that supporting XCCDF/metadata files do not become implicit evidence. Copy the workspace tree and verify coordinates/digests against the copied root while absolute URLs remain unchanged and origin-specific; keep these contract tests runnable under `make test` without credentials or a complyctl checkout.
- [x] 5.2 Verify the existing complyctl conversion preserves coordinates, digests, timestamps, absolute file URLs, and instance references in serialized Gemara Evaluation Logs. Validate converted logs against the applicable Gemara schema and exercise a downstream consumer; verify repeated evaluations do not lose references through deduplication and every reported evidence entry leads to its original file with the matching digest after later scans, without inferring extra evidence from directory contents.
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
- [x] 5.5 Update root `README.md`, `cmd/*-provider/README.md`, and all provider man pages with the selected workspace root as the coordinate base and absolute `file://` URLs as originating-filesystem locators. Document workspace precedence, how to locate the root, URI escaping, and the in-place workflow of reading the log's evidence mappings, resolving files, and checking digests without packaging. Explain that the Gemara Evaluation Log alone defines the evidence set, supporting files are not implicitly evidence, preservation is always on without automatic cleanup or retention settings, and future optional collection/packaging must map moved files without rewriting the log. State that storage backends and longevity remain customer-owned and absolute URLs do not automatically relocate; verify examples and regenerate affected man pages.

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
- [x] 5.6 Document upgrade and rollback behavior in README and provider man pages: fixed-path outputs stop being refreshed, existing files and historical logs remain untouched and are not migrated, previously overwritten evidence cannot be recovered, and older binaries resume overwrite behavior while retained directories remain readable. Verify this guidance against the design's Migration Plan and regenerate affected man pages.
- [x] 5.7 Add CHANGELOG entries under `Breaking Changes` for OPA result-file evidence, new output paths and instance-qualified IDs without fixed-path aliases, workspace-root-relative coordinates, and discontinued Ampel/OPA normalized summaries; record retained evidence under `Added` and updated guidance under `Documentation`. State that absolute `file://` URL representation is retained and legacy files and logs remain untouched, complete the AGENTS structural assessment from 1.5, confirm README and all man pages match the implemented layout, and file the required `unbound-force/website` issue before implementation merge; verify entries and the issue describe shipped behavior and local preservation/mapping, not an implemented collection, packaging, or managed retention feature.

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

### Verification checkpoint (2026-11-11)

- 5.4 partial: `TestComplyctlScan_OfflineRetainedEvidence` exercises unchanged
  complyctl `ebed460cb55e79e9c4d8f1189ef3286e0e6a218e` (v1.1.0 build) with rebuilt
  OPA provider and real conftest 0.68.2 / OPA 1.15.2. Offline Generate and three
  successive Scans pass default/environment/flag workspace selection, invocation A
  with workspace B and flag-over-environment precedence. Serialized JSON/YAML logs
  preserve instance references, coordinates, SHA256, collection-start timestamps and
  escaped absolute URLs; earlier evidence still verifies after subsequent scans.
  Tests verify normal execution UID/GID and 0700/0600 modes. Unchanged OSCAL conversion
  accepts these logs and preserves source coordinates/digests/reference URLs.
- Opt-in `GEMARA_CUE_TEST_MODULE=github.com/gemaraproj/gemara@v1.6.0` validates all
  nine serialized OPA log snapshots with cue v0.17.1, using the schema module selected
  by the unchanged host's schema-validation helper. These are tested versions, not
  new runtime version floors.
- 5.2 complete: an isolated consumer harness at
  `/tmp/opencode/retain-evidence-consumer/` uses local module replacements, without
  changing either repository's dependencies, to exercise the actual three provider
  servers with deterministic mock tools and unchanged complyctl conversion code.
  `go test -mod=mod -race -count=1 -v ./...` passes there. For each provider, three
  changing/identical evaluations with two assessments each retain original files;
  earlier responses are re-converted after later scans and combined into one log.
  All 42 JSON/YAML log serializations validate against Gemara v1.6.0, preserve exact
  source fields/timestamps and resolve every digest/URL. All 21 unchanged OSCAL
  conversions pass their internal OSCAL schema validation and preserve source
  coordinates/digests/reference URLs. Same-log deduplication preserves six distinct
  Ampel artifact references and three each for OpenSCAP/OPA, while assessments share
  each evaluation's files. Unreferenced supporting files do not enter the log.
  This tests host conversion, not real Ampel/OpenSCAP execution or privileged access.
- Installed-conftest isolation tests pass without version-based gates/skips. Updated
  previously skipped fixtures create their workspace roots, resolve relative inputs
  against that root and check no-files diagnostics on stderr. Existing alias,
  workspace exclusion, empty-input and ignore-precedence behavior assertions remain.
  Existing offline operational-error and distinct-workspace-boundary tests pass.
- Full `make test-cross-repo COMPLYCTL_DIR=<unchanged-built-checkout>` was attempted
  with rebuilt providers and real tools but stopped at the harness's unset
  `GITHUB_TOKEN` prerequisite. The unchanged harness also still asserts legacy fixed
  Ampel paths; this provider-local change does not modify that consumer repository.
  Controlled host-root integration remains unrun; injected/root-identity unit tests
  and normal-account ownership checks do not substitute for it.
- 5.5–5.7 documentation is consolidated and man pages regenerated. Website follow-up:
  [unbound-force/website#407](https://github.com/unbound-force/website/issues/407).
  AGENTS structural assessment: no additional production packages or build targets;
  the opt-in schema-test command/prerequisites are documented. CHANGELOG supersedes
  old OPA input-evidence claims and records tested integration versus pending checks.
- 5.3 partial: `make build`, `make test`, `go test -race -count=1 ./...`, `make lint`,
  `make man`, strict OpenSpec validation and whitespace checks pass locally. The
  unit/race suites now have conftest on PATH rather than skipping its behavior tests;
  the separate offline CLI/schema command passes with all three binary variables.
  All three released-baseline shell suites, shellcheck and cross-repo workflow
  yamllint also pass. Tests/scripts/workflows and quality gates were not weakened.
- Inspected pinned org-infra `8f347ba1aa2c7e9eb3b70e69c1709ddb456cd64a` definitions
  for reusable CI and CRAP analysis. Explicit lint using its fallback `.golangci.yml`
  (standard linters plus gosec) passes; changed Go files also pass goimports checking.
  Full MegaLinter v10.1.0 execution, pinact remote action verification/freshness,
  PR-title validation and Gaze baseline/report analysis remain unrun. Those require
  their container/tool/remote or PR context; no baseline/settings/gates were created
  or changed to substitute for the pinned workflow. Released-baseline build/test and
  credentialed integration jobs remain unverified. 5.3 stays incomplete.

### Additional verification checkpoint (2026-11-11)

- 5.3 remains blocked: Gaze v2.3.0 completed its JSON report without analysis
  errors, but its unchanged committed-baseline comparison exited 1 with
  `passed: false` and four `new_violation` entries. Changed-package coverage tests
  passed with conftest available. Ampel `(*ProviderServer).Scan` reports CRAP
  33.2659 / GazeCRAP 33; shared `Allocate` reports 24.2188 / 89.125, and shared
  `EnsureDirectories` reports 10.8 / 110. These require implementation/test-quality
  investigation, not a threshold increase. OpenSCAP `RuleResultMessage` reports
  CRAP 72, identical to its committed baseline entry; its body is unchanged.
- Baseline comparability also needs investigation: committed entries use absolute
  source paths while current Gaze emits repository-relative paths. The comparison
  classifies 276 functions as new and 191 as removed, with no unchanged entries.
  No baseline, Gaze settings, thresholds or workflow gates were modified. Raw results
  are `/tmp/opencode/retain-evidence-ci-crap.json` and
  `/tmp/opencode/retain-evidence-ci-gaze-report.json`; the coverage profile is
  `/tmp/opencode/retain-evidence-ci-coverage.out`.
- MegaLinter v10.1.0 ran in a rootless Podman container against an isolated clone of
  implementation commit `366b7db`, in changed-file mode, using the repository's
  existing configuration and unmodified missing-config fallbacks from pinned
  org-infra. Image revision matches CI's
  `9949bad031045f366be2467e00e8371a7328a2e2`; image digest is
  `sha256:b09486c423063200d6f203b63121e0967719eee03c234844f6f1dd658f00a31b`.
  It exits 0: Go lint and betterleaks pass, while markdownlint reports 115
  non-blocking errors across 11 files. This is not a zero-issue lint result.
  It also reports that configured `REPOSITORY_KICS` was removed and is ignored;
  no scanner replacement or configuration change was made. Details are in
  `/tmp/opencode/retain-evidence-megalinter.log` and the isolated clone's reports.
- Installed pinact v3.10.1 completed `run --check --verify` and
  `run --check --update` with exit 0 and no output or working-tree edits. Its CLI
  lacks the pinned action's `--verify-min-age` option, so these checks do not
  establish full pinact-action parity. PR-title validation still needs PR context.
- Released-baseline resolution selects complyctl v1.2.0 at
  `f10c1426f3674bbcb967a8aad8ca529b32e21212`; a separate public checkout's exact
  commit and prerequisites were verified. The unchanged release builds as
  `v1.2.0+clean`. In an isolated provider worktree, the workflow's local dependency
  replacement, `go mod tidy`, `GOFLAGS=-mod=mod make build` and
  `GOFLAGS=-mod=mod make test` pass. The original provider dependencies and original
  complyctl checkout remain untouched.
- All offline CLI regressions also pass under `-race -count=1` against the released
  host and providers built against it, including three retained-evidence scans per
  workspace-selection case and opt-in Gemara schema validation, without skips.
  Logs are `/tmp/opencode/retain-evidence-released-provider-test.log` and
  `/tmp/opencode/retain-evidence-released-offline-cli.log`. Remote main still resolves
  to the previously tested `ebed460cb55e79e9c4d8f1189ef3286e0e6a218e`.
- 5.4 remains incomplete: `GITHUB_TOKEN` is unset, both unchanged consumer harnesses
  retain fixed-path Ampel assertions, controlled host-root reading/ownership is
  unverified, and the remaining host-level configuration/ignore scenarios are not
  established by the offline retained-evidence test. No credentials were provisioned
  or sudo attempted. 5.8 council remains pending; no PR, archive or remote
  synchronization was performed. Documentation impact: verification bookkeeping
  only; existing README, CHANGELOG and AGENTS feature/structure descriptions need
  no change for these additional test results.

### In-scope Gaze remediation checkpoint (2026-11-11)

- Following maintainer authorization, separated Ampel's per-evaluation allocation,
  execution/parsing and retention/finalization from request orchestration in
  `server/retention.go`. Named-result cleanup closes each evaluation on success or
  failure and suppresses the RPC response on a handle-close failure. Existing
  per-evaluation operational errors, publication-critical RPC errors, all-SKIP
  handling, metadata warnings and retained evidence behavior are preserved.
- Added regressions for continuing to a later spec after allocation/tool/parse
  failures, retaining only that spec's evidence/metadata, redacting the access token
  from response errors, preserving legacy files, and closing handles on success and
  failure. Allocation-verification, digest, finalization and close failures stop
  before later evaluations and return no response with the underlying error intact.
- Made allocation success/error returns explicit in the collision/rollback test;
  added concrete empty-directory, private-mode and collection-time assertions.
  Directory tests now explicitly check returned errors, both created paths,
  idempotence, existing parent identity/modes/content and validation before writes.
  Shared storage production code and its filesystem guarantees are unchanged.
- Focused Gaze v2.3.0 comparison against the unmodified committed baseline passes
  with no new violations in Ampel server/shared evidence packages. Ampel `Scan`
  reports CRAP 24.084 / GazeCRAP 24 (previously 33.2659 / 33); its new per-evaluation
  helpers have complexity 6/4/2 and 100% line coverage. `Allocate` reports CRAP
  24.2188 / GazeCRAP 23 (previously 89.125 GazeCRAP), and `EnsureDirectories` reports
  10.3375 / 10 (previously 110 GazeCRAP). Both shared functions now have 100%
  measured contract coverage. This resolves the three scoped new-function
  violations, not the baseline path comparability issue or all historical scores.
- Rebuilt all providers; `make test`, `go test -race -count=1 ./...`, `make lint`,
  explicit pinned-config Go lint and changed-file goimports checks pass. Real
  conftest isolation tests pass. Man pages regenerate without changes. The isolated
  released-provider worktree was updated with these four implementation/test files;
  its v1.2.0 dependency replacement build/tests and offline CLI/schema regressions
  pass with the workflow's required `GOFLAGS=-mod=mod`. A first offline command
  there omitted GOFLAGS and failed vendoring validation; no vendor files were
  changed, and the corrected workflow-equivalent command passes.
- The standalone all-provider consumer harness passes again, including 42 Gemara
  JSON/YAML validations and 21 OSCAL conversions. A separate temporary modfile also
  verifies it against isolated unchanged complyctl v1.2.0, avoiding sibling drift.
- The user's sibling complyctl checkout/binary has independently moved to
  `REVIEW_PR_967`, commit `a0e1a9ec476eadee25a8d4b0f1e0e27101c29049` (v1.2.0+dirty).
  Running existing offline fixtures against that binary now fails host-side catalog
  validation (`metadata.type` is Invalid, expected ControlCatalog), before provider
  execution. Those CLI failures were not fixed or counted as passing; no sibling
  source/build files were modified here. The same provider tests pass against the
  separately retained v1.2.0 host binary. Direct conversion with the moved sibling
  source also passes, which does not establish its CLI fixture compatibility.
- Logs use `/tmp/opencode/retain-evidence-gaze-fix-*`. Full changed-package Gaze
  initially exceeded a 120-second timeout. The bounded 600-second rerun completed:
  `/tmp/opencode/retain-evidence-gaze-fix-ci-crap-final.json` contains exactly one
  remaining violation, unchanged OpenSCAP `RuleResultMessage` at CRAP 72. The three
  scoped functions retain the focused scores and 100% contract coverage. The full
  comparison still exits 1 (`passed: false`), classifying 282 functions as new and
  191 as removed because of the unresolved path comparability issue. No full CI
  pass or genuine zero-regression baseline result is claimed. Tasks 5.3,
  5.4 and 5.8 remain incomplete; no baseline, threshold, workflow gate, credential,
  ownership policy or consumer fixture was changed. No PR, archive or push occurred.
- Documentation impact: this is behavior-preserving internal decomposition and
  regression coverage. README/CHANGELOG descriptions remain accurate; the extra
  Ampel server file introduces no package, build target or convention requiring an
  AGENTS structural update. Verification outcomes are recorded here.

### Upstream verification findings (2026-11-11)

- Rechecked the pinned reusable workflow's install command: it uses
  `github.com/unbound-force/gaze/cmd/gaze@latest`, which currently resolves to
  v1.9.0, not the separately installed v2.3.0. Installed v1.9.0 only beneath
  `/tmp/opencode/retain-evidence-ci-gaze-v1/`; the normal Gaze binary is unchanged.
  Its changed-package comparison uses the existing post-remediation coverage
  profile and unmodified committed baseline. It exits 1 with `passed: false`,
  282 new functions, 191 removed, zero matched functions and one new violation.
  Results: `/tmp/opencode/retain-evidence-ci-gaze-v1-crap.json` and `.err`.
  This reproduces the v2 result with the workflow-selected version; it is not
  a fresh coverage run or a passing CI comparison.
- Both versions relativize report paths but compare literal file/function keys
  against the historical absolute paths. Confirmed `RuleResultMessage`'s full
  function body is byte-identical to merge base
  `4040a669897da2f37fef8849a1e3af71966daf9b`; its baseline and current complexity 8,
  zero measured package coverage and CRAP 72 agree. Classify that score as
  pre-existing, not an in-scope retention regression. The path mismatch still
  prevents a reliable whole-report regression comparison. With user authorization,
  reported [Gaze #330](https://github.com/unbound-force/gaze/issues/330) rather than
  rewriting the baseline or changing thresholds.
- Confirmed a separate false-pass bug in pinned and current org-infra workflow:
  `.comparison.passed // true` converts an explicit `false` to `true`. The workflow
  suppresses the Gaze exit code and derives its enforcement status from this
  expression. Applying it to the failing v1 report returns `true`; this is not
  acceptable passing verification. Reported
  [org-infra #738](https://github.com/complytime/org-infra/issues/738). No workflow,
  gate, baseline or settings were changed in this provider feature.
- Classified the historical MegaLinter report at `366b7db` by exact-line provenance
  against the merge base: 105 findings occur on unchanged lines, and 10 on
  branch-added/modified lines (nine MD013 and one MD046). This is not a fresh lint
  run or a complete causality analysis. The MD046 location is an indented list in
  the OPA man page's Pandoc definition list; do not blindly rewrite its semantics.
  Details: `/tmp/opencode/retain-evidence-markdown-provenance.json`. No formatting
  fixes were made after the user directed out-of-scope issues upstream.
- Tasks 5.3/5.4/5.8 remain incomplete. Credentialed integration and controlled
  host-root ownership/read/hash checks remain unauthorized/unrun; `GITHUB_TOKEN`
  is unset. No sibling edits, agent dispatch, PR, push or archive occurred.
  Documentation impact: verification bookkeeping and upstream links only; no
  README, CHANGELOG or AGENTS feature/structure change is needed.

### Unprivileged readiness follow-up (2026-11-11)

- Added `retention_config_e2e_test.go`, using unchanged complyctl v1.2.0
  (`f10c1426f3674bbcb967a8aad8ca529b32e21212`), real conftest 0.68.2 / OPA 1.15.2,
  rebuilt OPA provider and bubblewrap network isolation. Seven new scenarios pass:
  workspace configuration discovery, relative explicit configuration, environment
  ignore precedence, ambiguous discovered/explicit configuration, missing relative
  configuration and malformed ignore. Rejection cases also exercise recovery using
  the documented explicit effective-ignore remedy in the same fixture.
- Conflicting invalid input/configuration fixtures in invocation A prove actual
  execution selects workspace B. Tests inspect the exact evaluated filenames and
  findings in retained conftest bytes, then verify serialized log coordinates,
  SHA256, IDs, escaped absolute URLs, timestamps, metadata and private modes/owners.
  No generated index, retained results or scan logs appear beneath A. Rejected
  scans allocate no evaluations and publish no evidence or mapping references.
  The unchanged host does serialize operational-error logs; an initial new test
  incorrectly expected no log at all. Corrected that new assertion to inspect the
  error log's empty evidence/references, without changing any existing assertion
  or production behavior. Initial/final logs are
  `/tmp/opencode/retain-evidence-config-cli.log` and
  `/tmp/opencode/retain-evidence-config-cli-final.log`.
- The entire offline CLI suite passes with `-race -count=1`, explicit binary
  prerequisites and opt-in Gemara CUE validation, including the new cases and
  retained-evidence default/environment/flag scenarios. No offline test skips.
  Log: `/tmp/opencode/retain-evidence-readiness-offline-cli.log`. No sibling
  checkout/binary changes or fixture changes for its independently moved branch
  were needed; these results apply to the isolated unchanged released host.
- The pinned pinact-action v3.0.0 uses pinact v4.0.0 according to its pinned
  `aqua/imports/pinact.yaml`. Installed that version only under
  `/tmp/opencode/retain-evidence-pinact-v4/`. Its `run --check --verify
  --verify-min-age` and `run --check --update` both exit 0 without modifications;
  a debug run confirms workflow files were processed. Token environment variables
  were unset. Logs are `/tmp/opencode/retain-evidence-pinact-v4-{verify,freshness,debug}.log`.
  This closes the earlier unsupported-flag prerequisite, without adding a minimum
  age policy: this repository has no pinact config specifying a nonzero age floor.
- With user authorization to report confirmed upstream incompatibilities, filed
  [complyctl #970](https://github.com/complytime/complyctl/issues/970) for the
  current/released integration harness's legacy Ampel fixed-path assertions.
  The report distinguishes source-confirmed incompatibility with the pending
  retention change from an unperformed credentialed run. No consumer source edits
  or fixed-path provider aliases were introduced.
- `make build`, `make test`, the full `go test -race -count=1 ./...` suite with
  conftest available, `make lint`, pinned-config Go lint, goimports, `make man`,
  strict OpenSpec validation and whitespace checks pass after these test changes.
  Pinned-config lint was initially started concurrently with `make lint` and hit
  the tool's process lock; its subsequent standalone run passes with zero issues.
  Logs use `/tmp/opencode/retain-evidence-readiness-*`.
- Refreshed pinned MegaLinter v10.1.0 in a new isolated clone at `c7364e4` plus
  the new test file, with unchanged missing-config fallbacks from pinned org-infra.
  Exit 0: Go lint and betterleaks pass, while the same 115 non-blocking Markdown
  findings and removed/ignored `REPOSITORY_KICS` warning remain. This is not a
  zero-issue lint result, and the snapshot precedes this verification-bookkeeping
  addition. An initial container run could not access the shared clone's alternate
  object path; repacked only that agent-created clone to make it self-contained
  before the successful run. Final log:
  `/tmp/opencode/retain-evidence-readiness-megalinter-final.log`.
- Installed-conftest isolation checks pass again. The standalone all-provider
  consumer harness also passes using its separate release modfile and unchanged
  v1.2.0 consumer: 42 Gemara JSON/YAML schema validations and 21 OSCAL conversions.
  No provider/consumer dependency files or original sibling sources were changed.
  Logs: `/tmp/opencode/retain-evidence-readiness-real-conftest.log` and
  `/tmp/opencode/retain-evidence-readiness-consumer.log`.
- After final test readability edits, repeated build, unit/race, both Go lint
  configurations, man generation, strict OpenSpec, whitespace and goimports checks,
  then the entire offline CLI/schema suite: all exit 0. Final logs use
  `/tmp/opencode/retain-evidence-readiness-final-*`. A second MegaLinter refresh
  includes the final test source and verification notes; it exits 0 with the same
  115 non-blocking Markdown findings and ignored KICS warning, recorded in
  `/tmp/opencode/retain-evidence-readiness-megalinter-current.log`.
  All three released-baseline shell suites, shellcheck and cross-repo workflow
  yamllint also pass again. No full CI pass or council approval is claimed.
- Council preparation found a separate execution prerequisite: the prescribed
  command requires `capture_review_diff`, `plan_review_dispatch` and
  `dispatch_review_runs`, but those tools are not exposed in this session's tool
  catalog. No council or child review was started; do not substitute manually
  selected reviewers or claim an approval. Existing Gaze comparison, credentialed
  integration and actual host-root verification blockers remain. Tasks 5.3, 5.4
  and 5.8 stay incomplete; no gates, settings, credentials, PR or push were changed.
- Documentation impact: regression verification only, with no new production
  package, build target, public behavior or prerequisite. Existing AGENTS offline
  test command already selects these tests; README and CHANGELOG descriptions
  remain accurate. No structural documentation change is needed.

### CI artifact-identity remediation (2026-11-11)

- PR #238 at `2543d20` failed both local-CI and released-v1.2.0 provider tests in
  `TestEvaluation_RejectsChangesAfterValidation/substitute`. The fixture unlinked
  and recreated identical bytes; the filesystem could recycle the original inode
  because `Read` had closed its only artifact handle. Device/inode plus byte
  comparison therefore did not reliably detect replacement.
- Added deterministic lifecycle regressions before changing production code;
  `/tmp/opencode/retain-evidence-identity-regression-before.log` records the expected
  failures. The tests observe real file handles through the existing trusted Stat
  boundary, without a new `/proc` or privileged-test prerequisite.
- `Read` now opens and checks a second constrained identity handle while its
  snapshot descriptor is still live. It preserves the existing checked read-close
  path, keeps one identity handle per validated artifact until evaluation cleanup,
  and releases redundant handles on repeated or rejected reads. `Close` attempts
  every artifact and directory close and joins failures without deleting output.
  This implements the existing same-artifact guarantee rather than changing it.
- The original substitution fixture now allocates identical replacement bytes
  while the original still exists, verifies distinct identities and atomically
  renames the replacement. New tests also retain unlink/recreate coverage, prove
  identity handles survive finalization, and cover read/open/stat/replacement/close
  failures, preservation of the original pin after rejection, bounded handles on
  repeated reads and releasing every handle despite a close failure. Focused
  regressions pass 100 repetitions with the race detector.
- Full build/unit/race, local and pinned-config Go lint, goimports, man generation,
  strict OpenSpec, whitespace, real-conftest isolation and offline CLI/schema tests
  pass. The isolated provider checkout using unchanged complyctl v1.2.0 also passes
  build/tests and the 100-repeat identity regressions with `GOFLAGS=-mod=mod`.
  The all-provider consumer harness passes again. Logs use
  `/tmp/opencode/retain-evidence-identity-*`.
- Focused workflow-selected Gaze v1.9.0 passes without new threshold violations:
  Read complexity 6 / CRAP 6.027 / GazeCRAP 7.333; Close 3 / 3 / 12;
  snapshot complexity 8 / CRAP 8.048; pinIdentity complexity 4 / CRAP 4.
  The historical absolute/relative path mismatch still prevents a genuine
  baseline-regression comparison. No settings, baseline or thresholds changed.
- Pinned MegaLinter v10.1.0 passes in a fresh isolated clone with unchanged
  fallback configs: Go lint/betterleaks pass, with the existing 115 non-blocking
  Markdown findings and ignored KICS warning. An initial rerun in the earlier
  lint clone found incomplete Go packages inside its generated
  `megalinter-reports/updated_sources/` tree, not in repository source. Preserved
  those diagnostics and reran in a fresh self-contained clone rather than deleting
  user files or changing exclusions. Final log:
  `/tmp/opencode/retain-evidence-identity-megalinter-final.log`.
- Live main-consumer integration completed real credentialed Ampel and OPA scans
  and reported 18 assertions passed, with only the two obsolete Ampel globs failing.
  Added that CI evidence and the frozen-release-harness follow-up to
  [complyctl #970](https://github.com/complytime/complyctl/issues/970#issuecomment-6792982703).
  The released-host integration itself was skipped after its provider unit failure;
  its legacy harness assertions still need the separate upstream spec/review.
  No consumer source, host revision, fixed-path aliases or CI gate was changed.
- CHANGELOG records the identity-lifetime bug fix and GoDoc describes handle
  lifetime/cleanup. Existing README/man behavior remains accurate; no new package,
  build target, dependency or AGENTS structural update is needed. User-owned
  uncommitted AGENTS governance edits remain untouched. Manual root acceptance and
  council execution remain unperformed; tasks 5.3/5.4/5.8 are not marked complete.

### PR #238 review remediation (2026-11-12)

- A native five-persona PR council reviewed frozen head
  `85186221bc9037170a0d0d968195c7331accd15a` against base
  `d8d9107e9c5b1af5fe78de699b7e745fd67c558a`. All five runs completed;
  the result was REQUEST CHANGES, not approval. The Adversary found no security
  defects. Curator reported no documentation defects in its domain; Guard identified
  abbreviated CHANGELOG spec references. No plugin-validated artifact was produced.
- Fixed OpenSCAP discovery under the existing D8/task 1.8 contract. Generate and
  Scan now use `WorkspaceConfig.ResolveDatastream`; relative `SSG_CONTENT_DIR` and
  `OS_RELEASE_FILE` resolve before filesystem reads, not after discovery. Explicit
  paths still expand home references and absolute external paths remain unchanged.
  The legacy package-level resolver retains its invocation-relative behavior.
- Extended the process-isolated workspace regression with workspace-only discovery,
  conflicting invocation fixtures, missing workspace content, missing workspace
  system information and absolute external discovery. Four scenarios failed before
  the fix. All eight explicit/discovery scenarios pass afterward under the race
  detector, checking Generate and Scan, selected paths, error channels and absence
  of tool execution on discovery errors without parent-process chdir.
- Removed two Ampel results tests whose filesystem fixtures were disconnected from
  the exercised operation. The existing three-scan server regression now compares
  the complete actual results-file inventory after each scan against legacy files,
  retained attestation pairs and metadata. Unexpected summaries fail that assertion;
  existing summary-byte preservation assertions remain in place.
- Replaced the three new breaking-change entries' abbreviated capability references
  with canonical paths to both delta specs. Recorded the discovery correction in
  CHANGELOG. All seven new canonical spec references resolve. Existing README/man
  descriptions already promise workspace-relative lookup; man regeneration is
  unchanged. No new package, build target or convention needs an AGENTS update.
- Local verification passes: `make build`, `make test`, full
  `go test -race -count=1 ./...`, `make lint` (zero issues), `make man`, strict
  OpenSpec validation and `git diff --check`. All three released-baseline shell
  suites, shellcheck and cross-repo workflow yamllint pass. The integration-tagged
  OpenSCAP config suite compiles with `-run '^$'`; actual system integration was
  not run. Real-conftest cases skip because conftest is absent from PATH; these
  skips do not validate behavior or replace prior real-tool evidence.
- Both PR integration suites fail the obsolete flat Ampel attestation assertions;
  their exact base-commit checks passed. Classify them as PR-caused, even though
  anticipated and tracked by [complyctl #970](https://github.com/complytime/complyctl/issues/970).
  Consumer migration and passing integration evidence remain separate follow-up;
  no consumer source, fixed-path aliases or verification gates were changed here.
- Tasks 5.3, 5.4 and 5.8 remain incomplete. This remediation does not claim full
  external CI parity, controlled host-root verification or fresh council approval.
  The original PR review binds only its frozen head; changed code requires review
  again. Unrelated user-owned workflow/tooling edits and untracked files are preserved.

### Remaining follow-up

- Archive this change after implementation, verification, and review. Reconcile the
  existing untracked synchronized specs before updating canonical specs at archive.
- Keep issue #199 separate; link the artifact/logical-identity separation as context
  without claiming the general encoding assessment is complete.
- Address cleanup only after this feature merges, using a separately reviewed change.
