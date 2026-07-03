// SPDX-License-Identifier: Apache-2.0

package results

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/complytime/complyctl/pkg/provider"
)

const maxFieldSize = 10 * 1024 // 10KB per field

// conftestCheckResult mirrors conftest's CheckResult JSON output.
type conftestCheckResult struct {
	Filename   string           `json:"filename"`
	Namespace  string           `json:"namespace"`
	Successes  int              `json:"successes"`
	Warnings   []conftestResult `json:"warnings,omitempty"`
	Failures   []conftestResult `json:"failures,omitempty"`
	Exceptions []conftestResult `json:"exceptions,omitempty"`
}

// conftestResult mirrors conftest's Result JSON output.
type conftestResult struct {
	Message  string         `json:"msg"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PerTargetResult holds scan findings for a single target evaluation.
type PerTargetResult struct {
	Target       string    `json:"target"`
	Branch       string    `json:"branch,omitempty"`
	ScannedAt    time.Time `json:"scanned_at"`
	Findings     []Finding `json:"findings"`
	SuccessCount int       `json:"success_count"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
}

// Finding represents an individual policy violation.
type Finding struct {
	RequirementID string `json:"requirement_id"`
	Title         string `json:"title"`
	Result        string `json:"result"`
	Reason        string `json:"reason"`
	Filename      string `json:"filename"`
}

// ParseConftestOutput unmarshals conftest JSON output and creates findings
// from failures and warnings.
func ParseConftestOutput(raw []byte, target, branch string) (*PerTargetResult, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty conftest output")
	}

	var checkResults []conftestCheckResult
	if err := json.Unmarshal(raw, &checkResults); err != nil {
		return nil, fmt.Errorf("parsing conftest JSON output: %w", err)
	}

	result := &PerTargetResult{
		Target:    target,
		Branch:    branch,
		ScannedAt: time.Now(),
		Status:    "scanned",
	}

	for _, cr := range checkResults {
		result.SuccessCount += cr.Successes

		for _, f := range cr.Failures {
			result.Findings = append(result.Findings, buildFinding(f, cr.Filename, cr.Namespace))
		}
		for _, w := range cr.Warnings {
			result.Findings = append(result.Findings, buildFinding(w, cr.Filename, cr.Namespace))
		}
	}

	return result, nil
}

func buildFinding(cr conftestResult, filename, namespace string) Finding {
	reqID := extractRequirementID(cr.Metadata, namespace)
	reason := truncateField(stripControlChars(cr.Message), maxFieldSize)
	return Finding{
		RequirementID: reqID,
		Title:         reason,
		Result:        "fail",
		Reason:        reason,
		Filename:      filename,
	}
}

// extractRequirementID derives a Rego-namespace-based ID from conftest result
// metadata or the enclosing namespace. It prefers metadata["query"] (set by
// conftest for structured results) and falls back to the CheckResult namespace
// which corresponds to the Rego package path (e.g., "ci.action_pinning").
// The returned ID is Rego-derived and may be resolved to a Gemara requirement
// ID later via ResolveRequirementID.
func extractRequirementID(metadata map[string]any, namespace string) string {
	if metadata != nil {
		if query, ok := metadata["query"].(string); ok && query != "" {
			return deriveIDFromQuery(query)
		}
	}
	if namespace != "" {
		return namespace
	}
	return "unknown"
}

// deriveIDFromQuery parses a conftest query field to extract a requirement ID.
// Example: "data.docker.network_encryption.warn" → "docker.network_encryption"
func deriveIDFromQuery(query string) string {
	if query == "" {
		return "unknown"
	}

	parts := strings.Split(query, ".")

	// Strip "data." prefix
	if len(parts) > 0 && parts[0] == "data" {
		parts = parts[1:]
	}

	// Strip rule type suffix (warn, deny, violation)
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if last == "warn" || last == "deny" || last == "violation" {
			parts = parts[:len(parts)-1]
		}
	}

	if len(parts) == 0 {
		return "unknown"
	}

	return strings.Join(parts, ".")
}

// WritePerTargetResult writes a PerTargetResult as JSON to the given directory.
func WritePerTargetResult(result *PerTargetResult, dir string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating results directory: %w", err)
	}

	sanitized := sanitizeName(result.Target)
	if result.Branch != "" {
		sanitized += "-" + result.Branch
	}
	filename := sanitized + ".json"
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling per-target result: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing per-target result: %w", err)
	}

	return nil
}

// ResolveRequirementID resolves a Rego-derived ID (e.g., "kubernetes.run_as_root")
// to the corresponding Gemara requirement ID (e.g., "CIS-K8S-5.2.6") using
// the reverse mapping built by MatchRequirements. If the mapping does not
// contain the derived ID, the original ID is returned unchanged.
func ResolveRequirementID(derivedID string, reverseMap map[string]string) string {
	if gemaraID, ok := reverseMap[derivedID]; ok {
		return gemaraID
	}
	return derivedID
}

// ToScanResponse maps a slice of PerTargetResults to a provider.ScanResponse.
// Findings are grouped by Gemara requirement ID into AssessmentLog entries.
// Each target/branch scan becomes a Step within the assessment. Operational
// errors (targets with Status "error" and no findings) are placed into
// resp.Errors. When reverseMap is non-nil, Rego-derived IDs (e.g.,
// "kubernetes.run_as_root") are resolved to Gemara requirement IDs (e.g.,
// "CIS-K8S-5.2.6") before grouping.
func ToScanResponse(targetResults []*PerTargetResult, reverseMap map[string]string) *provider.ScanResponse {
	type reqGroup struct {
		requirementID string
		steps         []provider.Step
		passCount     int
		totalCount    int
	}

	groups := make(map[string]*reqGroup)
	var order []string
	var opErrors []string

	for _, tr := range targetResults {
		stepName := tr.Target
		if tr.Branch != "" {
			stepName += "@" + tr.Branch
		}

		for _, f := range tr.Findings {
			reqID := ResolveRequirementID(f.RequirementID, reverseMap)
			g, ok := groups[reqID]
			if !ok {
				g = &reqGroup{requirementID: reqID}
				groups[reqID] = g
				order = append(order, reqID)
			}
			result := mapResult(f.Result, tr.Status)
			g.steps = append(g.steps, provider.Step{
				Name:    stepName,
				Result:  result,
				Message: f.Reason,
			})
			g.totalCount++
			if result == provider.ResultPassed {
				g.passCount++
			}
		}

		if tr.Status == "error" && len(tr.Findings) == 0 {
			opErrors = append(opErrors, fmt.Sprintf("%s: %s", stepName, tr.Error))
		}
	}

	// Synthesize passing assessments for requirement IDs that had no findings.
	// The reverseMap values are the Gemara requirement IDs from the assessment
	// configurations sent during Generate; every requirement ID should appear
	// in the response so complyctl can correlate results. Each synthetic
	// assessment includes a passing step with the target name so evaluation
	// logs show a meaningful step identity.
	syntheticSteps := buildSyntheticSteps(targetResults)
	for _, reqID := range reverseMap {
		if _, exists := groups[reqID]; !exists {
			groups[reqID] = &reqGroup{
				requirementID: reqID,
				steps:         syntheticSteps,
				passCount:     len(syntheticSteps),
				totalCount:    len(syntheticSteps),
			}
			order = append(order, reqID)
		}
	}

	// Sort for deterministic output
	sort.Strings(order)
	// Deduplicate after sorting (order may have duplicates if same reqID from multiple targets)
	seen := make(map[string]bool, len(order))
	dedupOrder := make([]string, 0, len(order))
	for _, id := range order {
		if !seen[id] {
			seen[id] = true
			dedupOrder = append(dedupOrder, id)
		}
	}

	assessments := make([]provider.AssessmentLog, 0, len(groups))
	for _, reqID := range dedupOrder {
		g := groups[reqID]
		var msg string
		if g.totalCount == 0 {
			msg = "all checks passed"
		} else {
			msg = fmt.Sprintf("%d of %d targets passed", g.passCount, g.totalCount)
			for _, s := range g.steps {
				if s.Result != provider.ResultPassed {
					msg = s.Message
					break
				}
			}
		}
		assessments = append(assessments, provider.AssessmentLog{
			RequirementID: g.requirementID,
			Steps:         g.steps,
			Message:       msg,
			Confidence:    provider.ConfidenceLevelHigh,
		})
	}

	return &provider.ScanResponse{Assessments: assessments, Errors: opErrors}
}

// buildSyntheticSteps creates a passing step for each scanned target so that
// synthetic assessments (requirement IDs with no findings) have step identity in the
// evaluation log. Without this, the evaluation log shows steps: [] for
// requirements that passed all checks.
func buildSyntheticSteps(targetResults []*PerTargetResult) []provider.Step {
	steps := make([]provider.Step, 0, len(targetResults))
	for _, tr := range targetResults {
		if tr.Status == "error" {
			continue
		}
		stepName := tr.Target
		if tr.Branch != "" {
			stepName += "@" + tr.Branch
		}
		steps = append(steps, provider.Step{
			Name:    stepName,
			Result:  provider.ResultPassed,
			Message: "all checks passed",
		})
	}
	return steps
}

func mapResult(findingResult, targetStatus string) provider.Result {
	if targetStatus == "error" {
		return provider.ResultError
	}
	switch findingResult {
	case "pass":
		return provider.ResultPassed
	case "fail":
		return provider.ResultFailed
	default:
		return provider.ResultError
	}
}

func stripControlChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

func truncateField(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "[truncated]"
}

func sanitizeName(name string) string {
	var result []rune
	for _, r := range name {
		if r == '/' || r == '.' || r == ':' || r == ' ' {
			result = append(result, '-')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
