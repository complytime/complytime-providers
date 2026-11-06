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
require_workflow_text 'released-integration: cross-repo integration failed for %s\n'
require_workflow_text 'make -C ../_released_complyctl test-cross-repo'
# shellcheck disable=SC2016 # The GitHub Actions expression must remain literal.
require_workflow_text 'COMPLYCTL_TAG: ${{ needs.resolve-released-baseline.outputs.tag }}'

if grep -Fq '_compatibility_harness' "$WORKFLOW"; then
    printf 'temporary compatibility harness checkout is still present\n' >&2
    exit 1
fi

if [[ $(grep -Fc 'CONFTEST_ARCHIVE_SHA256:' "$WORKFLOW") != 1 ]] || \
    [[ $(grep -Fc 'CONFTEST_VERSION:' "$WORKFLOW") != 1 ]]; then
    printf 'conftest release metadata must have one workflow-level definition\n' >&2
    exit 1
fi

printf 'released baseline diagnostic tests passed\n'
