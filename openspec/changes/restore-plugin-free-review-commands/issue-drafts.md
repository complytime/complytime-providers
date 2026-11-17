# Upstream drafts

Historical drafts for upstream repositories, retained as change context.
The maintainer has managed the issue disposition; these drafts are not a
posting queue or authorization for further external actions. The UF #962
comment was declined and not posted. Three issues were created prematurely
without preview: [UF #1176](https://github.com/unbound-force/unbound-force/issues/1176),
[org-infra #747](https://github.com/complytime/org-infra/issues/747), and
[org-infra #746](https://github.com/complytime/org-infra/issues/746).
Do not infer their current state from this record. Future issue/comment
creation, editing, or closure requires showing the exact proposed content or
action and receiving explicit maintainer approval. No more posting is needed
to complete this PR.

Reconciled for the plugin-first implementation revision on 2026-11-17:
the session counts and plugin-load observations below are historical measurements,
not rerun statistics. The repository now has contract-gated plugin instructions,
bounded native fallback, and controlled selection/recovery fixtures, but no new
end-to-end V1/V2 compatibility claim. The pinned feedback response-contract gap
is documented in `docs/review-plugin-contracts.md`; the maintainer elected not
to modify upstream plugin sources here. Keep filing separate from implementation
acceptance and preserve these qualifications in any posted body.

- Draft 1: comment on [UF #962][962] (existing downstream bug).
- Draft 2: new UF issue for scaffold drift detection.
- Draft 3: new `org-infra` issue proposing a time-boxed Fullsend evaluation.
- Draft 4: new `org-infra` issue adding runtime-compatibility guidance to the
  provisioned `docs/AI_TOOLING.md`.

[962]: https://github.com/unbound-force/unbound-force/issues/962

---

## Draft 1 — comment on UF #962

> Historical downstream measurement on OpenCode v2.2.7 with the UF v0.21.0
> scaffold (not a claim about current upstream releases): the failure is
> broader than the missing workflow tools in the original report: all four
> scaffolded plugins fail to load, so no plugin tool exists at all.
>
> Log evidence:
>
> ```text
> WARN message="failed to load plugin"
>   target=.opencode/plugins/{invoke-agent,review-dispatch,uf-finale,uf-workflow}
>   cause="PluginModule.LoadError: Plugin must export a default definition with an
>   id and an effect or setup function."
> ```
>
> 247 such warnings in the local log. `opencode.json` still registers the four
> plugins as individual `index.ts` paths, and v2 also warns that a configured
> plugin path must be a directory.
>
> Downstream impact observed over 29 sessions since updating to v0.21.0:
>
> - 0 calls to `plan_review_dispatch`, `dispatch_review_runs`,
>   `consolidate_dispatch`, `finalize_review_dispatch`, `capture_review_diff`,
>   `fetch_pr_review_context`, `submit_review_findings`, or
>   `get_progress`/`checkpoint`. 52 reviewer child sessions were started by
>   hand-written shell/API supervisors instead.
> - `/uf.review-pr` produced `INCONCLUSIVE`, correctly naming the missing policy
>   tools, and then completed a full review without a council because the
>   contributor asked it to continue. The template has no way to make that
>   degradation visible in the output.
> - `/uf.address-feedback` is new in v0.21.0 and assumes plugin checkpoints,
>   `fetch_pr_review_context`, and native Tier-2 dispatch. On v2 it stalls on
>   `.uf/` directory-mode preflight and requires a handoff restart; the same two
>   feedback items were assessed twice across two sessions.
>
> Asks:
>
> 1. A timeline note on #797/#714 for the first V2-compatible release.
> 2. An interim guard in the scaffolded commands: detect absent plugin tools,
>    print one banner naming the path and unenforced safeguards, and never fall
>    back silently.
> 3. A `uf doctor` check that reports plugin load status per runtime version,
>    since the current failure is only visible in the OpenCode log.
>
> Happy to test a prerelease against this repository; the saved review and
> feedback fixtures here make a good downstream regression corpus.
>
> Downstream implementation follow-up (2026-11-17): the three command templates
> now prefer a complete compatible plugin contract before bounded native
> execution. Controlled decision fixtures are not real-runtime compatibility
> evidence. Source inspection of v0.21.0 also found that feedback prompt building
> injects the code-review response contract, while consolidation projects
> code-review verdicts into feedback recommendations. Please verify the
> item-specific feedback contract in a compatible release; registration alone
> does not establish it.

---

## Draft 2 — new UF issue

**Title:** `uf init offers no drift detection for locally adapted commands`

**Body:**

> ## Summary
>
> When a downstream repository must adapt a scaffolded command (for example
> because the plugin layer cannot load on its runtime), the adaptation is
> invisible to `uf`. A later `uf init --force` silently replaces it, and there is
> no supported way to know that a file diverged.
>
> ## Downstream case
>
> `complytime-providers` adapted `.opencode/commands/uf.review-council.md`,
> `uf.review-pr.md`, and `uf.address-feedback.md` to run without the plugins on
> the affected OpenCode v2 runtime. The initial fork supplied a native path.
> Until recently they lived uncommitted in a feature branch, precisely because a
> scaffold refresh would discard them; they are now committed behind a fork
> marker, a pinned version, and a digest manifest. That workaround is itself the
> argument for a supported adaptation path.
>
> The subsequent plugin-first revision retains those drift checks and adds
> static path-isolation fixtures; it does not establish end-to-end compatibility
> for either revised backend. Working compatible plugins should remain preferred.
>
> ## Proposal
>
> - `uf init --check` reports files that differ from the loaded scaffold and
>   prints the local diff summary.
> - An optional manifest records expected digests per scaffolded file, so a
>   downstream repository can fail a local target when a file changes
>   unexpectedly.
> - Document the supported adaptation workflow: fork marker, pinned version,
>   reconciliation procedure, and how to drop the fork after an upstream upgrade.
>
> ## Why it matters
>
> Without this, every downstream adaptation is a silent liability. In this
> repository the alternative was a 26k-line UF-managed tree with no way to tell
> which files were still upstream-owned.

---

## Draft 3 — new `org-infra` issue

**Title:** `Evaluate Fullsend as an org-level review/fix option (time-boxed)`

**Body:**

> ## Context
>
> Organization guidance names UF's local review workflows. In the measured
> `complytime-providers` case, the pinned UF v0.21.0 plugins fail to load on
> OpenCode v2.2.7/v2.3.0. That repository maintains a local command fork.
> Verify the current upstream migration/release status (#714, #797) before
> deciding whether another platform would address an ongoing gap.
>
> [Fullsend](https://github.com/fullsend-ai/fullsend) is licensed under Apache-2.0 and is a
> candidate for a time-boxed evaluation. Verify its current deployment model,
> supported hosts/runtimes, review/fix workflows and security controls against
> an explicitly selected release. Do not assume it replaces UF's local pre-PR
> workflow or treat repository marketing as compatibility evidence.
>
> ## Why evaluate rather than adopt
>
> - Existing repository rules require a local pre-PR council. Establish whether
>   the candidate supports that stage; post-PR automation alone would be additive,
>   not a replacement for the existing gate.
> - Confirm the chosen release's OpenCode support and read/write security
>   boundaries before attempting a fix workflow.
> - Inventory deployment ownership, GitHub App permissions, credential/OIDC
>   requirements, sandboxing and inference infrastructure rather than assuming
>   adoption is only a local tool switch.
>
> ## Proposed evaluation
>
> **Phase 0 (desk, ~1 day):** confirm the runtime and prerequisite matrix for the
> version to be tested; confirm the security posture (sandbox egress, prompt
> injection handling, credential model) and the license/governance fit.
>
> **Phase 1 (PoC, capped at one low-risk repository and three historical PRs):**
> run the `review` and `fix` agents against closed PRs whose human review is
> already recorded. Compare findings with the existing Divisor council output for
> the same commits.
>
> **Decision criteria:** finding precision and overlap, false-positive rate,
> latency per PR, reported cost per PR, maintainer review burden, and
> infrastructure ownership cost.
>
> **Kill switch:** no change to any repository's review gate during the
> evaluation; reverting means not installing the App.
>
> ## Documentation follow-up
>
> `org-infra/docs/AI_TOOLING.md` names `/uf.review-pr` as the review command for
> every provisioned repository. Until a V2-compatible UF release exists, that
> guidance needs to distinguish affected runtimes from compatible consumers.
> Either annotate the
> runtime requirement or publish the interim native-path guidance.

---

## Draft 4 — new `org-infra` issue

**Title:** `docs(AI_TOOLING): add runtime-compatibility guidance`

**Body:**

> ## Problem
>
> Every repository provisions `org-infra/docs/AI_TOOLING.md`, and it names
> `/uf.review-pr` as the review command. Command discovery does not establish
> runtime compatibility, and OpenCode V1 plugins do not run on V2 even when their
> Markdown commands are still discoverable. Consumers therefore read guidance for
> a command they cannot run, and each one either improvises or keeps an
> uncommitted local fork.
>
> ## Proposal
>
> Add a "Provisioning and Runtime Compatibility" section to the canonical guide:
>
> - State that the guide is provisioned and that local edits are replaced on sync.
> - Require inspecting a command's complete dependency closure (shared skills,
>   child prompts, resume paths, post-fix gates) before relying on it.
> - Require recording the exact runtime versions exercised and reporting untested
>   versions as unverified.
> - When adapting plugin-backed workflows: preserve reviewer eligibility, model
>   policy, limits, verdicts, quality checks, immutable scope, and confirmation
>   gates; report missing required capabilities instead of bypassing them; treat
>   prototype evidence as distinct from compatibility claims; keep disposable
>   fixtures and separate executable paths.
> - Point readers at the OpenCode V1 migration guide, API reference, and CLI guide.
>
> ## Why not fix it downstream
>
> A downstream copy is overwritten by the next sync and diverges per repository.
> The guidance is generic and belongs in the canonical source.
