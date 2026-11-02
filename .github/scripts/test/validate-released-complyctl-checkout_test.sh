#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Tests for .github/scripts/validate-released-complyctl-checkout.sh.

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
readonly REPOSITORY_ROOT
readonly VALIDATOR="$REPOSITORY_ROOT/.github/scripts/validate-released-complyctl-checkout.sh"
readonly COMPLYCTL_TAG='v1.2.3'

temporary_directory=$(mktemp -d)
cleanup() {
    rm -rf "$temporary_directory"
}
trap cleanup EXIT

create_valid_checkout() {
    local checkout_dir=$1

    mkdir -p "$checkout_dir/tests/cross-repo/testdata" "$checkout_dir/cmd/mock-oci-registry"
    cat > "$checkout_dir/Makefile" <<'EOF'
build:
	@:

test-cross-repo:
	@:
EOF
    cat > "$checkout_dir/tests/cross-repo/cross_repo_integration_test.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$checkout_dir/tests/cross-repo/cross_repo_integration_test.sh"
}

assert_fails_with() {
    local checkout_dir=$1
    local expected_message=$2

    if bash "$VALIDATOR" --checkout-dir "$checkout_dir" --tag "$COMPLYCTL_TAG" \
        > /dev/null 2> "$temporary_directory/error"; then
        printf 'expected validator to fail\n' >&2
        exit 1
    fi

    grep -Fqx "$expected_message" "$temporary_directory/error" || {
        cat "$temporary_directory/error" >&2
        exit 1
    }
}

test_valid_checkout_succeeds() {
    local checkout_dir="$temporary_directory/valid"

    create_valid_checkout "$checkout_dir"
    bash "$VALIDATOR" --checkout-dir "$checkout_dir" --tag "$COMPLYCTL_TAG" > /dev/null
}

test_missing_harness_reports_release_tag() {
    local checkout_dir="$temporary_directory/missing-harness"

    create_valid_checkout "$checkout_dir"
    rm "$checkout_dir/tests/cross-repo/cross_repo_integration_test.sh"
    assert_fails_with "$checkout_dir" \
        'released-checkout-prerequisite: v1.2.3: missing required path: tests/cross-repo/cross_repo_integration_test.sh'
}

test_missing_build_target_reports_release_tag() {
    local checkout_dir="$temporary_directory/missing-build-target"

    create_valid_checkout "$checkout_dir"
    cat > "$checkout_dir/Makefile" <<'EOF'
test-cross-repo:
	@:
EOF
    assert_fails_with "$checkout_dir" \
        'released-checkout-prerequisite: v1.2.3: missing or unusable build target'
}

test_valid_checkout_succeeds
test_missing_harness_reports_release_tag
test_missing_build_target_reports_release_tag

printf 'released checkout prerequisite tests passed\n'
