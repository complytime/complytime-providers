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

## PR #241 CI verification

All GitHub Actions workflows passed for provider PR head
`22c81fb75d0a09a5c2f9c121a1191e1fb5ca9783`. Actions tested GitHub's
merge commit `04e7036bf30d84a3d8e6c250e2c89135eecc6536` against
base `d8d9107e9c5b1af5fe78de699b7e745fd67c558a`.

- [Released-host job][released-job]: v1.2.0 at
  `f10c1426f3674bbcb967a8aad8ca529b32e21212`, canonical harness
  `3cdfdd2a4f2d66f186b91e8558710d07e6df6413`: 19 assertions passed,
  zero failed. Isolated released-dependency builds/tests and the actual
  integration step succeeded, not skipped.
- [Independent main-host job][main-job]: host and harness at
  `3cdfdd2a4f2d66f186b91e8558710d07e6df6413`: 19 assertions passed,
  zero failed.
- Both logs show real Ampel/Snappy mapped-evidence validation and OPA
  failing/compliant fixtures. These CI providers used legacy artifact
  paths; these results alone do not prove retention across evaluations.
- Build/unit tests, linters, security scans, CRAP analysis, action-version
  verification, SonarCloud, RPM builds, and reported Testing Farm jobs
  passed. Dependabot-only jobs skipped; two auxiliary annotations were
  neutral. No failed or pending checks remained when last inspected.

## Agent-executed retained-provider acceptance

On 2026-11-13, the maintainer explicitly authorized managing local tests
and using existing `gh` authentication on their behalf. The token was
retrieved into memory and passed only through credentialed test-process
environments, not printed or saved by the test driver. Earlier missing
`GITHUB_TOKEN` reports describe the initial blocked attempt, not a remaining
verification blocker.

Disposable local clones under
`/tmp/opencode/retained-acceptance.4Jl9HD/` used these immutable revisions:

| Component | Revision |
| --- | --- |
| Retained providers from PR #238 | `d59b46ae86bcf5ff801cd524d51137faa8fd1e20` |
| Canonical assertions/fixtures | `3cdfdd2a4f2d66f186b91e8558710d07e6df6413` |
| Released host v1.2.0 | `f10c1426f3674bbcb967a8aad8ca529b32e21212` |
| Main host, matching PR #241 CI | `3cdfdd2a4f2d66f186b91e8558710d07e6df6413` |
| Provider dispatch helpers from PR #241 | `22c81fb75d0a09a5c2f9c121a1191e1fb5ca9783` |

Both host checkouts ran `make build`. Each of two separate retained-provider
checkouts replaced its complyctl module with its matching host checkout,
ran `go mod tidy`, and passed `GOFLAGS=-mod=mod make build` and
`GOFLAGS=-mod=mod make test`. Original working trees were untouched.
Host tracked source and provider `cmd/`/`internal/` source remained clean.

The released-provider clone additionally received only the two provider
preparation/dispatch helpers from PR #241 as an explicit temporary test
layer; its Makefile and provider implementation were not patched. The
released integration executed that dispatcher and the provider Make target.
The main integration executed its provider Make target with explicit
`COMPLYCTL_BIN_DIR`. Both used the same pinned canonical script and fixtures,
both actually ran with PR #238 providers, and both passed 19 assertions with
zero failures. Verified artifacts used distinct `evaluation-*` directories,
not legacy aliases. No copied or patched complyctl harness was used.

For each host, a separate persistent workspace and isolated HOME/XDG paths
then ran real Get, Generate, and two Ampel Scan invocations. Each invocation's
reported JSON EvaluationLog was selected with `select_evaluation_log` and
copied immediately into snapshot A or B, avoiding report-filename collisions.
After scan B, the canonical `validate_evidence_history` succeeded on both
snapshots. Independent calls to `validate_mapped_evidence` and byte hashing
also confirmed all four Ampel/Snappy artifacts per host still existed with
their recorded SHA-256 digests. A/B evidence IDs, references, coordinates,
and resolved paths remained independently addressable.

These are actual credentialed evaluations, not fixture assertions or
maintainer-reported results. Both owned mock registries were terminated and
waited for after testing. Persistent history workspaces and snapshots remain
available for inspection; canonical integration's temporary workspaces were
cleaned by its normal exit trap.

Local tools: Go `go1.26.10-X:nodwarf5 linux/amd64`; Snappy module v0.2.6;
Ampel module v1.4.2; Conftest module v0.68.2. Module identities were read
from the actual executable build metadata with `go version -m`, not inferred
from source or required as a runtime compatibility gate.

Evidence under the private acceptance directory:

- `preparation.json`: exact build/test commands, results, and tool metadata.
- `live-integration.json` and `logs/{released,main}-live-integration.log`:
  actual integration outcomes and verified retained artifact paths.
- `history-results.json`: post-second-scan evidence records and digests.
- `{released,main}-history/snapshots/evaluation-{A,B}.json`: log snapshots.
- `{released,main}-history/`: scan output, history-check logs, and retained
  workspace artifacts for independent revalidation.

The retained provider implementation remains a separate PR; these combined
acceptance results do not imply PR #238 has merged into PR #241.

## Pre-PR council and remaining human review

The initial plugin-backed command could not start because its required UF
tools were unavailable. After the maintainer updated the local review commands,
native execution completed before opening draft PR #241: Adversary, Architect,
Curator, Guard, and SRE each returned a successful assessment. The result was
APPROVE WITH ADVISORIES with no HIGH/CRITICAL findings. Every reviewer used
the verified inherited `complytime/gpt-6.1-sol#default` runtime identity.
Testing was skipped by scope policy; content-only personas were excluded.

The native record is retained at
`.uf/workflows/uf.review-council/945e07cd-7b74-4535-baf6-c12c1e78576f/state.json`.
It is native evidence, not a plugin-validated canonical artifact. Gaze's wait
request failed before a final report; its child was explicitly interrupted and
verified terminal with no pending work. Gaze was not counted as successful.

Four nonblocking documentation/workflow advisories were recorded. This
verification update resolves stale guidance about plugin-only prerequisites.
The other advisories concern separate, uncommitted native-command migration
work, which is not included or modified here. No implementation source changed
after council approval; this follow-up updates verification bookkeeping only.

Tasks 5.1-5.3 now have executed evidence. Required human maintainer reviews,
draft readiness/merge, and reconciliation of the older unpromoted capability
before archive remain separate follow-up actions, not claims of test coverage.

[released-job]: https://github.com/complytime/complytime-providers/actions/runs/43435940192/job/131921128668
[main-job]: https://github.com/complytime/complytime-providers/actions/runs/43435940192/job/131920957547

## Documentation impact

- `CHANGELOG.md`: records the canonical-harness CI correction.
- `AGENTS.md`: adds the offline execution-suite command, its prerequisites,
  and the workflow actionlint command; governance text is unchanged.
- `README.md`: links to released-host reproduction guidance.
- `docs/dev-testing-environment.md`: explains revision ownership,
  initial pin provenance, reviewed refreshes, local prebuilt dispatch,
  provenance inspection, diagnostics, and offline-versus-live limits.

Unrelated pre-existing untracked files were not staged or changed.
