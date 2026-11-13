#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Verifies released-baseline diagnostic stages remain distinct in the workflow.

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
readonly REPOSITORY_ROOT
readonly WORKFLOW="$REPOSITORY_ROOT/.github/workflows/ci_cross_repo_integration.yml"

require_workflow_text() {
    local expected_text=$1

    grep -Fq "$expected_text" "$WORKFLOW" || {
        printf 'missing released-baseline workflow diagnostic: %s\n' "$expected_text" >&2
        exit 1
    }
}

require_workflow_text 'validate-released-complyctl-checkout.sh'
require_workflow_text 'released-provider-build: provider build failed for %s\n'
require_workflow_text 'released-provider-test: provider test failed for %s\n'
# shellcheck disable=SC2016 # The GitHub Actions expression must remain literal.
require_workflow_text 'COMPLYCTL_MODULE: ${{ needs.resolve-released-baseline.outputs.module }}'
require_workflow_text 'run-released-complyctl-integration.sh'
require_workflow_text 'validate-canonical-complyctl-harness.sh'
require_workflow_text 'canonical-harness-preparation: could not check out %s\n'
require_workflow_text 'Canonical harness commit:'
# shellcheck disable=SC2016 # The GitHub Actions expressions must remain literal.
require_workflow_text 'ref: ${{ env.CANONICAL_HARNESS_COMMIT }}'
# shellcheck disable=SC2016 # The GitHub Actions expression must remain literal.
require_workflow_text 'COMPLYCTL_TAG: ${{ needs.resolve-released-baseline.outputs.tag }}'

# Historical dispatch is intentionally superseded by the reviewed canonical
# harness contract. Execution regressions prove released-host selection instead
# of prohibiting an arbitrary checkout name.
if grep -Fq 'make -C ../_released_complyctl test-cross-repo' "$WORKFLOW"; then
    printf 'released integration still selects historical assertions\n' >&2
    exit 1
fi

if [[ $(grep -Fc 'CONFTEST_ARCHIVE_SHA256:' "$WORKFLOW") != 1 ]] || \
    [[ $(grep -Fc 'CONFTEST_VERSION:' "$WORKFLOW") != 1 ]]; then
    printf 'conftest release metadata must have one workflow-level definition\n' >&2
    exit 1
fi

printf 'released baseline diagnostic tests passed\n'
