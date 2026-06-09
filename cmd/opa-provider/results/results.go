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

// extractRequirementID derives a requirement ID from conftest result metadata
// or the enclosing namespace. It prefers metadata["query"] (set by conftest
// for structured results) and falls back to the CheckResult namespace which
// corresponds to the Rego package path (e.g. "ci.action_pinning").
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

// ResolveRequirementID resolves a Rego-derived requirement ID to a Gemara
// requirement ID using the reverse mapping. If the mapping does not contain
// the derived ID, the original ID is returned unchanged.
func ResolveRequirementID(derivedID string, reverseMap map[string]string) string {
	if gemaraID, ok := reverseMap[derivedID]; ok {
		return gemaraID
	}
	return derivedID
}

// ToScanResponse maps a slice of PerTargetResults to a provider.ScanResponse.
// Findings are grouped by requirement ID into AssessmentLog entries. Each
// target/branch scan becomes a Step within the assessment. Operational errors
// (targets with Status "error" and no findings) are placed into resp.Errors.
// When reverseMap is non-nil, Rego-derived requirement IDs are resolved to
// Gemara requirement IDs before grouping.
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

	// Synthesize passing assessments for requirements in the reverse map
	// that have no findings. The Gemara EvaluationLog CUE schema requires
	// at least one step per assessment, so requirements where all checks
	// passed (zero findings from conftest) need explicit passing entries.
	if reverseMap != nil {
		for _, gemaraID := range sortedValues(reverseMap) {
			if _, exists := groups[gemaraID]; exists {
				continue
			}
			g := &reqGroup{requirementID: gemaraID}
			for _, tr := range targetResults {
				if tr.Status != "scanned" {
					continue
				}
				stepName := tr.Target
				if tr.Branch != "" {
					stepName += "@" + tr.Branch
				}
				g.steps = append(g.steps, provider.Step{
					Name:    stepName,
					Result:  provider.ResultPassed,
					Message: "all checks passed",
				})
				g.totalCount++
				g.passCount++
			}
			groups[gemaraID] = g
			order = append(order, gemaraID)
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
		msg := fmt.Sprintf("%d of %d targets passed", g.passCount, g.totalCount)
		for _, s := range g.steps {
			if s.Result != provider.ResultPassed {
				msg = s.Message
				break
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

// ScanStatusAssessment returns a synthetic AssessmentLog reporting overall
// scan health across all targets.
func ScanStatusAssessment(targetResults []*PerTargetResult) provider.AssessmentLog {
	successCount := 0
	var steps []provider.Step

	for _, tr := range targetResults {
		stepName := tr.Target
		if tr.Branch != "" {
			stepName += "@" + tr.Branch
		}

		if tr.Status == "scanned" {
			successCount++
			steps = append(steps, provider.Step{
				Name:    stepName,
				Result:  provider.ResultPassed,
				Message: "scanned successfully",
			})
		} else {
			steps = append(steps, provider.Step{
				Name:    stepName,
				Result:  provider.ResultFailed,
				Message: tr.Error,
			})
		}
	}

	total := len(targetResults)
	var message string
	if successCount == total {
		message = fmt.Sprintf("all %d targets scanned successfully", total)
	} else {
		message = fmt.Sprintf("%d of %d targets scanned successfully", successCount, total)
	}

	return provider.AssessmentLog{
		RequirementID: "scan-status",
		Steps:         steps,
		Message:       message,
		Confidence:    provider.ConfidenceLevelHigh,
	}
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

// sortedValues returns the unique values of a string map in sorted order.
// Used to iterate reverse map values deterministically.
func sortedValues(m map[string]string) []string {
	seen := make(map[string]bool, len(m))
	vals := make([]string, 0, len(m))
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			vals = append(vals, v)
		}
	}
	sort.Strings(vals)
	return vals
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
