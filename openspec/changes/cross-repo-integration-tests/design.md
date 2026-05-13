## Context

The complyctl repository merged the `cross-repo-integration-tests` change, which
established:

- `tests/cross-repo/cross_repo_integration_test.sh` — the canonical integration test
  script accepting `PROVIDERS_BIN_DIR`, `COMPLYCTL_BIN_DIR`, and `GITHUB_TOKEN`.
- `tests/cross-repo/testdata/` — minimal Gemara content and a granular policy fixture
  for the Ampel `block-force-push` rule.
- `cmd/mock-oci-registry/` — seeded with the `policies/test-branch-protection` policy.
- `.github/workflows/ci_cross_repo_integration.yml` — validates complyctl PRs against
  `complytime-providers@main`.

This change adds only the mirrored workflow to `complytime-providers`. All test logic
and content remain in complyctl.

## Goals / Non-Goals

**Goals:**

- Validate on every `complytime-providers` PR that the Ampel provider binary built from
  the PR branch can interoperate with `complyctl@main` through the full
  Describe / Generate / Scan pipeline.
- Reuse the test script, fixtures, and mock registry from complyctl without duplication.

**Non-Goals:**

- Adding test logic or fixtures to `complytime-providers` — all test content lives in
  complyctl.
- Modifying the test script or Gemara content — those are owned by complyctl.
- OpenSCAP provider integration — deferred to a future change.

## Decisions

### D1: Reuse complyctl's test script verbatim

The workflow checks out `complyctl@main` into `_complyctl/` and runs the script from
there. No copy of the script is kept in `complytime-providers`.

**Rationale**: Keeping the script in one place (complyctl, the contract owner) prevents
drift. If the test script needs updating due to a complyctl change, it is updated there
and the providers workflow picks it up automatically on the next run.

### D2: Provider binary from PR branch, complyctl from main

The providers CI workflow builds `complyctl-provider-ampel` from the PR branch and
fetches `complyctl@main` for everything else (complyctl binary, mock registry, test
script, fixtures).

**Rationale**: Symmetric with the complyctl workflow (D2 in complyctl design). The repo
under PR contributes the binary under test; the other repo contributes the stable
baseline.

### D3: Same snappy/ampel action SHA pins as complyctl workflow

The `carabiner-dev/actions/install/snappy` and `carabiner-dev/actions/install/ampel`
actions are pinned to the same commit SHA used in the complyctl workflow.

**Rationale**: Consistent tool versions between the two workflows ensures that a test
failure is attributable to a code change, not a tool version difference.

## Risks / Trade-offs

**[Dependency on complyctl main]** → This workflow builds complyctl from `main`. If
complyctl's main branch is broken, providers PRs will also fail. Mitigation: complyctl's
own CI gates merges to main, so main should be stable.

**[Script API changes]** → If the complyctl test script changes its interface (env var
names, exit codes), this workflow may need updating. Mitigation: the script interface
is minimal (`PROVIDERS_BIN_DIR`, `COMPLYCTL_BIN_DIR`, `GITHUB_TOKEN`) and changes to
it will be visible in complyctl PRs.

## Open Questions

- Should the cross-repo integration test be a required status check blocking PR merge
  in `complytime-providers`? Recommended: yes, symmetric with complyctl.
