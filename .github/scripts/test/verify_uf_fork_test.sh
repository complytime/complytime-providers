#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Verifies `make verify-uf-fork` and `make clean-uf-workflows` guard behaviour
# against a temporary copy of the Makefile, manifest, and fork-owned commands.
# Requires GNU coreutils/findutils semantics (`find -newermt`, `touch -d`).

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
readonly REPOSITORY_ROOT
WORK_DIR=$(mktemp -d)
readonly WORK_DIR
readonly FILES=(
    .opencode/commands/uf.address-feedback.md
    .opencode/commands/uf.review-council.md
    .opencode/commands/uf.review-pr.md
)

cleanup() {
    rm -rf "$WORK_DIR"
}
trap cleanup EXIT

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    exit 1
}

copy_fixture() {
    cp "$REPOSITORY_ROOT/Makefile" "$WORK_DIR/Makefile"
    mkdir -p "$WORK_DIR/.uf" "$WORK_DIR/.opencode/commands"
    cp "$REPOSITORY_ROOT/.uf/fork-manifest.sha256" "$WORK_DIR/.uf/fork-manifest.sha256"
    local file
    for file in "${FILES[@]}"; do
        cp "$REPOSITORY_ROOT/$file" "$WORK_DIR/$file"
    done
    if [ ! -d "$WORK_DIR/.git" ]; then
        git init -q "$WORK_DIR"
    fi
}

run_make() {
    make --no-print-directory -C "$WORK_DIR" "$@"
}

# 1. The committed manifest matches the committed command files.
copy_fixture
run_make verify-uf-fork >/dev/null 2>&1 || fail "verify-uf-fork rejected the committed files"

# 2. A one-byte drift is detected and names the offending file.
printf '\n' >>"$WORK_DIR/${FILES[0]}"
if run_make verify-uf-fork >"$WORK_DIR/drift.log" 2>&1; then
    fail "verify-uf-fork accepted a modified command file"
fi
grep -Fq "${FILES[0]}" "$WORK_DIR/drift.log" || fail "drift output did not name the changed file"

# 3. The manifest update target restores a passing state.
run_make update-uf-fork-manifest >/dev/null 2>&1 || fail "update-uf-fork-manifest failed"
run_make verify-uf-fork >/dev/null 2>&1 || fail "verify-uf-fork failed after manifest update"

# 4. A missing manifest is reported, and the repair target recreates it.
rm "$WORK_DIR/.uf/fork-manifest.sha256"
if run_make verify-uf-fork >"$WORK_DIR/missing.log" 2>&1; then
    fail "verify-uf-fork passed without a manifest"
fi
grep -Fq 'fork-manifest.sha256 is missing' "$WORK_DIR/missing.log" ||
    fail "missing-manifest error was not actionable"
run_make update-uf-fork-manifest >/dev/null 2>&1 ||
    fail "update-uf-fork-manifest could not recreate a deleted manifest"
run_make verify-uf-fork >/dev/null 2>&1 || fail "verify-uf-fork failed after repair"

# 5. Each reintroduced plugin tool fails verification after the manifest update.
for tool in get_progress checkpoint dispatch_agent_run; do
    printf "\nCall \`%s\` with command scope.\n" "$tool" >>"$WORK_DIR/${FILES[2]}"
    run_make update-uf-fork-manifest >/dev/null 2>&1
    if run_make verify-uf-fork >"$WORK_DIR/plugin.log" 2>&1; then
        fail "verify-uf-fork accepted the plugin tool $tool"
    fi
    grep -Fq 'reference UF plugin tools' "$WORK_DIR/plugin.log" ||
        fail "plugin-tool rejection was not explained for $tool"
    copy_fixture
done

# 5b. The shasum fallback regenerates and verifies the manifest.
if command -v shasum >/dev/null 2>&1; then
    run_make update-uf-fork-manifest SHA256_SUM='shasum -a 256' >/dev/null 2>&1 ||
        fail "manifest update failed with the shasum fallback"
    run_make verify-uf-fork SHA256_CHECK='shasum -a 256 -c' >/dev/null 2>&1 ||
        fail "verification failed with the shasum fallback"
    copy_fixture
fi

# 6. Pruning rejects a non-numeric retention window.
if run_make clean-uf-workflows UF_WORKFLOW_RETENTION_DAYS='0; echo injected' \
    >"$WORK_DIR/days.log" 2>&1; then
    fail "clean-uf-workflows accepted a non-numeric retention window"
fi
grep -Fq 'must be a positive integer' "$WORK_DIR/days.log" ||
    fail "non-numeric retention error was not explained"

# 7. Pruning rejects zero and out-of-range windows.
if run_make clean-uf-workflows UF_WORKFLOW_RETENTION_DAYS=0 >/dev/null 2>&1; then
    fail "clean-uf-workflows accepted a zero-day window"
fi
if run_make clean-uf-workflows UF_WORKFLOW_RETENTION_DAYS=4000 >/dev/null 2>&1; then
    fail "clean-uf-workflows accepted an unbounded window"
fi

# Fixture: stale and fresh records plus a feedback posting receipt.
mkdir -p "$WORK_DIR/.uf/workflows/uf.review-council/stale" \
    "$WORK_DIR/.uf/workflows/uf.review-council/fresh" \
    "$WORK_DIR/.uf/feedback/pr-7"
printf '{"step":"done"}\n' >"$WORK_DIR/.uf/workflows/uf.review-council/stale/state.json"
printf '{"step":"done"}\n' >"$WORK_DIR/.uf/workflows/uf.review-council/fresh/state.json"
printf '{"posted":true}\n' >"$WORK_DIR/.uf/feedback/pr-7/receipts.json"
touch -d '20 days ago' "$WORK_DIR/.uf/workflows/uf.review-council/stale/state.json"
touch -d '40 days ago' "$WORK_DIR/.uf/feedback/pr-7/receipts.json"

# 8. The default is list-only and deletes nothing.
run_make clean-uf-workflows >"$WORK_DIR/list.log" 2>&1 || fail "clean-uf-workflows listing failed"
grep -Fq 'uf.review-council/stale' "$WORK_DIR/list.log" || fail "listing omitted the stale record"
grep -Fq 'set PRUNE=1' "$WORK_DIR/list.log" || fail "listing did not explain how to delete"
[ -d "$WORK_DIR/.uf/workflows/uf.review-council/stale" ] || fail "default run deleted a record"

# 9. PRUNE=1 removes only the stale record and keeps feedback receipts.
run_make clean-uf-workflows PRUNE=1 >/dev/null 2>&1 || fail "clean-uf-workflows PRUNE=1 failed"
[ ! -d "$WORK_DIR/.uf/workflows/uf.review-council/stale" ] || fail "stale record was not pruned"
[ -d "$WORK_DIR/.uf/workflows/uf.review-council/fresh" ] || fail "fresh record was pruned"
[ -f "$WORK_DIR/.uf/feedback/pr-7/receipts.json" ] || fail "feedback posting receipt was pruned"

# 10. Retention boundary: 13 days kept, 14 and 15 days pruned.
for age in 13 14 15; do
    mkdir -p "$WORK_DIR/.uf/workflows/uf.review-council/age-$age"
    printf '{"step":"done"}\n' >"$WORK_DIR/.uf/workflows/uf.review-council/age-$age/state.json"
    touch -d "$age days ago" "$WORK_DIR/.uf/workflows/uf.review-council/age-$age/state.json"
done
run_make clean-uf-workflows PRUNE=1 >/dev/null 2>&1 || fail "boundary prune failed"
[ -d "$WORK_DIR/.uf/workflows/uf.review-council/age-13" ] || fail "13-day-old record was pruned"
[ ! -d "$WORK_DIR/.uf/workflows/uf.review-council/age-14" ] || fail "14-day-old record was kept"
[ ! -d "$WORK_DIR/.uf/workflows/uf.review-council/age-15" ] || fail "15-day-old record was kept"

# 11. A recent nested write protects an otherwise old record.
mkdir -p "$WORK_DIR/.uf/workflows/uf.review-council/resumed"
printf '{"step":"done"}\n' >"$WORK_DIR/.uf/workflows/uf.review-council/resumed/state.json"
touch -d '30 days ago' "$WORK_DIR/.uf/workflows/uf.review-council/resumed/state.json"
printf '{"step":"assess"}\n' >"$WORK_DIR/.uf/workflows/uf.review-council/resumed/assessment.json"
run_make clean-uf-workflows PRUNE=1 >/dev/null 2>&1 || fail "resumed-record prune failed"
[ -d "$WORK_DIR/.uf/workflows/uf.review-council/resumed" ] ||
    fail "a recently written record was pruned"

# 12. An empty record directory is retained, not classified stale.
mkdir -p "$WORK_DIR/.uf/workflows/uf.review-council/empty"
run_make clean-uf-workflows PRUNE=1 >/dev/null 2>&1 || fail "empty-record prune failed"
[ -d "$WORK_DIR/.uf/workflows/uf.review-council/empty" ] || fail "empty record directory was pruned"

# 13. A missing workflow directory is reported and is not an error.
mv "$WORK_DIR/.uf/workflows" "$WORK_DIR/.uf/workflows.off"
run_make clean-uf-workflows >"$WORK_DIR/none.log" 2>&1 ||
    fail "clean-uf-workflows failed when .uf/workflows is absent"
grep -Fq 'no .uf/workflows directory' "$WORK_DIR/none.log" ||
    fail "missing workflow directory was not reported"
mv "$WORK_DIR/.uf/workflows.off" "$WORK_DIR/.uf/workflows"

# 14. Pruning refuses a tree containing tracked files.
(
    cd "$WORK_DIR"
    git init -q
    git add -f Makefile >/dev/null
    git -c user.email=test@example.com -c user.name=test commit -qm fixture
    git add -f .uf/workflows/uf.review-council/fresh/state.json
)
if run_make clean-uf-workflows PRUNE=1 >"$WORK_DIR/tracked.log" 2>&1; then
    fail "clean-uf-workflows pruned a tree with tracked files"
fi
grep -Fq 'contains tracked files' "$WORK_DIR/tracked.log" ||
    fail "tracked-file refusal was not explained"

printf 'verify-uf-fork and clean-uf-workflows guard behaviour: PASS\n'
