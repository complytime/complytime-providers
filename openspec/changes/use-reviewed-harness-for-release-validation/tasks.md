# Tasks

Implementation starts after these artifacts are reviewed, committed,
and pushed. The first group contains real prerequisites, not permission
to bypass or edit quality/governance gates.

## 1. Reviewed prerequisites

- [x] 1.1 Verify the companion complyctl change has merged and its full
  immutable commit supplies mapped evidence checks and
  `COMPLYCTL_BIN_DIR`; record the verified commit and interface before
  writing a harness pin, without inventing an unmerged revision.
  Verified 2026-11-13: approved complyctl PR #975 merged as
  `3cdfdd2a4f2d66f186b91e8558710d07e6df6413`. GitHub tree hashes for
  the integration script, host-selection helper, and evidence helpers
  match the locally inspected and offline-tested PR files.
  `COMPLYCTL_BIN_DIR` selects both prebuilt host executables from an
  absolute directory; assertions and fixtures remain harness-owned.
- [x] 1.2 Record maintainer resolution of the historical-harness test
  contract in the existing diagnostic test and prior design; preserve
  released-host validation and review the replacement assertions with
  implementation, without a separate prerequisite PR or gate exemption.
  Resolved 2026-11-13 in the maintainer conversation: replace obsolete
  dispatch assertions with stronger executed provenance checks, keep
  the suite and all quality/governance gates enabled. See design.
- [x] 1.3 Check this delta against the completed unpromoted
  `validate-complyctl-release-compatibility` change; record the exact
  harness-contract supersession and archive ordering without modifying
  historical artifacts or weakening baseline/dependency requirements.
  Recorded in design: supersede only the release-owned harness decision
  at the earlier design's lines 103-113; reconcile the older change
  before promotion and never promote its obsolete contract afterward.

## 2. Canonical harness preparation

- [x] 2.1 Add the verified immutable canonical harness pin and separate
  checkout to the released-baseline job (FR-002); verify checkout HEAD
  matches the pin and no mutable or historical fallback is possible.
- [x] 2.2 Add focused preparation validation for the harness script,
  fixture root, and host-selection interface; verify offline positive,
  missing-path, wrong-revision, and unsupported-interface cases.
- [x] 2.3 Report the released tag/commit and canonical harness commit
  in logs and the job summary (FR-006); verify diagnostics tests retain
  stage categories and identify harness preparation separately.
- [x] 2.4 Document pin provenance and the reviewed refresh procedure in
  existing integration/contributor guidance; verify the documented
  pin names a merged commit and both revision identities are clear.

## 3. Released host dispatch and behavior tests

- [x] 3.1 After task 1.2 is resolved, dispatch via the provider Make
  target with canonical `COMPLYCTL_DIR`, absolute released
  `COMPLYCTL_BIN_DIR`, and isolated provider binaries (FR-003/004);
  verify an executed fixture trace identifies the intended inputs.
- [x] 3.2 Add distinct default/released host and registry fixture
  executables (FR-007); verify only the released pair executes and
  neither canonical checkout nor released source is rebuilt/patched
  by the dispatch path.
- [x] 3.3 Add negative execution cases for missing binaries, absent
  harness, revision mismatch, and nonzero integration (FR-002/003/005);
  verify failures remain nonzero with no fallback execution.
- [x] 3.4 Preserve isolated released dependency replacement and all
  provider build/test steps (FR-004); verify failures stop integration
  and tests reject reuse of binaries built against another host.
- [x] 3.5 Document local invocation and provenance inspection; verify
  paths with spaces and commands use prebuilt released binaries rather
  than the historical released Make integration target.

## 4. Workflow protections and local CI parity

- [x] 4.1 Verify the maintainer-resolved diagnostic contract exists
  without disabling the suite; verify stronger host-provenance and
  failure-propagation checks pass and old requirements are explicitly
  superseded, not evaded by checkout names.
- [x] 4.2 Run all existing release-baseline shell suites plus the new
  execution suite; verify resolver behavior, prerequisite rejection,
  integration failure propagation, and distinct main/released results.
- [x] 4.3 Run shellcheck on helpers/tests and configured yamllint and
  actionlint on the workflow; verify action pinning, permissions,
  timeouts, jobs, and quality/governance gates are unchanged.
- [x] 4.4 Run workflow-derived provider build/tests and project lint;
  record commands and outcomes without treating missing tools or
  skipped behavioral checks as successful verification.
- [x] 4.5 Assess `CHANGELOG.md`, `AGENTS.md`, and `README.md`; verify
  necessary helper/test-command and integration usage documentation is
  updated, or record why a file needs no change.

## 5. Live verification and review

- [ ] 5.1 Run real retained-provider integration against the selected
  unchanged released host through the pinned canonical harness; record
  host/provider/harness revisions, tool versions, mapped evidence
  results, and the unmodified released source identity.
- [ ] 5.2 Verify the separate main-consumer job still completes its
  Ampel/OPA assertions; record its host revision independently and do
  not use its success to stand in for task 5.1.
- [ ] 5.3 Validate this change strictly and complete required pre-PR
  review; verify all blocking findings are resolved and no subsequent
  implementation changes invalidate final approval. Missing live
  prerequisites leave affected verification tasks incomplete.

## Workflow follow-up

- Submit this repository's PR independently of the provider retention
  PR and only after the canonical complyctl harness commit exists.
- Reconcile the earlier unpromoted release-compatibility spec before
  archive so it cannot reintroduce the superseded historical-harness
  requirement. Do not archive or rewrite that older change here.
- Archive this change only after implementation and required review.
