# SPDX-License-Identifier: Apache-2.0
# Pure recovery decisions. The command validates live backend/artifact evidence
# before supplying it; this filter neither reads records nor performs actions.

include "uf-review-common";

def binding:
  type == "object"
  and keys == ["active_children", "agents_digest", "backend_ref", "command",
               "execution_path", "invocation_id", "policy_digest", "project",
               "receipts", "scope_digest", "target", "version"]
  and .version == 1
  and (.execution_path | path_name)
  and (.command | command_name)
  and (.invocation_id | nonempty)
  and (.project | nonempty)
  and (.target | nonempty)
  and (.backend_ref | nonempty)
  and (.scope_digest | digest)
  and (.policy_digest | digest)
  and (.agents_digest | digest)
  and (.active_children | type == "array" and all(.[]; nonempty))
  and (.receipts | type == "array" and all(.[];
    type == "object" and keys == ["id", "status"]
    and (.id | nonempty) and (.status | IN("confirmed", "ambiguous"))));

def current:
  type == "object"
  and keys == ["agents_digest", "command", "execution_path", "item_digest",
               "policy_digest", "project", "scope_digest", "target"]
  and (.execution_path | path_name)
  and (.command | command_name)
  and (.project | nonempty)
  and (.target | nonempty)
  and (.scope_digest | digest)
  and (.item_digest | digest)
  and (.policy_digest | digest)
  and (.agents_digest | digest);

def assessment:
  type == "object"
  and keys == ["agents_digest", "backend_ref", "execution_path", "item_digest",
               "policy_digest", "provenance", "scope_digest"]
  and (.execution_path | path_name)
  and (.backend_ref | nonempty)
  and (.scope_digest | digest)
  and (.item_digest | digest)
  and (.policy_digest | digest)
  and (.agents_digest | digest)
  and (if .execution_path == "plugin" then .provenance == "plugin-validated"
       else .provenance == "native-validated" end);

def council:
  type == "object"
  and keys == ["agents_digest", "execution_path", "policy_digest", "scope_digest", "verdict"]
  and (.execution_path | path_name)
  and (.scope_digest | digest)
  and (.policy_digest | digest)
  and (.agents_digest | digest)
  and (.verdict | IN("APPROVE", "APPROVE WITH ADVISORIES", "REQUEST CHANGES",
                    "INCONCLUSIVE", "UNAVAILABLE"));

def valid_input:
  type == "object"
  and keys == ["assessment", "binding", "council", "current", "event"]
  and (.binding | binding)
  and (.current | current)
  and (.assessment | . == null or assessment)
  and (.council | . == null or council)
  and (.event | IN("resume", "runtime_error", "policy_rejection",
                  "model_unavailable", "blocking_verdict", "post_fix_council"));

def decision($status; $reason; $superseded):
  {status: $status, reason: $reason, execution_path: .binding.execution_path,
   supersede_assessment: $superseded, backend_actions: [],
   mutation_authorized: false, fresh_consent_required: true};

def recover:
  if .binding.execution_path != .current.execution_path
     or .binding.command != .current.command
     or .binding.project != .current.project
     or .binding.target != .current.target then
    decision("stop"; "incompatible_binding"; false)
  elif (.binding.active_children | length) > 0
       or any(.binding.receipts[]; .status == "ambiguous") then
    decision("stop"; "reconciliation_required"; false)
  elif .event != "resume" and .event != "post_fix_council" then
    # A failure never re-enters fresh capability selection or authorizes replay.
    decision("stop"; .event; false)
  elif .event == "post_fix_council" then
    if .council == null then
      decision("council_required"; "no_post_fix_review"; false)
    elif .council.execution_path != .binding.execution_path
         or .council.scope_digest != .current.scope_digest
         or .council.policy_digest != .current.policy_digest
         or .council.agents_digest != .current.agents_digest then
      decision("stop"; "incompatible_council"; false)
    elif (.council.verdict | IN("APPROVE", "APPROVE WITH ADVISORIES")) then
      decision("confirmation_required"; "post_fix_review_passed"; false)
    else
      decision("stop"; "post_fix_review_not_passed"; false)
    end
  elif .assessment != null
       and .assessment.execution_path != .binding.execution_path then
    decision("stop"; "incompatible_assessment_backend"; false)
  elif .assessment == null then
    decision("reassess"; "assessment_reconstruction_required"; false)
  elif .binding.scope_digest != .current.scope_digest
       or .binding.policy_digest != .current.policy_digest
       or .binding.agents_digest != .current.agents_digest
       or .assessment.scope_digest != .current.scope_digest
       or .assessment.item_digest != .current.item_digest
       or .assessment.policy_digest != .current.policy_digest
       or .assessment.agents_digest != .current.agents_digest then
    decision("reassess"; "changed_assessment_inputs"; true)
  else
    decision("reuse"; "matching_validated_assessment"; false)
  end;

if length == 1 and (.[0] | valid_input) then
  .[0] | recover
else
  {status: "stop", reason: "invalid_or_legacy_record", execution_path: null,
   supersede_assessment: false, backend_actions: [], mutation_authorized: false,
   fresh_consent_required: true}
end
