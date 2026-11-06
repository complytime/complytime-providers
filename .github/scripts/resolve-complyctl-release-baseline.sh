#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Resolve the stable complyctl release baseline for the declared Go dependency.

set -euo pipefail

readonly COMPLYCTL_MODULE='github.com/complytime/complyctl'
readonly COMPLYCTL_REPOSITORY='https://github.com/complytime/complyctl.git'
readonly GIT_TIMEOUT_SECONDS=60

die() {
    printf 'release-baseline: %s\n' "$*" >&2
    exit 1
}

usage() {
    printf 'Usage: %s [--module-file PATH] [--tags-file PATH]\n' "${0##*/}" >&2
}

module_file='go.mod'
tags_file=''

while (($# > 0)); do
    case "$1" in
        --module-file)
            (($# >= 2)) || die '--module-file requires a path'
            module_file=$2
            shift 2
            ;;
        --tags-file)
            (($# >= 2)) || die '--tags-file requires a path'
            tags_file=$2
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

[[ -r "$module_file" ]] || die "cannot read module file: $module_file"

# go mod edit provides the authoritative module parser, avoiding fragile parsing
# of require blocks and comments in go.mod.
dependency_version=$(go mod edit -json "$module_file" | awk -v module="$COMPLYCTL_MODULE" '
    function emit_version() {
        if (wanted && !indirect && version != "") {
            print version
            found = 1
            exit
        }
    }
    /"Path":/ {
        emit_version()
        path = $2
        gsub(/[",]/, "", path)
        wanted = path == module || path ~ ("^" module "/v[2-9][0-9]*$")
        indirect = 0
        version = ""
    }
    wanted && /"Version":/ {
        version = $2
        gsub(/[",]/, "", version)
    }
    wanted && /"Indirect": true/ { indirect = 1 }
    END { if (!found) emit_version() }
')

[[ -n "$dependency_version" ]] || die "missing direct $COMPLYCTL_MODULE dependency"

if [[ ! "$dependency_version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
    die "dependency version is not a stable semantic version: $dependency_version"
fi

dependency_major=${BASH_REMATCH[1]}
dependency_minor=${BASH_REMATCH[2]}
if [[ "$dependency_major" == '0' ]]; then
    compatibility_prefix="v0.${dependency_minor}."
else
    compatibility_prefix="v${dependency_major}."
fi

temporary_tags_file=''
cleanup() {
    [[ -z "$temporary_tags_file" ]] || rm -f "$temporary_tags_file"
}
trap cleanup EXIT

if [[ -z "$tags_file" ]]; then
    temporary_tags_file=$(mktemp)
    tags_file=$temporary_tags_file
    timeout "$GIT_TIMEOUT_SECONDS" git ls-remote --tags "$COMPLYCTL_REPOSITORY" > "$tags_file" || \
        die 'could not list complyctl release tags'
fi

[[ -r "$tags_file" ]] || die "cannot read tags file: $tags_file"

stable_tags=$(
    awk '{ print $2 }' "$tags_file" |
        while IFS= read -r reference; do
            tag=${reference#refs/tags/}
            [[ "$reference" == "refs/tags/$tag" ]] || continue
            [[ "$tag" == "$compatibility_prefix"* ]] || continue
            [[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || continue
            printf '%s\n' "$tag"
        done
)

[[ -n "$stable_tags" ]] || die "no stable complyctl release matches $compatibility_prefix"
selected_tag=$(printf '%s\n' "$stable_tags" | sort -V | tail -n 1)

# Prefer a peeled commit for annotated tags; a lightweight tag resolves directly.
selected_commit=$(awk -v tag="$selected_tag" '
    $2 == ("refs/tags/" tag) { direct = $1 }
    $2 == ("refs/tags/" tag "^{}") { peeled = $1 }
    END {
        if (peeled != "") {
            print peeled
        } else if (direct != "") {
            print direct
        }
    }
' "$tags_file")

[[ "$selected_commit" =~ ^[0-9a-fA-F]{40}$ ]] || \
    die "could not resolve commit for release tag: $selected_tag"

printf 'tag=%s\ncommit=%s\n' "$selected_tag" "$selected_commit"
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    printf 'tag=%s\ncommit=%s\n' "$selected_tag" "$selected_commit" >> "$GITHUB_OUTPUT"
fi
