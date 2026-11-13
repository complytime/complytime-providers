#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Offline execution of the same preparation and dispatch path used by CI.

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
readonly REPOSITORY_ROOT
readonly VALIDATOR="$REPOSITORY_ROOT/.github/scripts/validate-canonical-complyctl-harness.sh"
readonly WORKFLOW="$REPOSITORY_ROOT/.github/workflows/ci_cross_repo_integration.yml"
temporary_directory=$(mktemp -d)
trap 'rm -rf "$temporary_directory"' EXIT
fixture_count=0
passed=0

commit_fixture() {
    git -C "$1" add -- .
    git -C "$1" -c user.name='Harness fixture' -c user.email='fixture@example.invalid' \
        -c commit.gpgsign=false commit -qm fixture
}

write_binary() {
    cat > "$1" <<'BINARY'
#!/usr/bin/env bash
printf '%s\n' "$0" >> "$TRACE"
BINARY
    chmod +x "$1"
}

new_fixture() {
    local file binary
    fixture_count=$((fixture_count + 1))
    fixture="$temporary_directory/case $fixture_count"
    harness="$fixture/canonical harness"
    released="$fixture/released host"
    providers="$fixture/provider workspace"
    trace="$fixture/trace"
    mkdir -p "$harness/tests/cross-repo/testdata/granular-policies" \
        "$harness/bin" "$released/bin" "$providers/bin" "$providers/.github/scripts"
    : > "$trace"
    git -c init.defaultBranch=fixture init -q "$harness"
    git -c init.defaultBranch=fixture init -q "$released"
    cat > "$harness/tests/cross-repo/cross_repo_integration_test.sh" <<'HARNESS'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT=$(cd "$(dirname "$0")/../.." && pwd -P)
source "$REPO_ROOT/tests/cross-repo/harness_helpers.sh"
select_host_binaries "${REPO_ROOT}"
printf 'fixtures=%s\n' "$REPO_ROOT/tests/cross-repo/testdata" >> "$TRACE"
printf 'providers=%s\n' "$PROVIDERS_BIN_DIR" >> "$TRACE"
"$selected_bin/complyctl"
"$selected_bin/mock-oci-registry"
"$PROVIDERS_BIN_DIR/complyctl-provider-ampel"
"$PROVIDERS_BIN_DIR/complyctl-provider-opa"
echo 'fixture integration output preserved'
exit "${HARNESS_EXIT_STATUS:-0}"
HARNESS
    chmod +x "$harness/tests/cross-repo/cross_repo_integration_test.sh"
    cat > "$harness/tests/cross-repo/harness_helpers.sh" <<'HELPER'
select_host_binaries() {
    selected_bin=${COMPLYCTL_BIN_DIR:-$1/bin}
}
HELPER
    for file in evidence_helpers.sh evidence_assertions.sh testdata/complytime.yaml \
        testdata/granular-policies/block-force-push.json \
        testdata/test-deployment-bad.yaml testdata/test-deployment-good.yaml; do
        printf 'offline fixture\n' > "$harness/tests/cross-repo/$file"
    done
    cat > "$harness/Makefile" <<'MAKEFILE'
build test-cross-repo:
	@echo unexpected-canonical-build-or-dispatch >> "$$TRACE"
	@exit 97
MAKEFILE
    cat > "$released/Makefile" <<'MAKEFILE'
build test-cross-repo:
	@echo unexpected-historical-build-or-dispatch >> "$$TRACE"
	@exit 98
MAKEFILE
    commit_fixture "$harness"
    commit_fixture "$released"
    harness_commit=$(git -C "$harness" rev-parse HEAD)
    released_commit=$(git -C "$released" rev-parse HEAD)
    for binary in complyctl mock-oci-registry; do
        write_binary "$harness/bin/$binary"
        write_binary "$released/bin/$binary"
    done
    for binary in complyctl-provider-openscap complyctl-provider-ampel complyctl-provider-opa; do
        write_binary "$providers/bin/$binary"
    done
    cp "$REPOSITORY_ROOT/Makefile" "$providers/Makefile"
    cp "$REPOSITORY_ROOT/.github/scripts/validate-canonical-complyctl-harness.sh" \
        "$REPOSITORY_ROOT/.github/scripts/run-released-complyctl-integration.sh" \
        "$providers/.github/scripts/"
    git -c init.defaultBranch=fixture init -q "$providers"
    git -C "$providers" add -- Makefile .github
    git -C "$providers" -c user.name='Harness fixture' -c user.email='fixture@example.invalid' \
        -c commit.gpgsign=false commit -qm fixture
}

validate() {
    bash "$VALIDATOR" --checkout-dir "$harness" --commit "$harness_commit"
}

invoke() {
    local provider_bin=${1:-$providers/bin}
    env -u GITHUB_TOKEN TRACE="$trace" COMPLYCTL_DIR="$harness" \
        CANONICAL_HARNESS_COMMIT="$harness_commit" RELEASED_COMPLYCTL_DIR="$released" \
        RELEASED_COMPLYCTL_COMMIT="$released_commit" COMPLYCTL_TAG=v1.2.3 \
        PROVIDERS_BIN_DIR="$provider_bin" \
        bash "$providers/.github/scripts/run-released-complyctl-integration.sh"
}

pass() {
    passed=$((passed + 1))
    printf 'PASS: %s\n' "$1"
}

expect_failure() {
    local diagnostic=$1
    shift
    if "$@" > "$fixture/output" 2>&1; then
        printf 'unexpected success: %s\n' "$diagnostic" >&2
        exit 1
    fi
    grep -Fq "$diagnostic" "$fixture/output" || {
        cat "$fixture/output" >&2
        exit 1
    }
    [[ ! -s "$trace" ]] || {
        printf 'preparation failure executed a binary:\n' >&2
        cat "$trace" >&2
        exit 1
    }
    pass "$diagnostic"
}

assert_trace() {
    local provider_bin=$1
    printf 'fixtures=%s\nproviders=%s\n%s\n%s\n%s\n%s\n' \
        "$harness/tests/cross-repo/testdata" "$provider_bin" \
        "$released/bin/complyctl" "$released/bin/mock-oci-registry" \
        "$provider_bin/complyctl-provider-ampel" "$provider_bin/complyctl-provider-opa" \
        > "$fixture/expected-trace"
    cmp "$fixture/expected-trace" "$trace"
    git -C "$harness" diff --exit-code HEAD --
    git -C "$released" diff --exit-code HEAD --
}

new_fixture
validate > /dev/null
invoke > "$fixture/output"
assert_trace "$providers/bin"
grep -Fq "$harness_commit" "$fixture/output"
grep -Fq "$released_commit" "$fixture/output"
pass 'released binaries execute through provider Make with spaced paths and canonical fixtures'

new_fixture
COMPLYCTL_CROSS_REPO_INDEX_FIXTURE_TEST=1 \
    expect_failure 'fixture mode cannot validate released-host integration' invoke

new_fixture
harness_commit=main
expect_failure 'canonical-harness-preparation: expected a full immutable commit' invoke

new_fixture
harness_commit=$released_commit
expect_failure "expected $harness_commit" invoke

new_fixture
missing_harness="$harness/missing"
harness=$missing_harness
expect_failure 'canonical-harness-preparation: missing checkout directory' invoke

for file in cross_repo_integration_test.sh harness_helpers.sh evidence_helpers.sh \
    evidence_assertions.sh testdata/complytime.yaml \
    testdata/granular-policies/block-force-push.json \
    testdata/test-deployment-bad.yaml testdata/test-deployment-good.yaml; do
    new_fixture
    rm -- "$harness/tests/cross-repo/$file"
    expect_failure "missing or unreadable harness path: tests/cross-repo/$file" invoke
done

new_fixture
chmod -x "$harness/tests/cross-repo/cross_repo_integration_test.sh"
expect_failure 'integration harness is not executable' invoke

new_fixture
printf 'select_host_binaries() { :; }\n' > "$harness/tests/cross-repo/harness_helpers.sh"
expect_failure 'unsupported host-selection interface: COMPLYCTL_BIN_DIR' invoke

new_fixture
printf 'modified\n' >> "$harness/tests/cross-repo/evidence_helpers.sh"
expect_failure 'harness assertions or fixtures differ from the pinned checkout' invoke

new_fixture
released_commit=$harness_commit
expect_failure "expected $released_commit" invoke

new_fixture
printf '# modified\n' >> "$released/Makefile"
expect_failure 'released source has tracked modifications' invoke

for binary in complyctl mock-oci-registry; do
    new_fixture
    rm -- "$released/bin/$binary"
    expect_failure "missing released executable: bin/$binary" invoke
    new_fixture
    chmod -x "$released/bin/$binary"
    expect_failure "missing released executable: bin/$binary" invoke
    new_fixture
    rm -- "$released/bin/$binary"
    ln -s "$harness/bin/$binary" "$released/bin/$binary"
    expect_failure "released executable is a symlink: bin/$binary" invoke
done

for binary in complyctl-provider-openscap complyctl-provider-ampel complyctl-provider-opa; do
    new_fixture
    rm -- "$providers/bin/$binary"
    expect_failure "missing isolated provider executable: $binary" invoke
done

new_fixture
mkdir -p "$fixture/other provider bin"
cp "$providers/bin/"* "$fixture/other provider bin/"
expect_failure 'provider binaries must come from this isolated provider workspace' \
    invoke "$fixture/other provider bin"

new_fixture
status=0
HARNESS_EXIT_STATUS=37 invoke > "$fixture/output" 2>&1 || status=$?
[[ "$status" -eq 2 ]] # Make preserves recipe failure as exit status 2.
grep -Fq 'fixture integration output preserved' "$fixture/output"
grep -Fq 'released-integration: cross-repo integration failed for v1.2.3' "$fixture/output"
assert_trace "$providers/bin"
pass 'integration failure and its output survive Make dispatch without fallback'

# Extract the actual run blocks so build/test failure coverage executes workflow
# commands, not a second implementation of the build orchestration.
workflow_command() {
    awk -v step="$1" '
        $0 == "            - name: " step { selected=1; next }
        selected && /^            - / { exit }
        selected && /^              run: \|/ { block=1; next }
        selected && /^              run: / { sub(/^              run: /, ""); print; exit }
        block { sub(/^                  /, ""); print }
    ' "$WORKFLOW"
}

prepare_workflow() {
    new_fixture
    workflow_root="$fixture/workflow"
    mkdir -p "$workflow_root/fault-bin"
    mv "$providers" "$workflow_root/source"
    providers="$workflow_root/source"
    # Only committed provider inputs enter the isolated worktree, not prebuilt bin/.
    ln -s "$released" "$providers/_released_complyctl"
    cp "$released/bin/complyctl" "$workflow_root/binary-template"
    cat > "$workflow_root/fault-bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
printf 'go %s\n' "$*" >> "$BUILD_TRACE"
if [[ "$*" == 'mod edit -replace=github.com/complytime/complyctl=../_released_complyctl' ]]; then
    [[ "${FAIL_STAGE:-}" != dependency-replacement ]] || exit 61
elif [[ "$*" == 'mod tidy' ]]; then
    [[ "${FAIL_STAGE:-}" != dependency-resolution ]] || exit 62
else
    exit 90
fi
GO
    cat > "$workflow_root/fault-bin/make" <<'MAKE'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == -C ]]; then
    exec "$REAL_MAKE" "$@"
fi
printf 'make %s in %s\n' "$*" "$PWD" >> "$BUILD_TRACE"
case "$1" in
    build)
        [[ "${FAIL_STAGE:-}" != provider-build ]] || exit 63
        mkdir -p bin
        for binary in complyctl-provider-openscap complyctl-provider-ampel complyctl-provider-opa; do
            cp "$BINARY_TEMPLATE" "bin/$binary"
        done
        ;;
    test) [[ "${FAIL_STAGE:-}" != provider-test ]] || exit 64 ;;
    *) exit 91 ;;
esac
MAKE
    chmod +x "$workflow_root/fault-bin/"*
    {
        printf 'cd %q\n' "$providers"
        workflow_command 'Create isolated provider workspace'
        printf 'cd %q\n' "$providers/_released_providers"
        workflow_command 'Replace complyctl dependency in isolated workspace'
        workflow_command 'Build providers against released complyctl baseline'
        workflow_command 'Test providers against released complyctl baseline'
        workflow_command 'Run released baseline cross-repo integration test'
    } > "$workflow_root/pipeline.sh"
}

invoke_workflow() {
    env -u GITHUB_TOKEN TRACE="$trace" BUILD_TRACE="$workflow_root/build-trace" \
        REAL_MAKE="$(command -v make)" BINARY_TEMPLATE="$workflow_root/binary-template" \
        PATH="$workflow_root/fault-bin:$PATH" COMPLYCTL_MODULE=github.com/complytime/complyctl \
        COMPLYCTL_DIR="$harness" CANONICAL_HARNESS_COMMIT="$harness_commit" \
        RELEASED_COMPLYCTL_DIR="$released" RELEASED_COMPLYCTL_COMMIT="$released_commit" \
        COMPLYCTL_TAG=v1.2.3 PROVIDERS_BIN_DIR="$providers/_released_providers/bin" \
        bash -e "$workflow_root/pipeline.sh"
}

prepare_workflow
invoke_workflow > "$fixture/output" 2>&1
assert_trace "$providers/_released_providers/bin"
grep -Fxq 'go mod edit -replace=github.com/complytime/complyctl=../_released_complyctl' \
    "$workflow_root/build-trace"
pass 'workflow builds isolated providers with released dependency before dispatch'

for stage in dependency-replacement dependency-resolution provider-build provider-test; do
    prepare_workflow
    case "$stage" in
        dependency-replacement) diagnostic='released-provider-build: could not configure providers' ;;
        dependency-resolution) diagnostic='released-provider-build: could not resolve provider dependencies' ;;
        provider-build) diagnostic='released-provider-build: provider build failed' ;;
        provider-test) diagnostic='released-provider-test: provider test failed' ;;
    esac
    FAIL_STAGE=$stage expect_failure "$diagnostic" invoke_workflow
done

printf 'Canonical harness execution tests: %s passed\n' "$passed"
