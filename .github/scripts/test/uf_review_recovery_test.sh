#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Controlled recovery evidence; no GitHub writes or reviewer sessions.
set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
readonly REPOSITORY_ROOT
readonly HELPER="$REPOSITORY_ROOT/.github/scripts/uf-review-recovery.jq"
readonly LIBRARY="$REPOSITORY_ROOT/.github/scripts"
passed=0

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    exit 1
}

fixture() {
    jq -n '
      ("a" * 64) as $scope | ("b" * 64) as $policy
      | ("c" * 64) as $agents | ("d" * 64) as $item
      | {
        event: "resume", council: null,
        binding: {
          version: 1, execution_path: "plugin", command: "uf.address-feedback",
          invocation_id: "controlled-invocation", project: "/fixture/project",
          target: "pr-42", backend_ref: "controlled-dispatch",
          scope_digest: $scope, policy_digest: $policy, agents_digest: $agents,
          active_children: [], receipts: []
        },
        current: {
          execution_path: "plugin", command: "uf.address-feedback",
          project: "/fixture/project", target: "pr-42", scope_digest: $scope,
          item_digest: $item, policy_digest: $policy, agents_digest: $agents
        },
        assessment: {
          execution_path: "plugin", backend_ref: "controlled-assessment",
          provenance: "plugin-validated", scope_digest: $scope,
          item_digest: $item, policy_digest: $policy, agents_digest: $agents
        }
      }'
}

check() {
    local name=$1 transform=$2 expected=$3 result
    result=$(fixture | jq "$transform" | jq -L "$LIBRARY" -cs -f "$HELPER")
    printf '%s\n' "$result" | jq -e "$expected" >/dev/null || fail "$name: $result"
    printf '%s\n' "$result" |
        jq -e '.backend_actions == [] and .mutation_authorized == false
               and .fresh_consent_required == true' >/dev/null ||
        fail "$name performed actions or reused mutation consent"
    passed=$((passed + 1))
}

readonly REUSE='.status == "reuse" and .supersede_assessment == false'
readonly STALE='.status == "reassess" and .supersede_assessment == true'
readonly STOP='.status == "stop"'

check "same plugin digests reused without dispatch" '.' "$REUSE"
check "same native digests reused without plugin provenance" \
    '.binding.execution_path = "native" | .current.execution_path = "native"
     | .assessment.execution_path = "native" | .assessment.provenance = "native-validated"' \
    "$REUSE and .execution_path == \"native\""
for event in runtime_error policy_rejection model_unavailable blocking_verdict; do
    check "$event cannot switch backend" ".event = \"$event\"" \
        "$STOP and .execution_path == \"plugin\" and .reason == \"$event\""
done
for field in scope_digest policy_digest agents_digest item_digest; do
    check "changed $field reassesses affected item" \
        ".current.$field = (\"e\" * 64)" "$STALE"
done
for field in scope_digest policy_digest agents_digest; do
    check "assessment-only stale $field" ".assessment.$field = (\"e\" * 64)" "$STALE"
    check "binding-only stale $field" ".binding.$field = (\"e\" * 64)" "$STALE"
done
check "assessment-only stale item digest" '.assessment.item_digest = ("e" * 64)' "$STALE"
check "other unchanged item is still reused" '.' "$REUSE"
check "missing assessment reconstructs only after reconciliation" '.assessment = null' \
    '.status == "reassess" and .reason == "assessment_reconstruction_required"'
check "missing assessment cannot replay active review" \
    '.assessment = null | .binding.active_children = ["child-7"]' \
    "$STOP and .reason == \"reconciliation_required\""
check "ambiguous posting receipt prevents replay" \
    '.binding.receipts = [{id: "reply-7", status: "ambiguous"}]' \
    "$STOP and .reason == \"reconciliation_required\""
check "confirmed posting receipt permits assessment reuse, not mutation" \
    '.binding.receipts = [{id: "reply-7", status: "confirmed"}]' "$REUSE"
check "backend conversion forbidden" '.current.execution_path = "native"' \
    "$STOP and .reason == \"incompatible_binding\""
check "wrong project" '.current.project = "/another-project"' "$STOP"
check "wrong PR" '.current.target = "pr-99"' "$STOP"
check "wrong command" '.current.command = "uf.review-pr"' "$STOP"
check "mixed assessment backend" \
    '.assessment.execution_path = "native" | .assessment.provenance = "native-validated"' \
    "$STOP and .reason == \"incompatible_assessment_backend\""
check "native evidence cannot claim plugin validation" \
    '.assessment.execution_path = "native"' "$STOP"
check "missing path provenance" 'del(.binding.execution_path)' \
    "$STOP and .reason == \"invalid_or_legacy_record\""
check "legacy binding" '.binding.version = 0' "$STOP"
check "incompatible future binding" '.binding.version = 2' "$STOP"
check "corrupt digests" '.assessment.scope_digest = "not-a-digest"' "$STOP"
check "unknown receipt status" \
    '.binding.receipts = [{id: "reply-7", status: "probably-posted"}]' "$STOP"
check "raw prior consent is rejected" '.consent = true' "$STOP"
check "post-fix council required" '.event = "post_fix_council"' \
    '.status == "council_required"'
for verdict in APPROVE 'APPROVE WITH ADVISORIES'; do
    check "passing council $verdict still needs confirmation" \
        ".event = \"post_fix_council\" |
         .council = {execution_path: \"plugin\", scope_digest: .current.scope_digest,
                     policy_digest: .current.policy_digest, agents_digest: .current.agents_digest,
                     verdict: \"$verdict\"}" \
        '.status == "confirmation_required" and .reason == "post_fix_review_passed"'
done
for verdict in 'REQUEST CHANGES' INCONCLUSIVE UNAVAILABLE; do
    check "council $verdict blocks" \
        ".event = \"post_fix_council\" |
         .council = {execution_path: \"plugin\", scope_digest: .current.scope_digest,
                     policy_digest: .current.policy_digest, agents_digest: .current.agents_digest,
                     verdict: \"$verdict\"}" \
        "$STOP and .reason == \"post_fix_review_not_passed\""
done
check "stale passing council rejected" \
    '.event = "post_fix_council" |
     .council = {execution_path: "plugin", scope_digest: ("f" * 64), verdict: "APPROVE",
                 policy_digest: .current.policy_digest, agents_digest: .current.agents_digest}' \
    "$STOP and .reason == \"incompatible_council\""
check "other-backend passing council rejected" \
    '.event = "post_fix_council" |
     .council = {execution_path: "native", scope_digest: .current.scope_digest, verdict: "APPROVE",
                 policy_digest: .current.policy_digest, agents_digest: .current.agents_digest}' \
    "$STOP and .reason == \"incompatible_council\""
for field in policy_digest agents_digest; do
    check "passing council with stale $field rejected" \
        ".event = \"post_fix_council\" |
         .council = {execution_path: \"plugin\", scope_digest: .current.scope_digest,
                     policy_digest: .current.policy_digest, agents_digest: .current.agents_digest,
                     verdict: \"APPROVE\"} | .council.$field = (\"f\" * 64)" \
        "$STOP and .reason == \"incompatible_council\""
done
check "invalid object" '[]' "$STOP"
if printf '%s\n' '{broken' | jq -L "$LIBRARY" -cs -f "$HELPER" >/dev/null 2>&1; then
    fail "malformed JSON accepted"
fi
passed=$((passed + 1))

printf 'UF review recovery decision fixtures: %d passed; external actions: 0\n' "$passed"
