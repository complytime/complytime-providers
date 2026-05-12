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
			result.Findings = append(result.Findings, buildFinding(f, cr.Filename))
		}
		for _, w := range cr.Warnings {
			result.Findings = append(result.Findings, buildFinding(w, cr.Filename))
		}
	}

	return result, nil
}

func buildFinding(cr conftestResult, filename string) Finding {
	reqID := extractRequirementID(cr.Metadata)
	reason := truncateField(stripControlChars(cr.Message), maxFieldSize)
	return Finding{
		RequirementID: reqID,
		Title:         reason,
		Result:        "fail",
		Reason:        reason,
		Filename:      filename,
	}
}

// extractRequirementID extracts a requirement ID from conftest result metadata.
func extractRequirementID(metadata map[string]any) string {
	if metadata == nil {
		return "unknown"
	}
	query, ok := metadata["query"].(string)
	if !ok || query == "" {
		return "unknown"
	}
	return deriveIDFromQuery(query)
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

// ToScanResponse maps a slice of PerTargetResults to a provider.ScanResponse.
func ToScanResponse(targetResults []*PerTargetResult) *provider.ScanResponse {
	type reqGroup struct {
		requirementID string
		steps         []provider.Step
	}

	groups := make(map[string]*reqGroup)
	var order []string

	for _, tr := range targetResults {
		stepName := tr.Target
		if tr.Branch != "" {
			stepName += "@" + tr.Branch
		}

		for _, f := range tr.Findings {
			g, ok := groups[f.RequirementID]
			if !ok {
				g = &reqGroup{requirementID: f.RequirementID}
				groups[f.RequirementID] = g
				order = append(order, f.RequirementID)
			}
			g.steps = append(g.steps, provider.Step{
				Name:    stepName,
				Result:  mapResult(f.Result),
				Message: f.Reason,
			})
		}

		if tr.Status == "error" && len(tr.Findings) == 0 {
			const errorReqID = "scan-error"
			g, ok := groups[errorReqID]
			if !ok {
				g = &reqGroup{requirementID: errorReqID}
				groups[errorReqID] = g
				order = append(order, errorReqID)
			}
			g.steps = append(g.steps, provider.Step{
				Name:    stepName,
				Result:  provider.ResultError,
				Message: tr.Error,
			})
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
		failCount := 0
		for _, step := range g.steps {
			if step.Result == provider.ResultFailed {
				failCount++
			}
		}
		assessments = append(assessments, provider.AssessmentLog{
			RequirementID: g.requirementID,
			Steps:         g.steps,
			Message:       fmt.Sprintf("%d violations across %d targets", failCount, len(g.steps)),
			Confidence:    provider.ConfidenceLevelHigh,
		})
	}

	return &provider.ScanResponse{Assessments: assessments}
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

func mapResult(findingResult string) provider.Result {
	switch findingResult {
	case "fail":
		return provider.ResultFailed
	case "pass":
		return provider.ResultPassed
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
