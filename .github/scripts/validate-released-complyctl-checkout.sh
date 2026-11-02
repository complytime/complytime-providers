#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Validate that a released complyctl checkout can run the provider integration harness.

set -euo pipefail

checkout_dir=''
complyctl_tag=''

die() {
    printf 'released-checkout-prerequisite: %s: %s\n' "$complyctl_tag" "$*" >&2
    exit 1
}

usage() {
    printf 'Usage: %s --checkout-dir PATH --tag TAG\n' "${0##*/}" >&2
}

while (($# > 0)); do
    case "$1" in
        --checkout-dir)
            (($# >= 2)) || die '--checkout-dir requires a path'
            checkout_dir=$2
            shift 2
            ;;
        --tag)
            (($# >= 2)) || die '--tag requires a value'
            complyctl_tag=$2
            shift 2
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            usage
            die "unknown argument: $1"
            ;;
    esac
done

[[ -n "$complyctl_tag" ]] || die 'missing selected release tag'
[[ -d "$checkout_dir" ]] || die "missing checkout directory: $checkout_dir"

require_path() {
    local relative_path=$1

    [[ -e "$checkout_dir/$relative_path" ]] || die "missing required path: $relative_path"
}

require_path 'Makefile'
require_path 'tests/cross-repo/cross_repo_integration_test.sh'
require_path 'tests/cross-repo/testdata'
require_path 'cmd/mock-oci-registry'

[[ -x "$checkout_dir/tests/cross-repo/cross_repo_integration_test.sh" ]] || \
    die 'integration harness is not executable: tests/cross-repo/cross_repo_integration_test.sh'

make -C "$checkout_dir" -n build > /dev/null || die 'missing or unusable build target'
make -C "$checkout_dir" -n test-cross-repo COMPLYCTL_DIR=. > /dev/null || \
    die 'missing or unusable test-cross-repo target'

printf 'released checkout prerequisites verified for %s\n' "$complyctl_tag"
