# SPDX-License-Identifier: Apache-2.0
# Pure decision over inspected capability evidence. No tool discovery or I/O.
# Use: jq -L .github/scripts -cs -f .github/scripts/uf-review-preflight.jq
include "uf-review-common";

def native_controls:
  ["delegation", "model_variant", "permissions", "deadline", "cancellation",
   "terminal_state", "caller_binding", "policy"];

def plugin_requirements($input):
  ["plan_review_dispatch", "dispatch_review_runs", "consolidate_dispatch",
   "submit_review_findings", "get_progress", "checkpoint", "clear_progress",
   "dispatch_status", "immutable_scope", "child_contract", "lifecycle", "policy"]
  + (if $input.command == "uf.review-pr" then ["finalize_review_dispatch"] else [] end)
  + (if ($input.command == "uf.review-council" and $input.local_input)
         or $input.command == "uf.address-feedback"
     then ["capture_review_diff"] else [] end)
  + (if $input.command == "uf.address-feedback"
     then ["fetch_pr_review_context", "normalize_model_name", "feedback_assessment"]
     else [] end)
  + (if $input.siblings_configured then ["frozen_sibling_evidence"] else [] end);

def capability:
  type == "object" and keys == ["evidence", "status"]
  and (.status | IN("compatible", "missing", "incompatible", "unknown"))
  and (.evidence | nonempty);

def capability_map:
  type == "object" and all(.[]; capability);

def valid_input:
  type == "object"
  and (keys - ["binding", "command", "discovery", "local_input", "native",
               "plugin", "runtime_version", "siblings_configured"] | length == 0)
  and (.command | command_name)
  and (.local_input | type == "boolean")
  and (.siblings_configured | type == "boolean")
  and (.runtime_version | type == "string")
  and (.discovery | IN("complete", "ambiguous"))
  and (.plugin | capability_map)
  and (.native | capability_map)
  # Binding evidence is checked by the command before calling this pure helper.
  # Missing/legacy/incompatible ownership is represented explicitly, never null.
  and (.binding | type == "object"
    and keys == ["execution_path", "status", "unreconciled"]
    and (.status | IN("fresh", "valid", "legacy", "incompatible"))
    and (.execution_path | . == null or IN("plugin", "native"))
    and (.unreconciled | type == "boolean")
    and (if .status == "valid" then .execution_path != null
         elif .status == "fresh" then .execution_path == null else true end));

def gaps($required; $observed):
  [$required[] as $name
   | ($observed[$name] // {status: "missing",
                           evidence: "absent from complete capability inventory"}) as $cap
   | select($cap.status != "compatible")
   | {capability: $name, status: $cap.status, evidence: $cap.evidence}];

def decision($status; $path; $reason; $plugin_gaps; $native_gaps):
  {status: $status, execution_path: $path, reason: $reason,
   plugin_gaps: $plugin_gaps, native_gaps: $native_gaps,
   backend_actions: [], mutation_authorized: false};

def select_path:
  . as $input
  | gaps(plugin_requirements($input); .plugin) as $plugin_gaps
  | gaps(native_controls; .native) as $native_gaps
  | if .binding.status == "legacy" or .binding.status == "incompatible" then
      decision("stop"; .binding.execution_path; "binding_reconciliation_required";
               $plugin_gaps; [])
    elif .binding.unreconciled then
      decision("stop"; .binding.execution_path; "active_children_or_receipts";
               $plugin_gaps; [])
    elif .discovery == "ambiguous" then
      decision("stop"; .binding.execution_path; "ambiguous_discovery"; []; [])
    elif .binding.status == "valid" then
      .binding.execution_path as $path
      | (if $path == "plugin" then $plugin_gaps else $native_gaps end) as $saved_gaps
      | if ($saved_gaps | length) == 0 then
          decision("ready"; $path; "resume_bound_backend"; $plugin_gaps;
                   (if $path == "native" then $native_gaps else [] end))
        else
          decision("stop"; $path; "selected_backend_unavailable"; $plugin_gaps;
                   (if $path == "native" then $native_gaps else [] end))
        end
    elif any($plugin_gaps[]; .status == "unknown") then
      decision("stop"; null; "ambiguous_plugin_contract"; $plugin_gaps; [])
    elif ($plugin_gaps | length) == 0 then
      decision("ready"; "plugin"; "complete_plugin_contract"; []; [])
    elif any($native_gaps[]; .status == "unknown") then
      decision("stop"; null; "ambiguous_native_contract"; $plugin_gaps; $native_gaps)
    elif ($native_gaps | length) == 0 then
      decision("ready"; "native"; "plugin_contract_unusable"; $plugin_gaps; [])
    else
      decision("unavailable"; null; "no_complete_backend"; $plugin_gaps; $native_gaps)
    end;

if length != 1 then
  decision("stop"; null; "invalid_input"; []; [])
elif (.[0] | valid_input) then
  .[0] | select_path
else
  decision("stop"; null; "invalid_input"; []; [])
end
