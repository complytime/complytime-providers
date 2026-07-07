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
	"github.com/complytime/complytime-providers/cmd/ampel-provider/intoto"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/targets"
)

const maxFieldSize = 10 * 1024 // 10KB per field

// ampelResultStatement represents the in-toto attestation produced by ampel verify
// with --attest-results. The predicate contains the evaluation ResultSet.
type ampelResultStatement struct {
	Predicate ampelResultSetPred `json:"predicate"`
}

// ampelResultSetPred represents the ResultSet predicate from ampel verify.
type ampelResultSetPred struct {
	Status  string              `json:"status"`
	Results []ampelPolicyResult `json:"results"`
	Error   *ampelError         `json:"error,omitempty"`
}

// ampelPolicyResult represents a single policy evaluation result.
type ampelPolicyResult struct {
	Status      string            `json:"status"`
	Policy      ampelPolicyRef    `json:"policy"`
	EvalResults []ampelEvalResult `json:"eval_results"`
	Meta        ampelResultMeta   `json:"meta"`
}

// ampelPolicyRef identifies the policy that was evaluated.
type ampelPolicyRef struct {
	ID string `json:"id"`
}

// ampelEvalResult represents a single tenet evaluation result.
type ampelEvalResult struct {
	ID         string           `json:"id"`
	Status     string           `json:"status"`
	Assessment *ampelAssessment `json:"assessment,omitempty"`
	Error      *ampelError      `json:"error,omitempty"`
}

// ampelAssessment holds the message for a passing tenet.
type ampelAssessment struct {
	Message string `json:"message"`
}

// ampelError holds the message and guidance for a failing tenet or result set.
type ampelError struct {
	Message  string `json:"message"`
	Guidance string `json:"guidance"`
}

// ampelResultMeta holds metadata about a policy evaluation.
type ampelResultMeta struct {
	Description string `json:"description"`
}

// PerRepoResult holds scan findings for a single repository.
type PerRepoResult struct {
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	ScannedAt  time.Time `json:"scanned_at"`
	Findings   []Finding `json:"findings"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// Finding represents an individual rule evaluation result.
type Finding struct {
	TenetID  string `json:"tenet_id"`
	Title    string `json:"title"`
	Result   string `json:"result"`
	Reason   string `json:"reason"`
	Guidance string `json:"guidance,omitempty"`
}

// ParseAmpelOutput parses the in-toto attestation produced by ampel verify
// (with --attest-results) into a PerRepoResult. The attestation predicate
// contains the evaluation ResultSet with per-policy and per-tenet results.
func ParseAmpelOutput(raw []byte, repo, branch string) (*PerRepoResult, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty ampel verify output")
	}

	// Unwrap DSSE envelope if present (ampel --attest-results produces signed attestations)
	raw, err := intoto.UnwrapDSSE(raw)
	if err != nil {
		return nil, fmt.Errorf("unwrapping DSSE envelope in ampel result: %w", err)
	}

	var stmt ampelResultStatement
	if err := json.Unmarshal(raw, &stmt); err != nil {
		return nil, fmt.Errorf("parsing ampel verify attestation: %w", err)
	}

	result := &PerRepoResult{
		Repository: repo,
		Branch:     branch,
		ScannedAt:  time.Now(),
	}

	// Handle ResultSet-level error
	if stmt.Predicate.Error != nil && stmt.Predicate.Error.Message != "" {
		if len(stmt.Predicate.Error.Message) > maxFieldSize {
			return nil, fmt.Errorf("ampel output error field exceeds maximum size")
		}
		result.Status = "error"
		result.Error = stripControlChars(stmt.Predicate.Error.Message)
		return result, nil
	}

	// Extract findings from each policy result's tenet evaluations
	for _, policyResult := range stmt.Predicate.Results {
		policyID := stripControlChars(policyResult.Policy.ID)
		if !isPrintableASCII(policyID) {
			return nil, fmt.Errorf("policy ID %q contains non-printable characters", policyID)
		}

		description := stripControlChars(policyResult.Meta.Description)

		for _, er := range policyResult.EvalResults {
			checkID := "check-" + policyID
			if len(checkID) > maxFieldSize || len(description) > maxFieldSize {
				return nil, fmt.Errorf("field exceeds maximum size in policy %s", policyID)
			}

			finding := Finding{
				TenetID: checkID,
				Title:   description,
			}

			status := strings.ToUpper(er.Status)
			switch status {
			case "PASS":
				finding.Result = "pass"
				if er.Assessment != nil {
					finding.Reason = stripControlChars(er.Assessment.Message)
				}
			default:
				finding.Result = "fail"
				if er.Error != nil {
					finding.Reason = stripControlChars(er.Error.Message)
					guidance := stripControlChars(er.Error.Guidance)
					if len(guidance) > maxFieldSize {
						return nil, fmt.Errorf(
							"guidance field exceeds maximum size in policy %s",
							policyID,
						)
					}
					finding.Guidance = guidance
				}
			}

			result.Findings = append(result.Findings, finding)
		}
	}

	overallStatus := strings.ToUpper(stmt.Predicate.Status)
	if overallStatus == "PASS" {
		result.Status = "pass"
	} else {
		result.Status = "fail"
	}

	return result, nil
}

// WritePerRepoResult writes a PerRepoResult as JSON to the given directory.
func WritePerRepoResult(result *PerRepoResult, dir string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating results directory: %w", err)
	}

	filename := targets.SanitizeRepoURL(result.Repository) + "-" + result.Branch + ".json"
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling per-repo result: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing per-repo result: %w", err)
	}

	return nil
}

// ToScanResponse maps a slice of PerRepoResults to a provider.ScanResponse.
// Findings are grouped by Gemara requirement ID (derived from TenetID by
// stripping the "check-" prefix) into AssessmentLog entries. Each
// repository/branch scan becomes a Step within the assessment. Operational
// errors (repos with Status "error" and no findings) are placed into
// resp.Errors instead of synthetic assessments.
//
// When allRequirementIDs is non-nil, any requirement ID in the list that
// produced no findings is given a synthetic passing AssessmentLog with one
// step per scanned repository. This ensures every requirement evaluated
// during Generate appears in the response. When nil, only findings-derived
// assessments are returned (backward-compatible behavior).
func ToScanResponse(repoResults []*PerRepoResult, allRequirementIDs []string) *provider.ScanResponse {
	type reqGroup struct {
		requirementID string
		steps         []provider.Step
		guidances     []string
		passCount     int
		totalCount    int
	}

	groups := make(map[string]*reqGroup)
	var order []string // track insertion order for deterministic output
	var opErrors []string

	for _, rr := range repoResults {
		repoName := targets.RepoDisplayName(rr.Repository)
		stepName := repoName + "@" + rr.Branch

		for _, f := range rr.Findings {
			// Derive Gemara requirement ID from TenetID by stripping "check-" prefix
			reqID := strings.TrimPrefix(f.TenetID, "check-")

			g, ok := groups[reqID]
			if !ok {
				g = &reqGroup{requirementID: reqID}
				groups[reqID] = g
				order = append(order, reqID)
			}

			result := mapResult(f.Result, rr.Status)
			g.steps = append(g.steps, provider.Step{
				Name:    stepName,
				Result:  result,
				Message: f.Reason,
			})
			// Track guidance only from non-passing steps
			if result == provider.ResultPassed {
				g.guidances = append(g.guidances, "")
				g.passCount++
			} else {
				g.guidances = append(g.guidances, f.Guidance)
			}
			g.totalCount++
		}

		// Operational errors with no findings go to resp.Errors
		if rr.Status == "error" && len(rr.Findings) == 0 {
			opErrors = append(opErrors,
				fmt.Sprintf("%s: %s", stepName, rr.Error))
		}
	}

	// Synthesize passing assessments for requirement IDs that had no findings.
	// The allRequirementIDs are the Gemara requirement IDs from the assessment
	// configurations sent during Generate; every requirement ID should appear
	// in the response so complyctl can correlate results.
	if allRequirementIDs != nil {
		syntheticSteps := buildSyntheticSteps(repoResults)
		for _, reqID := range allRequirementIDs {
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
	}

	// Sort for deterministic output, consistent with the OPA provider.
	sort.Strings(order)

	assessments := make([]provider.AssessmentLog, 0, len(groups))
	for _, reqID := range order {
		g := groups[reqID]

		// Determine message and recommendation from steps.
		// Use the first non-passing step's message and guidance.
		// If all pass, use the first step's message (no recommendation).
		// Fall back to count string only when all messages are empty.
		var msg, recommendation string
		for i, s := range g.steps {
			if s.Result != provider.ResultPassed {
				msg = s.Message
				recommendation = g.guidances[i]
				break
			}
		}
		if msg == "" && len(g.steps) > 0 {
			msg = g.steps[0].Message
		}
		if msg == "" {
			msg = fmt.Sprintf(
				"%d of %d checks passed",
				g.passCount, g.totalCount,
			)
		}

		assessments = append(assessments, provider.AssessmentLog{
			RequirementID:  g.requirementID,
			Steps:          g.steps,
			Message:        msg,
			Recommendation: recommendation,
			Confidence:     provider.ConfidenceLevelHigh,
		})
	}

	return &provider.ScanResponse{Assessments: assessments, Errors: opErrors}
}

// buildSyntheticSteps creates a passing step for each scanned repository so
// that synthetic assessments (requirement IDs with no findings) have step
// identity in the evaluation log. Error repositories are excluded.
func buildSyntheticSteps(repoResults []*PerRepoResult) []provider.Step {
	steps := make([]provider.Step, 0, len(repoResults))
	for _, rr := range repoResults {
		if rr.Status == "error" {
			continue
		}
		repoName := targets.RepoDisplayName(rr.Repository)
		stepName := repoName + "@" + rr.Branch
		steps = append(steps, provider.Step{
			Name:    stepName,
			Result:  provider.ResultPassed,
			Message: "all checks passed",
		})
	}
	return steps
}

func mapResult(findingResult, repoStatus string) provider.Result {
	if repoStatus == "error" {
		return provider.ResultError
	}
	switch findingResult {
	case "pass":
		return provider.ResultPassed
	case "fail":
		return provider.ResultFailed
	case "skip":
		return provider.ResultSkipped
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

func isPrintableASCII(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			return false
		}
	}
	return true
}
