#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Dispatch canonical assertions against prebuilt released binaries; never build.

set -euo pipefail

SCRIPT_DIRECTORY=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
PROVIDER_ROOT=$(cd "$SCRIPT_DIRECTORY/../.." && pwd -P)
readonly SCRIPT_DIRECTORY PROVIDER_ROOT

die() {
    printf 'released-integration-preparation: %s\n' "$*" >&2
    exit 1
}

bash "$SCRIPT_DIRECTORY/validate-canonical-complyctl-harness.sh" \
    --checkout-dir "${COMPLYCTL_DIR:-}" --commit "${CANONICAL_HARNESS_COMMIT:-}"
harness_root=$(cd "$COMPLYCTL_DIR" && pwd -P)

[[ "${COMPLYCTL_CROSS_REPO_INDEX_FIXTURE_TEST:-}" != 1 ]] || \
    die 'fixture mode cannot validate released-host integration'
[[ -n "${COMPLYCTL_TAG:-}" ]] || die 'missing released host tag'
[[ "${RELEASED_COMPLYCTL_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] || die 'missing full released commit'
[[ "${RELEASED_COMPLYCTL_DIR:-}" == /* && -d "$RELEASED_COMPLYCTL_DIR" ]] || \
    die 'released checkout must be an existing absolute directory'
released_root=$(cd "$RELEASED_COMPLYCTL_DIR" && pwd -P)
actual_root=$(git -C "$released_root" rev-parse --show-toplevel) || die 'not a released checkout'
[[ "$actual_root" == "$released_root" ]] || die 'released host must be a separate checkout root'
actual_commit=$(git -C "$released_root" rev-parse HEAD) || die 'cannot resolve released HEAD'
[[ "$actual_commit" == "$RELEASED_COMPLYCTL_COMMIT" ]] || \
    die "released checkout is $actual_commit, expected $RELEASED_COMPLYCTL_COMMIT"
git -C "$released_root" diff --quiet HEAD -- || die 'released source has tracked modifications'

for binary in complyctl mock-oci-registry; do
    [[ ! -L "$released_root/bin/$binary" ]] || die "released executable is a symlink: bin/$binary"
    [[ -f "$released_root/bin/$binary" && -x "$released_root/bin/$binary" ]] || \
        die "missing released executable: bin/$binary"
done
[[ "${PROVIDERS_BIN_DIR:-}" == /* && -d "$PROVIDERS_BIN_DIR" ]] || \
    die 'provider binaries must be in an existing absolute directory'
provider_bin=$(cd "$PROVIDERS_BIN_DIR" && pwd -P)
[[ "$provider_bin" == "$PROVIDER_ROOT/bin" ]] || \
    die 'provider binaries must come from this isolated provider workspace'
for binary in complyctl-provider-openscap complyctl-provider-ampel complyctl-provider-opa; do
    [[ ! -L "$provider_bin/$binary" ]] || die "isolated provider executable is a symlink: $binary"
    [[ -f "$provider_bin/$binary" && -x "$provider_bin/$binary" ]] || \
        die "missing isolated provider executable: $binary"
done

printf 'Released complyctl host: %s (%s)\n' "$COMPLYCTL_TAG" "$actual_commit"
printf 'Canonical complyctl harness: %s\n' "$CANONICAL_HARNESS_COMMIT"
status=0
COMPLYCTL_DIR="$harness_root" COMPLYCTL_BIN_DIR="$released_root/bin" \
    PROVIDERS_BIN_DIR="$provider_bin" make -C "$PROVIDER_ROOT" test-cross-repo || status=$?
if ((status != 0)); then
    printf 'released-integration: cross-repo integration failed for %s\n' "$COMPLYCTL_TAG" >&2
fi
exit "$status"
