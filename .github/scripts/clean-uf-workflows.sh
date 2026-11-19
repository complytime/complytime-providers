#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Prune only expired, untracked invocation directories; receipts stay untouched.
set -euo pipefail

fail() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

# Make exports the raw value as data, never interpolated shell source.
days=${UF_WORKFLOW_RETENTION_DAYS_INPUT-}
if [[ ! $days =~ ^[0-9]+$ ]]; then
    printf 'error: UF_WORKFLOW_RETENTION_DAYS must be a positive integer\n' >&2
    exit 2
fi
if [[ ! $days =~ ^0*([1-9][0-9]{0,3})$ ]] || ((10#${BASH_REMATCH[1]} > 3650)); then
    printf 'error: UF_WORKFLOW_RETENTION_DAYS must be between 1 and 3650\n' >&2
    exit 2
fi
days=$((10#${BASH_REMATCH[1]}))

root=$(git rev-parse --show-toplevel) ||
    fail 'cannot verify that .uf/workflows is untracked; refusing to prune'
cd "$root"

check_tracked() {
    local tracked
    if ! tracked=$(git ls-files -- .uf/workflows); then
        fail 'cannot inspect tracked workflow paths; refusing to prune'
    fi
    [[ -z $tracked ]] || fail '.uf/workflows contains tracked files; refusing to prune'
}

check_tracked
[[ ! -L .uf && ! -L .uf/workflows ]] ||
    fail 'symlinked workflow tree; refusing to prune'
if [[ ! -d .uf/workflows ]]; then
    printf 'clean-uf-workflows: no .uf/workflows directory\n'
    exit 0
fi

listing=$(mktemp)
trap 'rm -f -- "$listing"' EXIT
if ! find .uf/workflows -mindepth 2 -maxdepth 2 -type d -print0 >"$listing"; then
    fail 'cannot enumerate .uf/workflows'
fi
mapfile -d '' -t directories <"$listing"
stale=()
for directory in "${directories[@]}"; do
    if ! first_file=$(find "$directory" -type f -print -quit); then
        fail 'cannot inspect workflow files'
    fi
    [[ -n $first_file ]] || continue
    if ! newer=$(find "$directory" -type f -newermt "-$days days" -print -quit); then
        fail 'cannot evaluate workflow retention'
    fi
    [[ -n $newer ]] || stale+=("$directory")
done

if ((${#stale[@]} == 0)); then
    printf 'clean-uf-workflows: no record lacks a write in the last %s days\n' "$days"
    exit 0
fi
printf 'clean-uf-workflows candidates (no write in the last %s days):\n' "$days"
printf '  %q\n' "${stale[@]}"
if [[ ${PRUNE:-0} != 1 ]]; then
    printf 'clean-uf-workflows: listing only; set PRUNE=1 to delete these records\n'
    exit 0
fi

for directory in "${stale[@]}"; do
    parent=${directory%/*}
    # Recheck confinement and Git immediately before deletion. Never traverse a
    # symlinked ancestor or reinterpret file names as shell words/glob patterns.
    [[ ${parent%/*} == .uf/workflows && -d $directory &&
        ! -L .uf && ! -L .uf/workflows && ! -L $parent && ! -L $directory ]] ||
        fail 'workflow path changed or escaped; refusing to prune'
    check_tracked
    rm -rf -- "$directory"
done
printf 'clean-uf-workflows: pruned %s record(s)\n' "${#stale[@]}"
