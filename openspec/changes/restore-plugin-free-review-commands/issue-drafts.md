# Upstream drafts

Drafts for upstream repositories. Nothing here has been posted; filing them is
tracked by task 6.3 in `tasks.md`, which also records the resulting issue URLs.
They are part of this change's evidence so the measured friction informs the
projects that own the root cause, instead of being rediscovered.

- Draft 1: comment on [UF #962][962] (existing downstream bug).
- Draft 2: new UF issue for scaffold drift detection.
- Draft 3: new `org-infra` issue proposing a time-boxed Fullsend evaluation.
- Draft 4: new `org-infra` issue adding runtime-compatibility guidance to the
  provisioned `docs/AI_TOOLING.md`.

[962]: https://github.com/unbound-force/unbound-force/issues/962

---

## Draft 1 — comment on UF #962

> Re-verified on OpenCode v2.2.7 with the UF v0.21.0 scaffold. The failure is
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
> OpenCode v2 (about 870 changed lines). Those edits are the only working path.
> Until recently they lived uncommitted in a feature branch, precisely because a
> scaffold refresh would discard them; they are now committed behind a fork
> marker, a pinned version, and a digest manifest. That workaround is itself the
> argument for a supported adaptation path.
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
> Several organization repositories depend on UF's local `review-council`,
> `review-pr`, and `address-feedback` commands. Those commands require UF plugins
> that do not load on OpenCode v2, and the v2 migration (#714, #797) is
> unreleased. Downstream repositories are currently maintaining local forks.
>
> [Fullsend](https://github.com/fullsend-ai/fullsend) is an Apache-2.0,
> self-hosted agentic SDLC platform that runs purpose-built triage, review, fix,
> and retro agents against GitHub and GitLab repositories. It is a different
> layer from UF: post-PR bots in CI, not local pre-PR commands.
>
> ## Why evaluate rather than adopt
>
> - It cannot replace the local pre-PR gate as written: the repository
>   constitution requires a pre-PR council, and Fullsend reviews after the PR
>   exists. It would be additive, or the gate would have to be redefined.
> - Its OpenCode runtime is experimental and limited to read-only agents
>   (triage, prioritize) until a security-hook adapter lands. Default runtimes are
>   Claude Code, pi, and codex.
> - Adoption needs infrastructure: GitHub App, an OIDC token mint
>   (GCP Cloud Function or Cloudflare Worker), an OpenShell sandbox, and an
>   inference gateway. That is an ownership decision, not a tool switch.
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
> guidance points at commands most consumers cannot run. Either annotate the
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
