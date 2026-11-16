---
description: "Review PR #$ARGUMENTS — alignment, security, and constitution compliance via Divisor council fan-out"
---
<!-- repo-owned fork of the uf v0.21.0 scaffold; verify with make verify-uf-fork -->

# Review Pull Request

You are a token-efficient code reviewer. The user will provide a PR number or you will auto-detect it from the current branch. Delegate deterministic checks to local tools and CI results first, then dispatch the PR through the full Divisor review council for multi-persona AI analysis.

<protect>

> **Session-resume guard**: If this session was resumed
> from compressed context, re-read this entire template
> before continuing. Do NOT infer step completion from
> compressed summaries. When in doubt, re-read — false
> re-reads are harmless; skipping steps due to stale
> context causes incomplete reviews or unauthorized
> actions.

Read **Native Progress and Recovery** and the **Shared Dispatch, Evidence,
and Finalization Protocol** in `.opencode/commands/uf.review-council.md` before
execution. Apply only those shared sections with command `uf.review-pr` and
its frozen PR input; do not invoke the council command's fix loop or posting
steps. Persist this command's completed steps and action receipts there.
Native todos are optional. No UF plugin tool or canonical plugin artifact is
required or produced. Missing shared instructions fail closed.

## Arguments

- **PR number** (optional): The pull request number to review (e.g., `42`). If omitted, the command auto-detects the open PR for the current branch.

**Argument parsing** (before any tool calls): Check the
user's message for a PR number argument. If present, set
`PR_NUMBER` to that value immediately. All subsequent steps
use `<PR_NUMBER>` — no auto-detection commands are needed
or permitted.

## Execution Steps

### 0. Prerequisites

Verify the `gh` CLI is available and authenticated before proceeding:

```bash
which gh
```

If `gh` is not found: **STOP** with error:
> "`gh` CLI is not installed. Install it from https://cli.github.com/ or via your package manager."

If `gh` is found, verify authentication:

```bash
gh auth status
```

If not authenticated: **STOP** with error:
> "`gh` is installed but not authenticated. Run `gh auth login` to authenticate."

#### Execution Mode Check

This command requires running local tools (build, test,
lint) as part of the review. Verify you can execute
commands by running a harmless probe:

```bash
echo "mode-check-ok"
```

If the probe cannot be executed (the agent runtime
returns a tool-access-denied error, or you are in plan
mode, read-only mode, or otherwise restricted from
running commands): **STOP** with message:

> "This review requires running local tools (build,
> test, lint) to verify the PR. I am currently in
> plan/read-only mode which prevents executing these
> checks. Switch to a mode that allows command
> execution (e.g., full mode / auto mode) and
> re-invoke `/uf.review-pr <N>`."

Do NOT proceed with a partial review that skips local
tool execution. The local tool results are the
foundation of the review — without them, AI-only
findings lack verification and the review does not
meet the command's quality standard.

### 0.1 Execution Path Preflight

Apply **Execution Path Preflight** from the Shared Execution Contract in
`.opencode/commands/uf.review-council.md` and print the banner it defines.

### 1. Resolve PR Number

Before any tool call, set `PR_NUMBER` from the command argument if one was given
and write it into the phase record. Every later step uses that value; do not
re-resolve it from the branch.

Validate the argument first: it MUST match ASCII `[0-9]+` and be between 1 and
999999. Reject empty, zero, signed, non-numeric, or repeated values with an
explicit error and stop. Never interpolate an unvalidated value into a shell
command or a path.

**If `PR_NUMBER` was already set from the argument**: skip
this step entirely. Do NOT run `gh pr view`,
`git branch --show-current`, or any branch/PR detection
commands.

**Only if no PR number was provided**: auto-detect from
the current branch:

```bash
gh pr view --json number --jq '.number'
```

If no open PR exists for the current branch: **STOP** with error:
> "No open PR found for branch '`<branch>`'. Provide a PR number: `/uf.review-pr 42`"

### 2. Fetch PR Metadata

Retrieve PR metadata — base/head refs and SHAs, changed files, diff stats:

```bash
gh pr view <PR_NUMBER> --json title,body,files,additions,deletions,baseRefName,baseRefOid,headRefName,headRefOid,labels,milestone,commits,reviewDecision,reviewRequests
```

Record all values. Resolve the immutable input context:

```
kind: pr
pr_number: <PR_NUMBER>
base_ref: <baseRefName>
base_sha: <baseRefOid>
head_ref: <headRefName>
head_sha: <headRefOid>
```

Validate every resolved SHA against `^[0-9a-f]{40}$`. A validation failure is `INCONCLUSIVE` and MUST stop before child dispatch.

Use only `base_sha...head_sha` for changed paths, inserted/deleted counts, review-context discovery, walkthroughs, prompts, and finalization. Never substitute the current checkout or later ref values.

### 3. Fetch CI Check Results

Retrieve the CI/CD check suite status for the PR:

```bash
gh pr checks <PR_NUMBER> --json name,state,description,link
```

Categorize each check as:
- **PASS**: Check succeeded
- **FAIL**: Check failed
- **PENDING**: Check still running
- **SKIPPED**: Check was skipped

If checks are still PENDING, inform the user and use
the **question tool** with options
`["Wait for checks to complete", "Proceed with
available results"]`.

**If all checks pass**: Record this and move to Step 3.5. No CI triage needed.

**If any checks fail**: Proceed to Step 3a for causality determination.

#### 3a. CI Failure Causality Determination

For each failing check, determine whether the failure is caused by the PR's changes or is a pre-existing issue on the base branch.

**Method**: Check if the same test/check also fails on the base branch:

```bash
gh api repos/{owner}/{repo}/commits/<base_sha>/check-runs \
  --jq --arg name "<FAILING_CHECK_NAME>" '.check_runs[] | select(.name == $name) | {name, conclusion}'
```

**Classification**:

| Base branch status | PR check status | Classification |
|--------------------|-----------------|----------------|
| Pass | Fail | **PR-caused** — the PR introduced the failure |
| Fail | Fail | **Pre-existing** — failure exists independently of the PR |
| No data | Fail | **Unknown** — treat as PR-caused (conservative) |

Record the classification for each failing check.

### 3.5. Diff Size Advisory

Using the `additions`, `deletions`, and `files` fields
from Step 2 metadata:

1. Calculate total diff lines: `additions + deletions`
2. Count changed files: length of `files` array

**If total diff lines > 2000 OR changed files > 50**:

Use the **question tool** with options
`["Review all files", "Focus on specific files"]`.

If the user selects "Focus on specific files", follow up
with the **question tool** (open-ended, no preset
options) to ask which files or directories to focus on.

Record the user's choice as `FILE_FOCUS_SCOPE`:
- "all" if reviewing all files
- A list of file paths/patterns if focusing on specific
  files

**If total diff lines <= 2000 AND changed files <= 50**:
Set `FILE_FOCUS_SCOPE = "all"` silently.

### 3.6. Fetch PR Diff

Fetch the full PR diff from the resolved immutable base and head SHAs:

Ensure both frozen commit objects are available, fetching exact immutable
objects without checkout if necessary; verify their hashes. Capture
`git diff <base_sha>...<head_sha>` and its per-file statistics to a private file
once, with a SHA256 recorded in the native phase record. Never use a later
`gh pr diff` response as evidence for previously fetched metadata. Pass this
same file and context to every reviewer. Apply any user-selected file focus
consistently and label the report as scoped, never as approval of unreviewed files.

**Large diff handling** (500+ lines): Use targeted reads of the frozen file.
Report lock/generated/binary/baseline files as such rather than silently
changing the immutable file inventory used for planning.

### 3.6.1 Tooling Scope Partition

Apply **Tooling Scope Partition** from the Shared Execution Contract in
`.opencode/commands/uf.review-council.md` using the frozen file inventory.

### 3.7. Pre-flight Checks (ci-aware, soft gate)

Load the `pre-flight` skill and run in `ci-aware` mode:

1. Invoke the `skill` tool with name `pre-flight` to
   load the shared pre-flight check instructions.

2. Execute the pre-flight skill's phases in order:
   a. CI Workflow Parsing — discover commands from `.github/workflows/`
   b. Local Tool Detection — check for config files and verify binary availability
   c. CI Coverage Matrix — build the matrix using the CI check results from Step 3. Apply ci-aware decision rules:
      - CI PASS → skip locally (CI already verified)
      - CI FAIL → skip locally (failure already captured in CI check analysis)
      - CI NONE → MUST run locally
      - No CI checks at all → MUST run ALL detected local tools
   d. Execution — run only tools marked "Yes" in the coverage matrix. Do NOT stop on first failure.

3. **Record results**: CI Coverage Matrix, Execution Results, Verdict.
   If tools pass, skip those categories in the AI review.
   If tools fail, include the failure output as context.

### 3.8. Discover Review Context

Load the `review-context` skill for spec artifact
discovery, issue linking, path classification, and
walkthrough generation:

1. Invoke the `skill` tool with name `review-context`
   to load the shared context discovery instructions.

2. Execute the skill's protocols in order:
   a. Protocol 1 (Spec Artifact Discovery) — locate
      the specification matching this PR using the branch
      name, PR description, and changed file list.
   b. Protocol 2 (Issue Linking) — parse the PR body
      for linked issues, validate, fetch, sanitize, and
      extract acceptance criteria.
   c. Protocol 3 (Path-Based Focus Heuristics) —
      retain the skill's focus heuristics for prompt
      emphasis. Pass raw diff statistics to the planner;
      do not reuse heuristic output as planner policy.
   d. Protocol 4 (Walkthrough Generation) — generate
      per-file change summaries from the diff.

3. **Record results**: Use the skill's Review Context
   output format (Specification, File Classification,
   Walkthrough). This context is used in child prompts
   and the final report.

4. **If the skill fails to load**: **STOP immediately.**
   Report the error as a CRITICAL finding. Do NOT start
   child dispatch. Run only the non-child planning and
   finalization portions of the shared protocol with a
   calculation cause and failing `INCONCLUSIVE`.

### 3.9. Load Convention Packs

Check if convention packs are available for enhanced review precision:

```bash
test -d .opencode/uf/packs && echo "PACKS=yes"
```

**If packs are available**:
1. Always read `.opencode/uf/packs/default.md` (language-agnostic rules)
2. Detect language and load the appropriate pack:
   - `go.mod` exists → read `.opencode/uf/packs/go.md`
   - `tsconfig.json` or `package.json` exists → read `.opencode/uf/packs/typescript.md`
3. Read corresponding `-custom.md` files if they exist (e.g., `go-custom.md`)
4. Read `.opencode/uf/packs/severity.md` if it exists
5. Do NOT load `content.md` or `content-custom.md` — these contain writing standards, not code quality rules

Pass loaded pack content to child agent prompts. Reference specific rule IDs (CS-001, AP-001, SC-001, etc.) in findings.

**If packs are NOT available**: proceed without them. No error or warning needed.

### 3.10. Fetch Existing Review State

Fetch existing PR reviews and inline comments to prevent duplicate findings and provide context.

#### Step 3.10-i. Fetch Reviews

```bash
gh api repos/{owner}/{repo}/pulls/<PR_NUMBER>/reviews \
  --jq '[.[] | {id: .id, user: .user.login, state: .state, body: .body, submitted_at: .submitted_at, commit_id: .commit_id}]'
```

#### Step 3.10-ii. Fetch Inline Comments

```bash
gh api repos/{owner}/{repo}/pulls/<PR_NUMBER>/comments \
  --jq '[.[] | {path: .path, line: .line, body: .body, user: .user.login, created_at: .created_at}]'
```

#### Step 3.10-iii. Identify Current User

```bash
gh api user --jq '.login'
```

#### Step 3.10-iv. Token Budget

Existing review comments passed to child prompts MUST be
capped at 3000 characters total. When exceeded: filter to
files changed in this PR, sort by `created_at` descending,
include until budget exhausted, truncate remainder with
"`N additional prior comments truncated for token budget`".

#### Step 3.10-v. Error Handling

If any `gh api` call returns 403, 404, 429, or times out:
log the error, skip the sub-step, proceed. All review state
data is additive context — its absence reduces only
deduplication accuracy.

---

### 3.11. Discover Divisor Agents

Before dispatch, discover which reviewer agents are available:

1. **Read the `.opencode/agents/` directory** using the Read tool to
   list all entries.

2. **Discover Divisor persona agents**: retain every regular file whose
   name matches `divisor-*.md`. Strip `.md` and sort the complete list.
   Filenames/frontmatter alone do not establish review eligibility.

3. **Validate existing policy** through the shared native planning protocol.
   Do not change the manifest, matrix, overrides, or persona definitions.

4. **Guard clause**: zero discovered agents produces `INCONCLUSIVE`.
   Do not start a child session. Persist the failed native review record.

---

### 4. Council Dispatch

Use the shared native protocol in `.opencode/commands/uf.review-council.md`
without invoking its command-level fix loop or posting steps.

#### 4.1 Plan and Run the PR Council

Use mode `code`, `full: false`, augmentation off, all discovered personas,
and raw path/addition/deletion counts from the frozen object diff. Bind the
plan to the exact PR base/head and diff digest, never the current checkout.
Display and validate the complete plan before starting any child. Preserve
all eligibility, scope, override, model, concurrency, cost, timeout,
cancellation, and permission safeguards from the shared protocol.

Pass the frozen diff, complete changed paths, applicable review focus,
pre-flight/CI coverage, loaded packs, Step 3.8 review context/walkthrough,
and Step 3.10 prior review state (within its existing budget) to the council.
Each child returns its structured assessment through native output.

#### 4.2 Consolidate and Persist

Apply the shared successful-run-only deduplication, compound severity, and
verdict rules, including `UNAVAILABLE` and `INCONCLUSIVE` for zero-success
outcomes. Preserve PR-caused CI failures as blocking findings and pre-existing
failures as informational context. Optional lessons use the shared parent-only
validation and Dewey skip rules.

Persist every terminal run and its actual output/provenance in this command's
native record. Revalidate frozen scope and persistence before accepting an
approval or offering posting. Persistence, unsafe live-child, policy, or stale
scope failures block automation even when a reviewer approved. Report the
native record path; canonical plugin output is not produced. Do not fabricate
plugin-validated artifacts or substitute a parent-only AI review.

**Artifact retention**: apply **Artifact Retention** from the Shared Execution
Contract in `.opencode/commands/uf.review-council.md`.

---

### 5. Output Format

Present the findings in this structured format:

```markdown
## PR Review: #<NUMBER> — <TITLE>

### Dispatch Provenance
| Run | Agent | Source | Requested | Resolved parent | Reported | Verdict |
|---|---|---|---|---|---|---|
| ... | ... | ... | model + variant | model + variant | model | APPROVE |

### CI Status
| Check | Status | Classification |
|-------|--------|----------------|
| <name> | PASS/FAIL | PR-caused / Pre-existing / N/A |

### Local Tool Results
<Table showing which tools ran, pass/fail status, and summary of failures if any>

### Walkthrough
| File | Change | Focus |
|------|--------|-------|
| `internal/gateway/provider.go` | Add token expiry tracking | security |

<For PRs with 30+ files, group by directory with counts:>
| Directory | Files | Summary | Focus |
|-----------|-------|---------|-------|
| `internal/gateway/` | 3 | Token refresh and provider detection | security |

### Linked Issues
<Only include if Step 3.8 found linked issues>
| Issue | Title | Criteria |
|-------|-------|----------|
| #38 | Export metrics to CSV | 3/4 COVERED |

### Summary
<1-2 sentence assessment of what the PR does and overall quality.>

### Alignment
- <Finding with severity>

### Security
- <Finding with severity>

### Constitution Compliance
- <Finding with severity>

### CI Failures (PR-caused)
- <Finding with severity — only if PR-caused failures exist>

### CI Failures (Pre-existing)
- <Description — only if pre-existing failures exist>
- Note: These failures exist independently of this PR. See fix-branch offer below.

### Verdict
**<APPROVE / APPROVE WITH ADVISORIES / REQUEST CHANGES / INCONCLUSIVE / UNAVAILABLE>**

<Brief justification. Pre-existing CI failures do NOT block the PR verdict.>

Native record: <path and persistence status>
Canonical plugin output: not produced (native execution)
```

Severity levels from `.opencode/uf/packs/severity.md` when loaded, otherwise:
- **CRITICAL**: Must be fixed before merge (security vulnerabilities, data loss risks)
- **HIGH**: Should be fixed before merge (spec violations, missing tests for critical paths, PR-caused CI failures)
- **MEDIUM**: Recommended to fix (code quality, minor compliance issues)
- **LOW**: Optional improvements (style, naming suggestions)

If no issues are found in a category, state "No issues found."

---

### 6. Offer Fix-Branch for Pre-existing CI Failures

If Step 3a identified any **pre-existing** CI failures, offer to create a fix branch:

```
I identified <N> pre-existing CI failure(s) that are NOT caused by this PR:
- <check name>: <brief description of failure>

These failures also occur on the base branch (<baseRefName>).
```

Use the **question tool** with options
`["Yes -- create fix branch", "No -- skip"]`.

**If the user selects "Yes -- create fix branch"**:

1. **Verify clean working tree**:
   ```bash
   git status --porcelain
   ```
   If the output is not empty: **STOP** branch creation with message:
   > "Working tree has uncommitted changes. Commit or stash them before creating a fix branch."
   Switch back to the PR branch and continue to Step 7.

2. **Check for branch name collision**:
   ```bash
   git branch --list "fix/pr-<PR_NUMBER>-<check-name>"
   ```
   If the branch already exists, inform the user:
   > "Branch `fix/pr-<PR_NUMBER>-<check-name>` already exists. Switch to it with `git checkout fix/pr-<PR_NUMBER>-<check-name>`, or delete it first."
   Switch back to the PR branch and continue to Step 7.

3. **Sanitize the check name** for branch-name safety:
   lowercase, replace spaces and special characters with
   hyphens, strip consecutive hyphens, remove characters
   outside `[a-z0-9._-]`, truncate to 50 characters.
   Example: `"Build (ubuntu/latest)"` → `build-ubuntu-latest`.

4. **Create a fix branch** from the base branch:
   ```bash
   git checkout <baseRefName>
   git checkout -b fix/pr-<PR_NUMBER>-<sanitized-check-name>
   ```

5. **Analyze and propose the fix**: Use the CI failure output and the failing file(s) to determine the minimal change needed.

   >>> MANDATORY GATE: HUMAN CONFIRMATION REQUIRED <<<

   **Session-resume guard**: If this session was resumed
   from compressed context, or if you cannot verify that
   the human explicitly confirmed the fix-branch commit
   in the current uncompressed conversation history,
   you MUST re-present the commit preview below and
   obtain fresh confirmation via the **question tool**
   before committing. Do NOT rely on confirmation
   recorded in compressed context. When in doubt,
   re-confirm — false re-confirmation is harmless;
   committing without consent is a violation.

   Before committing, show the user:

   > **Fix-branch commit preview:**
   >
   > ```
   > git diff --cached --stat
   > ```
   >
   > **Proposed commit message:**
   > ```
   > fix: resolve <failing-check> CI failure
   >
   > <Brief description>
   >
   > This failure was pre-existing on <baseRefName>
   > and unrelated to PR #<PR_NUMBER>.
   >
   > Assisted-by: <model>
   > ```

   Use the **question tool** with options
   `["Commit -- apply fix",
   "Edit commit message", "Abort -- discard changes"]`.

   - **"Commit -- apply fix"**: Proceed with the commit
     using the displayed message.
   - **"Edit commit message"**: Let the user modify the
     commit message, then re-confirm.
   - **"Abort -- discard changes"**: Discard staged
     changes and skip the fix branch. Switch back to
     the PR branch.

   **CRITICAL RULE**: NEVER commit on a fix branch
   without explicit human confirmation via the
   **question tool**.

   >>> END MANDATORY GATE <<<

6. **Commit with Conventional Commits format**:
   ```bash
   git add <changed-files>
   git commit -s -F <temp-commit-message-file>
   ```

7. **Report to the user**:
   ```
   Fix branch created: fix/pr-<PR_NUMBER>-<check-name>

   Changes:
   - <file>: <what changed>

   The branch is local. To review and push:
     git checkout fix/pr-<PR_NUMBER>-<check-name>
     git log -1
     git push -u origin fix/pr-<PR_NUMBER>-<check-name>
   ```

8. **Switch back** to the PR branch:
   ```bash
   git checkout <headRefName>
   ```

**Guardrails**:
- The fix MUST be scoped to the specific failing check — no unrelated changes
- The agent MUST NOT push to the remote or file a PR automatically
- If the fix is non-trivial (requires understanding business logic, architectural decisions, or modifying more than 3 files), inform the user instead of attempting a fix:
  ```
  The CI failure in <check> appears to require a non-trivial fix involving <description>.
  I recommend investigating this separately rather than proposing an automated fix.
  ```

---

### 7. Offer Verdict-aligned PR Review

After presenting the review, always offer to post the review as a
formal GitHub review on the PR. Use the **question tool** with
options `["Yes -- post as GitHub review", "No -- terminal
summary is sufficient"]`.

For `INCONCLUSIVE`, `UNAVAILABLE`, or partial file-focus review, report the
limitation and do not post an approving review or imply full-PR approval.
No-success/safety failures block posting entirely.

**If the user selects "Yes -- post as GitHub review"**:

#### 8a. Pre-posting Checks

**Duplicate review detection**: Check if a review from the current
user (from Step 3.10-iii) already exists in the review list (from
Step 3.10-i):

- If a prior review with the **same verdict** exists: use the
  **question tool** with options
  `["Yes -- post new review", "No -- skip posting"]`.
- If a prior review with a **different verdict** exists: use the
  **question tool** with options
  `["Yes -- override with <new_verdict>",
  "No -- keep existing <old_verdict>"]`.
- If no prior review exists: proceed silently.

**Stale review + CODEOWNER checks** (APPROVE verdicts only):

```bash
gh api repos/{owner}/{repo}/branches/<baseRefName>/protection \
  --jq '{dismiss_stale: .required_pull_request_reviews.dismiss_stale_reviews, require_codeowners: .required_pull_request_reviews.require_code_owner_reviews}'
```

If 404 or 403: skip both checks silently.

If `dismiss_stale` is true, display:
> "Warning: This repo dismisses stale reviews. If the author pushes
> any new commits after this APPROVE, it will be automatically
> invalidated and the PR will return to REVIEW_REQUIRED. You may
> need to re-run `/uf.review-pr` after final commits."

If `require_codeowners` is true, check for CODEOWNERS file at
`.github/CODEOWNERS`, `CODEOWNERS`, and `docs/CODEOWNERS` in order.
If found, display:
> "Warning: This repo requires code owner reviews. This APPROVE may
> not satisfy branch protection if this account is not listed in
> CODEOWNERS."

#### 8b. Inline Comment Preparation

**Load review voice packs**: Before composing inline comments or
the Step 8d review body, read `.opencode/uf/packs/review-voice.md`
and `.opencode/uf/packs/review-voice-custom.md`. If either file is
missing, proceed without it — no error or warning needed.

**Apply review voice**: The inline comment text composed in this
step and the review body prose composed in Step 8d MUST follow the
loaded voice rules (RV-NNN defaults, CR-NNN custom overrides where
present). Voice is style-only per the Style-Only Precedence clause
in `review-voice.md` (RV-004, RV-005): voice rules MUST NOT alter
severities, verdicts, finding counts, required footers, tone-tier
content requirements, reply-template required elements, top-15
inline comment selection, suggestion-block format, or body size
limits. Where a voice rule conflicts with a structural or content
requirement, the structural or content requirement prevails.
Terminal reports (Step 5) keep their fixed format; voice governs
GitHub-posted prose only.

For findings mapped to specific files and line ranges in the diff,
prepare inline comments:

1. Collect all file-specific findings from all personas
2. Sort by severity (CRITICAL > HIGH > MEDIUM > LOW)
3. Within the same severity tier, round-robin across personas in
   **alphabetical order** by persona name
4. Take the top 15
5. Overflow goes to the review body summary

Use suggestion blocks ONLY for literal code replacements. MUST NOT
use them for architectural recommendations, multi-file changes, or
removal of security controls.

#### 8c. Verdict Mapping and Human Confirmation

>>> MANDATORY GATE: HUMAN CONFIRMATION REQUIRED <<<

**Session-resume guard**: If this session was resumed from compressed
context, or if you cannot verify that the human explicitly confirmed
the review in the current uncompressed conversation history, you MUST
re-present the review content (verdict + all comments) and obtain
fresh confirmation via the **question tool** before posting.

Map the council verdict to the GitHub API event type:

| Council Verdict | GitHub Event |
|-----------------|-------------|
| APPROVE | `APPROVE` |
| REQUEST CHANGES | `REQUEST_CHANGES` |
| APPROVE WITH ADVISORIES | `COMMENT` |
| INCONCLUSIVE | Do not post |
| UNAVAILABLE | Do not post |

**VISIBILITY DIRECTIVE**: Before invoking the question tool, print
the full verdict context (verdict type, review body, and all inline
comments) as plain assistant output.

For APPROVE verdicts, use options:
`["Approve -- post review", "No -- skip posting",
"Edit comments first", "Change verdict"]`.

For REQUEST CHANGES or COMMENT verdicts, use options:
`["Yes -- post review", "No -- skip posting",
"Edit comments first", "Change verdict"]`.

**CRITICAL RULE**: NEVER post reviews without explicit human
confirmation via the **question tool**.

>>> END MANDATORY GATE <<<

#### 8d. Post Review

Revalidate PR base/head against the reviewed input immediately before posting.
If changed, stop and require fresh review. Construct a JSON payload containing
`event`, `body`, `comments`, and `commit_id` set to the frozen reviewed head SHA.
For an auto-detected PR or a local-to-PR posting path, require that the PR's
immutable reviewed scope is the same as the scope being approved; never use an
uncommitted local approval to approve a different remote head.
Write the payload to a temporary file and post:

```bash
gh api repos/{owner}/{repo}/pulls/<PR_NUMBER>/reviews \
  --method POST \
  --input <json-file>
```

The review body MUST include:
`_This review was generated by /uf.review-pr (AI-assisted)._`

Always write the JSON payload to a temporary file rather than
interpolating into shell arguments. Create it with mode `0600` from the start.
Remove the temporary file after posting, on ALL exit paths. Record the returned
review ID/URL before marking the native posting step complete.

**Graceful degradation**: If `gh api` returns HTTP 403, 404, or 422:
fall back to posting as `"event": "COMMENT"` with a note. If the
fallback also fails, inform the user their token lacks write
permissions.

</protect>
