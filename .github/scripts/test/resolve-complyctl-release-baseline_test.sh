#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Tests for .github/scripts/resolve-complyctl-release-baseline.sh.

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
readonly REPOSITORY_ROOT
readonly RESOLVER="$REPOSITORY_ROOT/.github/scripts/resolve-complyctl-release-baseline.sh"
readonly COMMIT_A='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
readonly COMMIT_B='bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
readonly COMMIT_C='cccccccccccccccccccccccccccccccccccccccc'

temporary_directory=$(mktemp -d)
cleanup() {
    rm -rf "$temporary_directory"
}
trap cleanup EXIT

write_module_file() {
    local version=$1
    local module_path=${2:-github.com/complytime/complyctl}

    cat > "$temporary_directory/go.mod" <<EOF
module example.com/test

go 1.26.10

require $module_path $version
EOF
}

write_tags_file() {
    cat > "$temporary_directory/tags"
}

assert_output() {
    local expected=$1
    local actual

    actual=$(bash "$RESOLVER" --module-file "$temporary_directory/go.mod" \
        --tags-file "$temporary_directory/tags")
    [[ "$actual" == "$expected" ]] || {
        printf 'expected:\n%s\nactual:\n%s\n' "$expected" "$actual" >&2
        exit 1
    }
}

test_normal_release_selection() {
    write_module_file 'v1.1.0'
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v1.1.0
$COMMIT_B	refs/tags/v1.4.2
$COMMIT_C	refs/tags/v1.4.2^{}
EOF
    assert_output "tag=v1.4.2
commit=$COMMIT_C"
}

test_prerelease_tags_are_excluded() {
    write_module_file 'v1.1.0'
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v1.2.0
$COMMIT_B	refs/tags/v1.9.0-rc.1
EOF
    assert_output "tag=v1.2.0
commit=$COMMIT_A"
}

test_malformed_tags_are_excluded() {
    write_module_file 'v1.1.0'
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v1.2
$COMMIT_B	refs/tags/v1.2.3.4
$COMMIT_C	refs/tags/v1.2.3
EOF
    assert_output "tag=v1.2.3
commit=$COMMIT_C"
}

test_v0_requires_matching_minor_version() {
    write_module_file 'v0.4.1'
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v0.4.9
$COMMIT_B	refs/tags/v0.5.0
EOF
    assert_output "tag=v0.4.9
commit=$COMMIT_A"
}

test_v2_uses_semantic_import_path() {
    write_module_file 'v2.1.0' 'github.com/complytime/complyctl/v2'
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v1.9.0
$COMMIT_B	refs/tags/v2.3.0
EOF
    assert_output "tag=v2.3.0
commit=$COMMIT_B"

    GITHUB_OUTPUT="$temporary_directory/github-output" \
        bash "$RESOLVER" --module-file "$temporary_directory/go.mod" \
        --tags-file "$temporary_directory/tags" > /dev/null
    grep -Fxq 'module=github.com/complytime/complyctl/v2' "$temporary_directory/github-output" || {
        printf 'expected resolver to emit the v2 module path\n' >&2
        exit 1
    }
}

test_no_baseline_fails_closed() {
    write_module_file 'v1.1.0'
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v2.0.0
$COMMIT_B	refs/tags/v1.2.0-rc.1
EOF

    if bash "$RESOLVER" --module-file "$temporary_directory/go.mod" \
        --tags-file "$temporary_directory/tags" > /dev/null 2> "$temporary_directory/error"; then
        printf 'expected resolver to fail when no stable baseline exists\n' >&2
        exit 1
    fi

    grep -qx 'release-baseline: no stable complyctl release matches v1\.' \
        "$temporary_directory/error" || {
        cat "$temporary_directory/error" >&2
        exit 1
    }
}

assert_failure() {
    local expected_message=$1

    : > "$temporary_directory/github-output"
    if GITHUB_OUTPUT="$temporary_directory/github-output" \
        bash "$RESOLVER" --module-file "$temporary_directory/go.mod" \
        --tags-file "$temporary_directory/tags" \
        > "$temporary_directory/output" 2> "$temporary_directory/error"; then
        printf 'expected resolver to fail: %s\n' "$expected_message" >&2
        exit 1
    fi
    grep -Fxq "$expected_message" "$temporary_directory/error" || {
        cat "$temporary_directory/error" >&2
        exit 1
    }
    [[ ! -s "$temporary_directory/output" && ! -s "$temporary_directory/github-output" ]] || {
        printf 'failed resolver must not publish a baseline\n' >&2
        exit 1
    }
}

test_non_direct_entries_are_rejected() {
    local directive

    write_tags_file <<EOF
$COMMIT_A	refs/tags/v1.2.0
EOF
    for directive in \
        'exclude github.com/complytime/complyctl v1.0.0' \
        'replace github.com/complytime/complyctl v1.1.0 => example.com/fork v1.2.0' \
        'require github.com/complytime/complyctl v1.1.0 // indirect'; do
        cat > "$temporary_directory/go.mod" <<EOF
module example.com/test

go 1.26.10

$directive
exclude github.com/complytime/complyctl v0.9.0
EOF
        assert_failure 'release-baseline: missing direct github.com/complytime/complyctl dependency'
    done
}

test_direct_requirement_ignores_other_sections() {
    write_module_file 'v1.1.0'
    cat >> "$temporary_directory/go.mod" <<'EOF'

exclude github.com/complytime/complyctl v0.9.0
replace github.com/complytime/complyctl => example.com/fork v2.0.0+incompatible
EOF
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v1.2.0
$COMMIT_B	refs/tags/v0.9.0
$COMMIT_C	refs/tags/v2.0.0
EOF
    assert_output "tag=v1.2.0
commit=$COMMIT_A"
}

test_malformed_module_reports_resolution_stage() {
    printf 'module example.com/test\ninvalid-directive\n' > "$temporary_directory/go.mod"
    write_tags_file <<EOF
$COMMIT_A	refs/tags/v1.2.0
EOF
    assert_failure "release-baseline: could not parse module file: $temporary_directory/go.mod"
}

test_normal_release_selection
test_prerelease_tags_are_excluded
test_malformed_tags_are_excluded
test_v0_requires_matching_minor_version
test_v2_uses_semantic_import_path
test_no_baseline_fails_closed
test_non_direct_entries_are_rejected
test_direct_requirement_ignores_other_sections
test_malformed_module_reports_resolution_stage

printf 'release baseline resolver tests passed\n'
