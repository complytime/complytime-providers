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

	"github.com/complytime/complytime-providers/internal/evidence"
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
	Target         string            `json:"target"`
	TargetURL      string            `json:"target_url,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	InputPath      string            `json:"input_path,omitempty"`
	InputDigest    string            `json:"input_digest,omitempty"`
	ScannedAt      time.Time         `json:"scanned_at"`
	Findings       []Finding         `json:"findings"`
	SuccessCount   int               `json:"success_count"`
	Status         string            `json:"status"`
	Error          string            `json:"error,omitempty"`
	Remarks        string            `json:"remarks,omitempty"`
	ReverseMapping map[string]string `json:"-"`
}

type reqGroup struct {
	requirementID string
	steps         []provider.Step
	passCount     int
	totalCount    int
	targets       []*PerTargetResult
	targetSeen    map[*PerTargetResult]bool
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
// resp.Errors. Rego-derived IDs are resolved with each result's reverse mapping;
// reverseMap supports callers that provide one legacy shared mapping. Each assessment
// log includes one Evidence entry linking back to the conftest input path that produced it.
// MappingReferences are built from unique target/branch combinations.
func ToScanResponse(
	targetResults []*PerTargetResult, reverseMap map[string]string,
) *provider.ScanResponse {
	groups := make(map[string]*reqGroup)
	var order []string
	var opErrors []string

	for _, tr := range targetResults {
		stepName := tr.Target
		if tr.Branch != "" {
			stepName += "@" + tr.Branch
		}

		for _, f := range tr.Findings {
			reqID := ResolveRequirementID(f.RequirementID, mappingForResult(tr, reverseMap))
			g, ok := groups[reqID]
			if !ok {
				g = &reqGroup{
					requirementID: reqID,
					targetSeen:    make(map[*PerTargetResult]bool),
				}
				groups[reqID] = g
				order = append(order, reqID)
			}

			// Track all contributing targets for evidence aggregation
			if !g.targetSeen[tr] {
				g.targets = append(g.targets, tr)
				g.targetSeen[tr] = true
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

	addSyntheticAssessments(groups, &order, targetResults, reverseMap)

	// Sort for deterministic output
	sort.Strings(order)
	// Deduplicate after sorting (order may have duplicates if same reqID
	// from multiple targets)
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
			msg = fmt.Sprintf(
				"%d of %d targets passed", g.passCount, g.totalCount,
			)
			for _, s := range g.steps {
				if s.Result != provider.ResultPassed {
					msg = s.Message
					break
				}
			}
		}
		al := provider.AssessmentLog{
			RequirementID: g.requirementID,
			Steps:         g.steps,
			Message:       msg,
			Confidence:    provider.ConfidenceLevelHigh,
		}
		// Collect evidence from all contributing targets
		var allEvidence []provider.Evidence
		for _, tr := range g.targets {
			allEvidence = append(allEvidence, buildEvidence(tr)...)
		}
		al.Evidence = allEvidence
		assessments = append(assessments, al)
	}

	mappingRefs := buildMappingReferences(targetResults)

	return &provider.ScanResponse{
		Assessments:       assessments,
		Errors:            opErrors,
		MappingReferences: mappingRefs,
	}
}

func mappingForResult(tr *PerTargetResult, fallback map[string]string) map[string]string {
	if tr.ReverseMapping != nil {
		return tr.ReverseMapping
	}
	return fallback
}

func addSyntheticAssessments(
	groups map[string]*reqGroup,
	order *[]string,
	targetResults []*PerTargetResult,
	fallbackMapping map[string]string,
) {
	for _, tr := range targetResults {
		if tr.Status == "error" {
			continue
		}

		foundRequirements := make(map[string]bool)
		for _, finding := range tr.Findings {
			foundRequirements[ResolveRequirementID(
				finding.RequirementID, mappingForResult(tr, fallbackMapping),
			)] = true
		}
		for _, requirementID := range mappingForResult(tr, fallbackMapping) {
			if foundRequirements[requirementID] {
				continue
			}
			group, ok := groups[requirementID]
			if !ok {
				group = &reqGroup{
					requirementID: requirementID,
					targetSeen:    make(map[*PerTargetResult]bool),
				}
				groups[requirementID] = group
				*order = append(*order, requirementID)
			}
			if group.targetSeen[tr] {
				continue
			}
			group.targets = append(group.targets, tr)
			group.targetSeen[tr] = true
			group.steps = append(group.steps, syntheticStep(tr))
			group.passCount++
			group.totalCount++
		}
	}
}

// buildEvidence creates a single Evidence entry for a target result.
// When the input is a regular file, InputDigest carries the SHA256
// hash computed by the server; for directories it is empty.
func buildEvidence(tr *PerTargetResult) []provider.Evidence {
	suffix := evidenceSuffix(tr)
	return []provider.Evidence{{
		ID:          evidence.IDPrefixConftest + suffix,
		Type:        evidence.TypeConftestResult,
		CollectedAt: tr.ScannedAt.UTC().Format(time.RFC3339),
		Source: &provider.EvidenceSource{
			ReferenceID: evidence.RefPrefixConftest + suffix,
			Coordinate:  tr.InputPath,
			Digest:      tr.InputDigest,
			Remarks:     tr.Remarks,
		},
	}}
}

// evidenceSuffix produces a human-readable ID suffix for a
// target result. For remote targets (TargetURL set), it strips
// the URL scheme and uses the full host+path identity. For local
// targets, it uses the target path directly. Components are
// joined via evidence.JoinForID with underscore separators to
// prevent ambiguity between repo identity and branch name.
func evidenceSuffix(tr *PerTargetResult) string {
	var identity string
	if tr.TargetURL != "" {
		identity = evidence.StripURLScheme(tr.TargetURL)
	} else {
		identity = tr.Target
	}
	if tr.Branch != "" {
		return evidence.JoinForID(identity, tr.Branch)
	}
	return evidence.SanitizeForID(identity)
}

// buildMappingReferences creates one MappingReference per unique
// target/branch combination from successfully scanned targets.
// The reference ID uses the same collision-resistant encoding as
// buildEvidence so that Evidence.Source.ReferenceID always matches
// a MappingReference.ID.
func buildMappingReferences(
	targetResults []*PerTargetResult,
) []provider.MappingReference {
	seen := make(map[string]bool)
	var refs []provider.MappingReference

	for _, tr := range targetResults {
		if tr.Status == "error" {
			continue
		}
		suffix := evidenceSuffix(tr)
		refID := evidence.RefPrefixConftest + suffix
		if seen[refID] {
			continue
		}
		seen[refID] = true

		title := "Conftest input: " + tr.Target
		if tr.Branch != "" {
			title += "@" + tr.Branch
		}
		refs = append(refs, provider.MappingReference{
			ID:          refID,
			Title:       title,
			Description: "Input path evaluated by conftest",
		})
	}

	sort.Slice(refs, func(i, j int) bool {
		return refs[i].ID < refs[j].ID
	})

	return refs
}

func syntheticStep(tr *PerTargetResult) provider.Step {
	stepName := tr.Target
	if tr.Branch != "" {
		stepName += "@" + tr.Branch
	}
	return provider.Step{
		Name:    stepName,
		Result:  provider.ResultPassed,
		Message: "all checks passed",
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
