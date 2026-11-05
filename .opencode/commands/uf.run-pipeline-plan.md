---
description: >
  Execute a prioritized work plan through the full Unbound
  Force pipeline (/opsx-propose -> /uf.unleash -> /uf.finale)
  with fresh subagent isolation per phase, human gate relay,
  and per-item progress tracking.
---
<!-- scaffolded by uf v0.21.0 -->

# Command: /uf.run-pipeline-plan

## User Input

```text
$ARGUMENTS
```

## Description

Orchestrate a prioritized work plan through the full
Unbound Force pipeline. For each work item, spawns three
fresh `general` subagents in sequence: `/opsx-propose`
(spec artifacts), `/uf.unleash` (implementation + review),
`/uf.finale` (commit + PR). Human confirmation gates are
relayed through the orchestrator's `question` tool.
Progress is tracked via `todowrite`.

Supports two input modes: a plan file path or space-
separated GitHub issue references (`#NNN` or URLs).

## Usage

```
/uf.run-pipeline-plan sprint-42.md
/uf.run-pipeline-plan #123 #456 #789
/uf.run-pipeline-plan https://github.com/unbound-force/unbound-force/issues/42 #99
```

## Instructions

<protect>

> **SESSION-RESUME GUARD**: When resuming after context
> compression or a session restart, re-read this entire
> template before continuing. Compressed context summaries
> MUST NOT determine item completion. Only `todowrite`
> entries and disk state (branches, spec artifacts, PRs)
> are authoritative for resumability.

### Execution Checklist

Maintain this checklist using the `todowrite` tool: one
entry per work item. Entry completion follows the rule
in the Tracking section.

### Phase 0 — Input Resolution and Confirmation

Phase 0 runs in the orchestrator context (no subagents).

1. **Detect input type**: Determine whether `$ARGUMENTS`
   is an existing file path.
   - File path: PROCEED to the Parse Plan File step.
   - Otherwise: PROCEED to the Resolve Issue Metadata
     step. Treat the input as space-separated GitHub
     issue references (`#NNN` or full GitHub URLs).

2. **Resolve issue metadata** (issue-ref input only):
   For each issue reference, run:

   ```bash
   gh issue view <N> --json title,body,number
   ```

   **On failure**: STOP and report the `gh` error.
   From each issue, record a kebab-case change name
   (from the title) and a one-line description (from
   the body).

3. **Parse plan file** (file input only): Read the plan
   file line by line. Each numbered item MUST carry the
   fields defined in the Plan File Contract.

4. **Flag non-mechanical items**: ASK the human to
   resolve every item tagged `[INVESTIGATION]` or
   `[POLICY]`.

   | Resolution | Meaning |
   |---|---|
   | remove | Drop the item from this run |
   | convert to mechanical | Rewrite the item as a mechanical work item |
   | defer | Postpone the item to a later plan |

   **On failure**: none (the question tool blocks until
   answered). The pipeline MUST NOT auto-execute flagged
   items.

5. **Present work-item table**: ASK the human to confirm
   the full ordered work-item list. Include issue refs,
   change names, and descriptions.
   - Plan-file input: group items as the plan file
     defines.
   - Bare issue list: ASK how to group issues into work
     items (default: one PR per issue).

   **On failure**: none (the question tool blocks until
   answered). The pipeline MUST NOT PROCEED before the
   human confirms.

### Plan File Contract

A plan file is a numbered markdown list. Each item
carries:

```
1. #42 Change: add-run-plan-command — Create /uf.run-pipeline-plan slash command
2. #99 Change: fix-scaffold-drift — Sync embedded asset copies
3. #55 [INVESTIGATION] Investigate CI flakiness in TestEmbeddedAssets
```

- Issue refs: `#NNN` (one or more per item)
- Change name: `Change: <kebab-case-name>`
- Description: text after the `—` separator (optional)
- Non-mechanical tag: `[INVESTIGATION]` or `[POLICY]`

### Per-Item Pipeline Protocol

For each confirmed work item, execute three pipeline
phases sequentially. Each phase runs in a fresh `general`
subagent:

1. **Subagent 1** (`/opsx-propose`): DELEGATE with the
   change name and description as input. Expected
   artifacts: proposal.md, design.md, specs/, tasks.md
   on an `opsx/<change-name>` branch.

2. **Subagent 2** (`/uf.unleash`, OpenSpec mode):
   DELEGATE. Expected outcome: the `/uf.unleash`
   pipeline completes — implement, spec review, code
   review, retrospective, demo. The Code Review step of
   `/uf.unleash` MUST NOT be skipped.

3. **Subagent 3** (`/uf.finale`): DELEGATE. Expected
   outcome: changes committed and pushed; a PR created
   with `Closes #N` in the commit message and PR body.
   The PR targets the parent `unbound-force/unbound-force`
   repo with head `jflowers:<branch>`. The PR stays open
   for human review.

**Disk handoff verification**: Before spawning each next
subagent, VERIFY the previous subagent's outputs on disk:

| After | VERIFY on disk |
|---|---|
| `/opsx-propose` | Spec artifacts exist (proposal.md, design.md, specs/, tasks.md); the `opsx/<name>` branch is checked out |
| `/uf.unleash` | Implementation files exist and tests pass |
| `/uf.finale` | A PR URL is returned |

**On failure**: STOP and report the missing output.

**Clean tree between items**: After each item completes
(PR created), VERIFY the return to `main` with a clean
working tree. `git status --short` MUST be empty before
the next item starts.

**On failure**: STOP and report the dirty tree.

### Gate Relay Protocol

When a subagent reaches a mandatory human confirmation
gate, the subagent RELAYs a structured value to the
orchestrator:

```
GATE:<name>
<exact question text>
<full proposed text / findings verbatim>
<options list>
```

The subagent stays resumable via its `task_id`. The
orchestrator then:

1. **Action**: Receive the `GATE:` return value.
2. **Action**: ASK the question through the `question`
   tool. Pass the exact question, full text, and
   options. **On failure**: none (the question tool
   blocks until answered).
3. **Action**: RESUME the same subagent via `task_id`
   with the human's answer.

Subagents MUST NOT invoke the `question` tool directly.
All human interaction flows through the orchestrator.
This prevents subagents from racing for user input and
ensures consistent gate formatting.

### Guardrails

These rules are non-negotiable:

- **Scaffold drift sync**: Any file under
  `.opencode/commands/` or `openspec/` that changes
  MUST have its `internal/scaffold/assets/...` copy
  synced byte-identical, or `TestEmbeddedAssets_MatchSource`
  fails.
- **Explicit staging**: Stage files explicitly by path.
  `git add -A` and `git add .` are forbidden.
- **Stray file removal**: Remove any untracked
  `internal/scaffold/AGENTS.md` before staging. Never
  commit it.
- **No force-push**: `git push --force` is forbidden.
  Only `git push --force-with-lease` is allowed.
- **PR targeting**: PRs MUST target the parent
  `unbound-force/unbound-force` repo with head
  `jflowers:<branch>`.
- **Closes linkage**: The commit message and PR body
  MUST include `Closes #N` for each issue in the work
  item.
- **No auto-merge**: The PR MUST NOT be merged
  automatically. It stays open for human review.

### Tracking

Maintain `todowrite` with one entry per work item:

```
- [ ] Item 1: #42 add-run-plan-command
- [ ] Item 2: #99 fix-scaffold-drift
- [ ] Item 3: #55 [INVESTIGATION] CI flakiness
```

Mark an entry complete (`[x]`) only after both hold:
1. The item's PR has been created
2. CI checks on the PR are green

Entries MUST NOT be marked complete prematurely. The
`todowrite` tracker is the source of truth for item
completion, not the orchestrator's context.

</protect>
