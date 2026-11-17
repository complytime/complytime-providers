# SPDX-License-Identifier: Apache-2.0
# Static isolation check, not a proof that prompt instructions execute correctly.
# Unmarked legacy native commands are accepted, but cannot reference plugin tools.

BEGIN {
    tools = "(plan_review_dispatch|dispatch_review_runs|consolidate_dispatch|" \
        "finalize_review_dispatch|capture_review_diff|fetch_pr_review_context|" \
        "submit_review_findings|submit_lesson_proposal|normalize_model_name|" \
        "format_unleash_exit|dispatch_agent_run|resolve_base_ref|get_progress|" \
        "clear_progress|dispatch_status|redispatch_failed_runs|" \
        "discard_orphaned_submissions|build_review_prompts|read_assessment_fields|" \
        "acquire_sibling_evidence|prepare_lesson_learning|invoke_agent)"
    # "checkpoint" is also ordinary recovery prose; reject tool references/calls,
    # not phrases such as "checkpoint data".
    plugin_tool = "(^|[^[:alnum:]_])" tools "([^[:alnum:]_]|$)" \
        "|`checkpoint`|([Cc]all|[Ii]nvoke|[Rr]equire|[Uu]se)[[:space:]]+checkpoint"
}

function reject(message) {
    printf "%s:%d: %s\n", FILENAME, FNR, message > "/dev/stderr"
    failed = 1
}

function finish_file() {
    if (FILENAME == "") return
    if (section != "") reject("unclosed execution-path section")
    if (marked && (!plugin_seen || !native_seen))
        reject("path-marked command requires both plugin and native sections")
    if (!fork_seen || scaffold_seen)
        reject("missing repository fork marker or restored scaffold marker")
}

FNR == 1 {
    # Make invokes this checker once per file so EOF diagnostics name that file.
    section = ""
    marked = plugin_seen = native_seen = fork_seen = scaffold_seen = 0
}

/<!-- repo-owned fork/ { fork_seen = 1 }
/<!-- scaffolded by uf/ { scaffold_seen = 1 }

/uf-path:/ {
    if ($0 !~ /^<!-- uf-path: (plugin|native) (begin|end) -->$/) {
        reject("malformed execution-path marker")
        next
    }
    path = $3
    action = $4
    marked = 1
    if (action == "begin") {
        if (section != "") reject("nested execution-path sections are forbidden")
        section = path
        if (path == "plugin") plugin_seen = 1
        else native_seen = 1
    } else {
        if (section != path) reject("unmatched execution-path end marker")
        section = ""
    }
    next
}

section != "plugin" && $0 ~ plugin_tool {
    reject("UF plugin tool reference outside plugin section")
}

section != "plugin" && /dispatch-advisor/ {
    reject("plugin-only dispatch-advisor skill outside plugin section")
}

END {
    finish_file()
    exit failed
}
