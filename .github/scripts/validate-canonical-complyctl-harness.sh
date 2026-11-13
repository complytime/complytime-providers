#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Validate the pinned assertion checkout independently of the released host.

set -euo pipefail

checkout_dir=''
expected_commit=''

die() {
    printf 'canonical-harness-preparation: %s\n' "$*" >&2
    exit 1
}

while (($# > 0)); do
    case "$1" in
        --checkout-dir|--commit)
            (($# >= 2)) || die "$1 requires a value"
            case "$1" in
                --checkout-dir) checkout_dir=$2 ;;
                --commit) expected_commit=$2 ;;
            esac
            shift 2
            ;;
        *) die "unknown argument: $1" ;;
    esac
done

[[ "$expected_commit" =~ ^[0-9a-f]{40}$ ]] || die 'expected a full immutable commit'
[[ -d "$checkout_dir" ]] || die "missing checkout directory: $checkout_dir"
checkout_dir=$(cd "$checkout_dir" && pwd -P)
actual_root=$(git -C "$checkout_dir" rev-parse --show-toplevel) || die 'not a Git checkout'
[[ "$actual_root" == "$checkout_dir" ]] || die 'harness must be a separate checkout root'
actual_commit=$(git -C "$checkout_dir" rev-parse HEAD) || die 'cannot resolve checkout HEAD'
[[ "$actual_commit" == "$expected_commit" ]] || \
    die "checkout is $actual_commit, expected $expected_commit"

readonly HARNESS_PATH='tests/cross-repo'
for file in cross_repo_integration_test.sh harness_helpers.sh evidence_helpers.sh \
    evidence_assertions.sh testdata/complytime.yaml \
    testdata/granular-policies/block-force-push.json \
    testdata/test-deployment-bad.yaml testdata/test-deployment-good.yaml; do
    [[ -f "$checkout_dir/$HARNESS_PATH/$file" && -r "$checkout_dir/$HARNESS_PATH/$file" ]] || \
        die "missing or unreadable harness path: $HARNESS_PATH/$file"
done
[[ -x "$checkout_dir/$HARNESS_PATH/cross_repo_integration_test.sh" ]] || \
    die 'integration harness is not executable'

# Check the pinned interface without sourcing code during preparation. Execution
# regressions and live integration separately prove binary selection behavior.
if ! grep -Fq 'COMPLYCTL_BIN_DIR' "$checkout_dir/$HARNESS_PATH/harness_helpers.sh" || \
    ! grep -Fq "select_host_binaries \"\${REPO_ROOT}\"" \
        "$checkout_dir/$HARNESS_PATH/cross_repo_integration_test.sh"; then
    die 'unsupported host-selection interface: COMPLYCTL_BIN_DIR'
fi
changes=$(git -C "$checkout_dir" status --porcelain --untracked-files=all -- "$HARNESS_PATH") || \
    die 'cannot verify harness contents'
[[ -z "$changes" ]] || die 'harness assertions or fixtures differ from the pinned checkout'

printf 'Canonical complyctl harness verified: %s\n' "$actual_commit"
