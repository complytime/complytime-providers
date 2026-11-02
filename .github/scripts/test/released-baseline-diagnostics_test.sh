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
require_workflow_text 'released-integration: cross-repo integration failed for %s\n'
# shellcheck disable=SC2016 # The GitHub Actions expression must remain literal.
require_workflow_text 'COMPLYCTL_TAG: ${{ needs.resolve-released-baseline.outputs.tag }}'

printf 'released baseline diagnostic tests passed\n'
