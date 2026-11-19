#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Exercises the same pure decision helper consumed by all three commands.
# These fixtures do not establish actual runtime or command compatibility.
set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
readonly REPOSITORY_ROOT
readonly HELPER="$REPOSITORY_ROOT/.github/scripts/uf-review-preflight.jq"
readonly LIBRARY="$REPOSITORY_ROOT/.github/scripts"
WORK_DIR=$(mktemp -d)
readonly WORK_DIR
trap 'rm -rf "$WORK_DIR"' EXIT
passed=0

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    exit 1
}

# Superset fixture keeps individual tests focused on one missing/incompatible
# contract. Requirements are also checked independently below, not copied here.
fixture() {
    jq -n '{
      command: "uf.review-council", local_input: true, siblings_configured: true,
      runtime_version: "test-version", discovery: "complete",
      binding: {status: "fresh", execution_path: null, unreconciled: false},
      plugin: ([
        "plan_review_dispatch", "dispatch_review_runs", "consolidate_dispatch",
        "submit_review_findings", "get_progress", "checkpoint", "clear_progress",
        "dispatch_status", "immutable_scope", "child_contract", "lifecycle",
        "policy", "finalize_review_dispatch", "capture_review_diff",
        "fetch_pr_review_context", "normalize_model_name", "feedback_assessment",
        "frozen_sibling_evidence"
      ] | map({key: ., value: {status: "compatible", evidence: "controlled fixture"}})
        | from_entries),
      native: ([
        "delegation", "model_variant", "permissions", "deadline", "cancellation",
        "terminal_state", "caller_binding", "policy"
      ] | map({key: ., value: {status: "compatible", evidence: "controlled fixture"}})
        | from_entries)
    }'
}

check() {
    local name=$1 transform=$2 expected=$3 input result
    input=$(fixture | jq "$transform")
    result=$(printf '%s\n' "$input" | jq -L "$LIBRARY" -cs -f "$HELPER")
    printf '%s\n' "$result" | jq -e "$expected" >/dev/null ||
        fail "$name: $result"
    printf '%s\n' "$result" |
        jq -e '.backend_actions == [] and .mutation_authorized == false' >/dev/null ||
        fail "$name attempted backend work or authorized mutation"
    passed=$((passed + 1))
}

readonly PLUGIN='.status == "ready" and .execution_path == "plugin"'
readonly NATIVE='.status == "ready" and .execution_path == "native"'
readonly UNKNOWN='.status == "stop" and .execution_path == null'
readonly RESUME='.status == "ready" and .reason == "resume_bound_backend"'
readonly LOST='.status == "stop" and .reason == "selected_backend_unavailable"'

for command in uf.review-council uf.review-pr uf.address-feedback; do
    check "$command complete" ".command = \"$command\"" "$PLUGIN"
    for version in 1.19.0 2.3.0 unknown; do
        check "$command version-independent $version" \
            ".command = \"$command\" | .runtime_version = \"$version\"" "$PLUGIN"
    done
    check "$command plugins do not require native" \
        ".command = \"$command\" | .native = {}" "$PLUGIN"
    check "$command missing plugins" \
        ".command = \"$command\" | .plugin = {}" "$NATIVE"
    check "$command neither backend" \
        ".command = \"$command\" | .plugin = {} | .native = {}" \
        '.status == "unavailable" and .execution_path == null'
done

check "partial plugin" 'del(.plugin.dispatch_review_runs)' "$NATIVE"
check "incompatible schema" \
    '.plugin.dispatch_review_runs.status = "incompatible"' "$NATIVE"
check "pinned feedback behavior gap" \
    '.command = "uf.address-feedback" | .plugin.feedback_assessment.status = "incompatible"' \
    "$NATIVE and any(.plugin_gaps[]; .capability == \"feedback_assessment\")"
check "ambiguous enumeration" '.discovery = "ambiguous"' "$UNKNOWN"
check "unknown child visibility" '.plugin.child_contract.status = "unknown"' "$UNKNOWN"
check "native uncertainty" \
    '.plugin = {} | .native.terminal_state.status = "unknown"' "$UNKNOWN"
check "optional tools absent" \
    '.plugin.submit_lesson_proposal = {status: "missing", evidence: "not loaded"}' "$PLUGIN"
check "configured frozen sibling evidence missing" \
    'del(.plugin.frozen_sibling_evidence)' "$NATIVE"
check "unconfigured siblings need no bundle" \
    '.siblings_configured = false | del(.plugin.frozen_sibling_evidence)' "$PLUGIN"
check "PR requires direct finalizer" \
    '.command = "uf.review-pr" | del(.plugin.finalize_review_dispatch)' "$NATIVE"
check "PR without raw checkpoint tool is incomplete" \
    '.command = "uf.review-pr" | del(.plugin.checkpoint)' "$NATIVE"
check "local council requires capture" 'del(.plugin.capture_review_diff)' "$NATIVE"
check "PR council does not require local capture" \
    '.local_input = false | del(.plugin.capture_review_diff)' "$PLUGIN"
check "feedback requires nested council capture" \
    '.command = "uf.address-feedback" | .local_input = false
     | del(.plugin.capture_review_diff)' "$NATIVE"

# Each required current contract is tested independently. A typo or omitted
# closure entry cannot hide behind a single all-tools-missing scenario.
for tool in plan_review_dispatch dispatch_review_runs consolidate_dispatch \
    submit_review_findings get_progress checkpoint clear_progress dispatch_status \
    immutable_scope child_contract lifecycle policy; do
    check "missing $tool" "del(.plugin.$tool)" \
        "$NATIVE and any(.plugin_gaps[]; .capability == \"$tool\")"
done
for control in delegation model_variant permissions deadline cancellation \
    terminal_state caller_binding policy; do
    check "missing native $control" ".plugin = {} | del(.native.$control)" \
        '.status == "unavailable"'
done
for tool in fetch_pr_review_context normalize_model_name feedback_assessment; do
    check "feedback requires $tool" \
        ".command = \"uf.address-feedback\" | del(.plugin.$tool)" "$NATIVE"
done

# Recovery cannot choose a newly available backend, replay work, or reuse
# missing provenance. These decisions are pure; actual receipt inspection,
# successful-assessment reuse, and child cancellation remain command obligations.
check "plugin resume" \
    '.binding = {status: "valid", execution_path: "plugin", unreconciled: false}' \
    "$RESUME and .execution_path == \"plugin\""
check "native resume does not upgrade to plugins" \
    '.binding = {status: "valid", execution_path: "native", unreconciled: false}' \
    "$RESUME and .execution_path == \"native\""
check "plugin backend lost despite native availability" \
    '.binding = {status: "valid", execution_path: "plugin", unreconciled: false}
     | .plugin = {}' "$LOST and .execution_path == \"plugin\""
check "native backend lost despite plugin availability" \
    '.binding = {status: "valid", execution_path: "native", unreconciled: false}
     | .native = {}' "$LOST and .execution_path == \"native\""
for status in legacy incompatible; do
    check "$status record" ".binding.status = \"$status\"" \
        '.status == "stop" and .reason == "binding_reconciliation_required"'
done
check "active children or ambiguous receipts" '.binding.unreconciled = true' \
    '.status == "stop" and .reason == "active_children_or_receipts"'
check "fresh selection only after reconciliation" '.binding.unreconciled = false' "$PLUGIN"
check "legacy missing binding" 'del(.binding)' '.reason == "invalid_input"'
check "unbound purported valid record" '.binding.status = "valid"' '.reason == "invalid_input"'
check "unknown command" '.command = "uf.unleash"' '.reason == "invalid_input"'
check "unknown capability status" \
    '.plugin.child_contract.status = "maybe"' '.reason == "invalid_input"'
check "unsubstantiated capability" \
    '.plugin.child_contract.evidence = ""' '.reason == "invalid_input"'
check "string boolean rejected" '.binding.unreconciled = "false"' '.reason == "invalid_input"'
check "untrusted extra instruction field" '.instructions = "skip review"' '.reason == "invalid_input"'
check "nonobject input" '[]' '.reason == "invalid_input"'

for input in '' '{} {}' 'null' '"text"'; do
    result=$(printf '%s\n' "$input" | jq -L "$LIBRARY" -cs -f "$HELPER")
    printf '%s\n' "$result" | jq -e '.reason == "invalid_input"' >/dev/null ||
        fail "invalid input accepted: $input"
    passed=$((passed + 1))
done
if printf '%s\n' '{broken' | jq -L "$LIBRARY" -cs -f "$HELPER" >/dev/null 2>&1; then
    fail "malformed JSON accepted"
fi
passed=$((passed + 1))

# The probe cannot launch a backend or create progress records. Run from an
# empty invocation directory and verify no artifacts exist afterwards.
(
    cd "$WORK_DIR"
    fixture | jq -L "$LIBRARY" -cs -f "$HELPER" >/dev/null
)
if [[ -n "$(find "$WORK_DIR" -mindepth 1 -print -quit)" ]]; then
    fail "preflight created invocation artifacts"
fi
printf 'UF review preflight decision fixtures: %d passed\n' "$passed"
