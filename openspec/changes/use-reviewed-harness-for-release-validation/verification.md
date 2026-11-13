# Verification record

## Scope and prerequisites

On 2026-11-13, the maintainer resolved the obsolete historical-dispatch
test contract in the conversation after reviewing its exact purpose.
Planning clarification commit `9b7a525` was committed and pushed before
implementation. No quality/governance rule, CI permission, assertion
severity, threshold, timeout, or failure policy was relaxed.

GitHub reports approved complyctl PR #975 merged as
`3cdfdd2a4f2d66f186b91e8558710d07e6df6413`. Its integration script,
host-selection helper, and evidence helpers have the same Git blob hashes
as the locally inspected PR checkout. The workflow pins this merged
commit, not the prior branch head. See design for the earlier contract's
supersession and archive ordering; historical artifacts remain untouched.

## Passing local checks

- `make build`: all three providers built.
- `make test`: all provider packages passed; Go used cached results for
  some unchanged packages. This is the exact local CI command, not a
  credentialed cross-repository run.
- `make lint`: zero issues.
- `TMPDIR=/tmp/opencode bash
  .github/scripts/test/resolve-complyctl-release-baseline_test.sh`: passed.
- `TMPDIR=/tmp/opencode bash
  .github/scripts/test/validate-released-complyctl-checkout_test.sh`: passed.
- `bash .github/scripts/test/released-baseline-diagnostics_test.sh`: passed.
- `TMPDIR=/tmp/opencode bash
  .github/scripts/test/canonical-complyctl-harness_test.sh`: 34 cases passed.
- `shellcheck .github/scripts/*.sh .github/scripts/test/*.sh`: passed.
- `yamllint -c .yamllint.yml
  .github/workflows/ci_cross_repo_integration.yml`: passed.
- `actionlint .github/workflows/ci_cross_repo_integration.yml`: passed.
- `openspec validate use-reviewed-harness-for-release-validation --strict`:
  passed.
- `git diff --check`: passed.

The execution suite exercises the actual dispatch helper and provider
Make target using distinguishable host, registry, and provider fixtures.
Exact traces exclude default binaries, historical dispatch, rebuilds,
and fallback. Cases include paths with spaces, immutable-revision checks,
missing helpers/fixtures/binaries, unsupported host selection, modified
source, symlinked host binaries, provider workspace mismatch, fixture-only
mode rejection, and nonzero integration output/status propagation.

The same suite extracts and executes the workflow's isolated-workspace,
dependency replacement/resolution, provider build/test, and integration
commands with offline tool fixtures. All four provider preparation/check
failures stop integration even while alternate prebuilt providers exist.
This verifies workflow wiring, not real released-dependency compilation.

Real released-dependency compilation was verified separately, without a
token, in `/tmp/opencode/released-harness-check.Z7m5m1`. The unchanged
release resolver selected v1.2.0 at
`f10c1426f3674bbcb967a8aad8ca529b32e21212`. A disposable released
checkout built both hosts with `make build`; its tracked source remained
clean according to `git diff --exit-code HEAD --`. A separate provider
checkout at `9b7a525` ran the actual workflow commands successfully:

```bash
go mod edit -replace=github.com/complytime/complyctl=../_released_complyctl
go mod tidy
GOFLAGS=-mod=mod make build
GOFLAGS=-mod=mod make test
```

All three provider binaries built and all package tests passed against
the real released dependency. No provider Go source changed between that
planning commit and this implementation; the new helpers, quoted harness
invocation, and workflow dispatch are exercised separately by the current
working-tree execution suite. This build/test result is not a live scan.
The original provider working tree's module files were not changed.

An independent comparison against the pre-implementation workflow confirms
the entire main-consumer job is unchanged. Workflow permissions are
unchanged, actions remain full-SHA pinned, no continue-on-error was added,
and the provider Make target still enforces its 120-second bound.

## Verification still required

`GITHUB_TOKEN` is not set in this session. No credentials were retrieved
from another source. Tasks 5.1 and 5.2 remain incomplete: no real retained
provider scan was run against either the released host or main. Earlier
companion-PR results and local offline fixtures do not establish those
results for this provider change.

Before completion, run the real released-host job with the pinned harness
and separately run the unchanged main-consumer job. Record actual provider,
host, and harness revisions, tool versions, mapped evidence results, and
released source identity. The workflow-derived isolated released build
and tests passed above with real Go tools, but do not replace live scans.

Task 5.3 remains open: strict validation passed, but required pre-PR
review council has not run. At the maintainer's request to open a PR,
the council command and dispatch skill were loaded. Tool discovery did
not expose its required `capture_review_diff`, `plan_review_dispatch`,
or `dispatch_review_runs` tools. No reviewer sessions were launched,
no council approval or finalization artifact was produced, and no PR
was opened. The available native subagent and forge-review tools are
not substitutes for the command's required dispatch policy.

The maintainer intends to run pending live local checks after PR
creation; keep those tasks open and report CI results independently.
Council review must first run in a runtime providing its required tools.
Do not submit or archive this implementation as complete until live
prerequisites and review are satisfied.

## Documentation impact

- `CHANGELOG.md`: records the canonical-harness CI correction.
- `AGENTS.md`: adds the offline execution-suite command, its prerequisites,
  and the workflow actionlint command; governance text is unchanged.
- `README.md`: links to released-host reproduction guidance.
- `docs/dev-testing-environment.md`: explains revision ownership,
  initial pin provenance, reviewed refreshes, local prebuilt dispatch,
  provenance inspection, diagnostics, and offline-versus-live limits.

Unrelated pre-existing untracked files were not staged or changed.
