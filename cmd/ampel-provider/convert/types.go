// SPDX-License-Identifier: Apache-2.0

package convert

// AmpelPolicyBundle is the top-level document written to disk for ampel verify.
type AmpelPolicyBundle struct {
	ID       string         `json:"id"`
	Meta     BundleMeta     `json:"meta"`
	Policies []*AmpelPolicy `json:"policies"`
}

// BundleMeta holds metadata for the policy bundle.
type BundleMeta struct {
	Frameworks []Framework `json:"frameworks"`
}

// Framework identifies a compliance framework referenced by the bundle.
type Framework struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AmpelPolicy represents a single AMPEL policy (one per granular file).
type AmpelPolicy struct {
	ID     string       `json:"id"`
	Meta   PolicyMeta   `json:"meta"`
	When   *WhenClause  `json:"when,omitempty"`
	Tenets []AmpelTenet `json:"tenets"`
}

// WhenClause holds a CEL guard expression that controls whether a
// policy is evaluated. Ampel skips evaluation when the expression
// evaluates to false, producing a SKIP result instead. The pointer
// field with omitempty on AmpelPolicy avoids emitting the key for
// policies that have no guard.
type WhenClause struct {
	Expression string `json:"expression"`
}

// PolicyMeta holds metadata for an individual policy.
type PolicyMeta struct {
	Description string          `json:"description"`
	Controls    []PolicyControl `json:"controls"`
}

// PolicyControl references a compliance control associated with the policy.
type PolicyControl struct {
	Framework string `json:"framework"`
	Class     string `json:"class"`
	ID        string `json:"id"`
}

// AmpelTenet represents a single verification check within a policy.
type AmpelTenet struct {
	ID         string        `json:"id"`
	Code       string        `json:"code"`
	Predicates PredicateSpec `json:"predicates"`
	Assessment TenetMessage  `json:"assessment"`
	Error      TenetError    `json:"error"`
}

// PredicateSpec defines the attestation predicate types a tenet evaluates.
type PredicateSpec struct {
	Types []string `json:"types"`
}

// TenetMessage holds the assessment message for a passing tenet.
type TenetMessage struct {
	Message string `json:"message"`
}

// TenetError holds the error message and remediation guidance for a failing tenet.
type TenetError struct {
	Message  string `json:"message"`
	Guidance string `json:"guidance"`
}
