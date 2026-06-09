// SPDX-License-Identifier: Apache-2.0

package results

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

var happyPathFixture = `[
  {
    "filename": "deployment.yaml",
    "namespace": "main",
    "successes": 3,
    "failures": [
      {
        "msg": "Container must not run as root",
        "metadata": {
          "query": "data.kubernetes.run_as_root.deny"
        }
      }
    ],
    "warnings": [
      {
        "msg": "Resource limits should be set",
        "metadata": {
          "query": "data.kubernetes.resource_limits.warn"
        }
      }
    ]
  }
]`

var allSuccessFixture = `[
  {
    "filename": "deployment.yaml",
    "namespace": "main",
    "successes": 5
  }
]`

var multiNamespaceFixture = `[
  {
    "filename": "deployment.yaml",
    "namespace": "kubernetes.run_as_root",
    "successes": 1,
    "failures": [{"msg": "violation", "metadata": {"query": "data.kubernetes.run_as_root.deny"}}]
  },
  {
    "filename": "deployment.yaml",
    "namespace": "kubernetes.resource_limits",
    "successes": 2,
    "warnings": [{"msg": "warning", "metadata": {"query": "data.kubernetes.resource_limits.warn"}}]
  }
]`

func TestParseConftestOutput_HappyPath(t *testing.T) {
	result, err := ParseConftestOutput([]byte(happyPathFixture), "org/repo", "main")
	require.NoError(t, err)
	assert.Equal(t, "org/repo", result.Target)
	assert.Equal(t, "main", result.Branch)
	assert.Len(t, result.Findings, 2)
	for _, f := range result.Findings {
		assert.Equal(t, "fail", f.Result)
	}
}

func TestParseConftestOutput_WithMetadata(t *testing.T) {
	result, err := ParseConftestOutput([]byte(happyPathFixture), "org/repo", "main")
	require.NoError(t, err)
	require.Len(t, result.Findings, 2)

	ids := map[string]bool{}
	for _, f := range result.Findings {
		ids[f.RequirementID] = true
	}
	assert.True(t, ids["kubernetes.run_as_root"])
	assert.True(t, ids["kubernetes.resource_limits"])
}

func TestParseConftestOutput_NoMetadata(t *testing.T) {
	fixture := `[{"filename":"test.yaml","namespace":"main","successes":0,
		"failures":[{"msg":"bad config"}]}]`
	result, err := ParseConftestOutput([]byte(fixture), "target", "")
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.NotEmpty(t, result.Findings[0].RequirementID)
}

func TestParseConftestOutput_WarnAndDeny(t *testing.T) {
	result, err := ParseConftestOutput([]byte(happyPathFixture), "org/repo", "main")
	require.NoError(t, err)
	for _, f := range result.Findings {
		assert.Equal(t, "fail", f.Result, "both warn and deny should map to fail")
	}
}

func TestParseConftestOutput_SuccessesCount(t *testing.T) {
	result, err := ParseConftestOutput([]byte(allSuccessFixture), "org/repo", "main")
	require.NoError(t, err)
	assert.Equal(t, 5, result.SuccessCount)
	assert.Empty(t, result.Findings)
}

func TestParseConftestOutput_MultipleNamespaces(t *testing.T) {
	result, err := ParseConftestOutput([]byte(multiNamespaceFixture), "org/repo", "main")
	require.NoError(t, err)
	assert.Len(t, result.Findings, 2)
	assert.Equal(t, 3, result.SuccessCount)
}

func TestParseConftestOutput_EmptyInput(t *testing.T) {
	_, err := ParseConftestOutput(nil, "target", "")
	assert.Error(t, err)

	_, err = ParseConftestOutput([]byte{}, "target", "")
	assert.Error(t, err)
}

func TestParseConftestOutput_InvalidJSON(t *testing.T) {
	_, err := ParseConftestOutput([]byte("not json"), "target", "")
	assert.Error(t, err)
}

func TestParseConftestOutput_FieldSizeLimit(t *testing.T) {
	longMsg := strings.Repeat("x", maxFieldSize+100)
	fixture := `[{"filename":"test.yaml","namespace":"main","successes":0,
		"failures":[{"msg":"` + longMsg + `","metadata":{"query":"data.main.deny"}}]}]`

	result, err := ParseConftestOutput([]byte(fixture), "target", "")
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.True(t, strings.HasSuffix(result.Findings[0].Reason, "[truncated]"))
	assert.LessOrEqual(t, len(result.Findings[0].Reason), maxFieldSize+len("[truncated]"))
}

func TestParseConftestOutput_ControlChars(t *testing.T) {
	// Build JSON with control characters injected into the message
	msg := "bad" + string([]byte{0x07}) + "config" + string([]byte{0x08}) + "here"
	cr := []conftestCheckResult{{
		Filename:  "test.yaml",
		Namespace: "main",
		Successes: 0,
		Failures:  []conftestResult{{Message: msg, Metadata: map[string]any{"query": "data.main.deny"}}},
	}}
	fixture, err := json.Marshal(cr)
	require.NoError(t, err)

	result, err := ParseConftestOutput(fixture, "target", "")
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.NotContains(t, result.Findings[0].Reason, string([]byte{0x07}))
	assert.NotContains(t, result.Findings[0].Reason, string([]byte{0x08}))
	assert.Contains(t, result.Findings[0].Reason, "badconfighere")
}

func TestDeriveIDFromQuery(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"data.docker.network_encryption.warn", "docker.network_encryption"},
		{"data.kubernetes.run_as_root.deny", "kubernetes.run_as_root"},
		{"data.main.deny", "main"},
		{"data.main.warn", "main"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			assert.Equal(t, tc.expected, deriveIDFromQuery(tc.query))
		})
	}
}

func TestDeriveIDFromQuery_ShortQuery(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"data.main", "main"},
		{"main", "main"},
		{"", "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			assert.Equal(t, tc.expected, deriveIDFromQuery(tc.query))
		})
	}
}

func TestToScanResponse_Aggregation(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "org/repo",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "docker.network_encryption", Result: "fail", Reason: "fail1"},
			},
		},
		{
			Target: "org/repo2",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "docker.network_encryption", Result: "fail", Reason: "fail2"},
			},
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "docker.network_encryption", resp.Assessments[0].RequirementID)
	assert.Len(t, resp.Assessments[0].Steps, 2)
	assert.Empty(t, resp.Errors)
}

func TestToScanResponse_ErrorTargets(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "org/repo",
			Branch: "main",
			Status: "error",
			Error:  "clone failed",
		},
	}

	resp := ToScanResponse(results, nil)
	assert.Empty(t, resp.Assessments, "operational errors should not produce assessments")
	require.Len(t, resp.Errors, 1)
	assert.Contains(t, resp.Errors[0], "clone failed")
	assert.Contains(t, resp.Errors[0], "org/repo@main")
}

func TestToScanResponse_ErrorTargetWithFindings(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "org/repo",
			Branch: "main",
			Status: "error",
			Error:  "partial failure",
			Findings: []Finding{
				{RequirementID: "docker.network_encryption", Result: "fail", Reason: "violation"},
			},
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "docker.network_encryption", resp.Assessments[0].RequirementID)
	require.Len(t, resp.Assessments[0].Steps, 1)
	assert.Equal(t, provider.ResultError, resp.Assessments[0].Steps[0].Result,
		"findings on error targets should map to ResultError via mapResult")
	assert.Empty(t, resp.Errors,
		"error targets with findings should not duplicate into resp.Errors")
}

func TestToScanResponse_DeterministicOrder(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "target1",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "b.rule", Result: "fail", Reason: "b"},
				{RequirementID: "a.rule", Result: "fail", Reason: "a"},
			},
		},
	}

	resp1 := ToScanResponse(results, nil)
	resp2 := ToScanResponse(results, nil)
	require.Equal(t, len(resp1.Assessments), len(resp2.Assessments))
	for i := range resp1.Assessments {
		assert.Equal(t, resp1.Assessments[i].RequirementID, resp2.Assessments[i].RequirementID)
	}
	assert.Empty(t, resp1.Errors)
}

func TestToScanResponse_Empty(t *testing.T) {
	resp := ToScanResponse(nil, nil)
	assert.Empty(t, resp.Assessments)
	assert.Empty(t, resp.Errors)
}

func TestScanStatusAssessment_AllPassed(t *testing.T) {
	results := []*PerTargetResult{
		{Target: "org/repo", Branch: "main", Status: "scanned"},
		{Target: "org/repo2", Branch: "main", Status: "scanned"},
	}
	assessment := ScanStatusAssessment(results)
	assert.Equal(t, "scan-status", assessment.RequirementID)
	assert.Len(t, assessment.Steps, 2)
	for _, step := range assessment.Steps {
		assert.Equal(t, provider.ResultPassed, step.Result)
	}
	assert.Contains(t, assessment.Message, "all 2 targets scanned")
}

func TestScanStatusAssessment_PartialFailure(t *testing.T) {
	results := []*PerTargetResult{
		{Target: "org/repo", Branch: "main", Status: "scanned"},
		{Target: "org/repo2", Branch: "main", Status: "error", Error: "clone failed"},
	}
	assessment := ScanStatusAssessment(results)
	assert.Equal(t, "scan-status", assessment.RequirementID)
	assert.Len(t, assessment.Steps, 2)

	passCount := 0
	failCount := 0
	for _, step := range assessment.Steps {
		if step.Result == provider.ResultPassed {
			passCount++
		} else {
			failCount++
		}
	}
	assert.Equal(t, 1, passCount)
	assert.Equal(t, 1, failCount)
	assert.Contains(t, assessment.Message, "1 of 2 targets scanned")
}

func TestScanStatusAssessment_AllErrors(t *testing.T) {
	results := []*PerTargetResult{
		{Target: "org/repo", Branch: "main", Status: "error", Error: "fail1"},
		{Target: "org/repo2", Branch: "main", Status: "error", Error: "fail2"},
	}
	assessment := ScanStatusAssessment(results)
	for _, step := range assessment.Steps {
		assert.Equal(t, provider.ResultFailed, step.Result)
	}
}

func TestScanStatusAssessment_Empty(t *testing.T) {
	assessment := ScanStatusAssessment(nil)
	assert.Equal(t, "scan-status", assessment.RequirementID)
	assert.Len(t, assessment.Steps, 0)
	assert.Contains(t, assessment.Message, "all 0 targets scanned")
}

func TestScanStatusAssessment_OnlyTargetSteps(t *testing.T) {
	results := []*PerTargetResult{
		{Target: "org/repo", Branch: "main", Status: "scanned"},
		{Target: "org/repo2", Branch: "main", Status: "scanned"},
	}
	assessment := ScanStatusAssessment(results)

	require.Len(t, assessment.Steps, 2)
	assert.Equal(t, "org/repo@main", assessment.Steps[0].Name)
	assert.Equal(t, "org/repo2@main", assessment.Steps[1].Name)
	assert.Contains(t, assessment.Message, "all 2 targets scanned successfully")
}

func TestWritePerTargetResult(t *testing.T) {
	dir := t.TempDir()
	result := &PerTargetResult{
		Target:       "org/repo",
		Branch:       "main",
		Status:       "scanned",
		SuccessCount: 3,
		Findings: []Finding{
			{RequirementID: "test.rule", Result: "fail", Reason: "bad"},
		},
	}

	err := WritePerTargetResult(result, dir)
	require.NoError(t, err)

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	data, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	require.NoError(t, err)

	var parsed PerTargetResult
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "org/repo", parsed.Target)
	assert.Len(t, parsed.Findings, 1)

	info, err := os.Stat(filepath.Join(dir, files[0].Name()))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestResolveRequirementID_MappingHit(t *testing.T) {
	reverseMap := map[string]string{
		"kubernetes.run_as_root": "CIS-K8S-5.2.6",
	}
	result := ResolveRequirementID("kubernetes.run_as_root", reverseMap)
	assert.Equal(t, "CIS-K8S-5.2.6", result)
}

func TestResolveRequirementID_MappingMiss(t *testing.T) {
	reverseMap := map[string]string{
		"kubernetes.run_as_root": "CIS-K8S-5.2.6",
	}
	result := ResolveRequirementID("docker.network_encryption", reverseMap)
	assert.Equal(t, "docker.network_encryption", result)
}

func TestResolveRequirementID_NilMapping(t *testing.T) {
	result := ResolveRequirementID("kubernetes.run_as_root", nil)
	assert.Equal(t, "kubernetes.run_as_root", result)
}

func TestParseConftestOutput_NamespaceFallback(t *testing.T) {
	fixture := `[
	  {"filename":"bad-workflow.yml","namespace":"ci.action_pinning","successes":0,
	   "failures":[{"msg":"job uses unpinned action"}]},
	  {"filename":"bad-workflow.yml","namespace":"ci.permissions","successes":1,
	   "failures":[{"msg":"workflow uses write-all"}]}
	]`
	result, err := ParseConftestOutput([]byte(fixture), ".github/workflows", "")
	require.NoError(t, err)
	require.Len(t, result.Findings, 2)
	assert.Equal(t, "ci.action_pinning", result.Findings[0].RequirementID)
	assert.Equal(t, "ci.permissions", result.Findings[1].RequirementID)
}

func TestToScanResponse_WithReverseMapping(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "target1",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "kubernetes.run_as_root", Result: "fail", Reason: "violation"},
			},
		},
	}
	reverseMap := map[string]string{
		"kubernetes.run_as_root": "CIS-K8S-5.2.6",
	}

	resp := ToScanResponse(results, reverseMap)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "CIS-K8S-5.2.6", resp.Assessments[0].RequirementID)
}

func TestToScanResponse_MessageUsesViolationText(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "org/repo",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "ci.cert_validity", Result: "fail", Reason: "cert validity exceeds 397 days"},
			},
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "cert validity exceeds 397 days", resp.Assessments[0].Message,
		"message should contain the violation text, not a generic pass-count summary")
}

func TestToScanResponse_PassingAssessmentsForNoFindings(t *testing.T) {
	targetResults := []*PerTargetResult{
		{Target: "org/repo1", Branch: "main", Status: "scanned"},
		{Target: "org/repo2", Branch: "main", Status: "scanned"},
	}
	reverseMap := map[string]string{
		"ci.action_pinning": "REQ-001",
		"ci.permissions":    "REQ-002",
	}

	resp := ToScanResponse(targetResults, reverseMap)

	require.Len(t, resp.Assessments, 2, "one assessment per mapped requirement")
	for _, a := range resp.Assessments {
		require.Len(t, a.Steps, 2, "one step per scanned target for %s", a.RequirementID)
		for _, s := range a.Steps {
			assert.Equal(t, provider.ResultPassed, s.Result)
			assert.Equal(t, "all checks passed", s.Message)
		}
		assert.Equal(t, provider.ConfidenceLevelHigh, a.Confidence)
	}
	assert.Empty(t, resp.Errors)
}

func TestToScanResponse_MixedFindingsAndPassingRequirements(t *testing.T) {
	targetResults := []*PerTargetResult{
		{
			Target: "org/repo",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "ci.action_pinning", Result: "fail", Reason: "unpinned action"},
			},
		},
	}
	reverseMap := map[string]string{
		"ci.action_pinning": "REQ-001",
		"ci.permissions":    "REQ-002",
	}

	resp := ToScanResponse(targetResults, reverseMap)

	require.Len(t, resp.Assessments, 2, "should have both failing and passing assessments")

	assessmentsByID := make(map[string]provider.AssessmentLog, len(resp.Assessments))
	for _, a := range resp.Assessments {
		assessmentsByID[a.RequirementID] = a
	}

	// REQ-001 has a finding -> should have a failing step
	failingAssessment, ok := assessmentsByID["REQ-001"]
	require.True(t, ok, "REQ-001 should be present")
	require.Len(t, failingAssessment.Steps, 1)
	assert.Equal(t, provider.ResultFailed, failingAssessment.Steps[0].Result)
	assert.Equal(t, "unpinned action", failingAssessment.Steps[0].Message)

	// REQ-002 has no findings -> should have a synthesized passing step
	passingAssessment, ok := assessmentsByID["REQ-002"]
	require.True(t, ok, "REQ-002 should be present")
	require.Len(t, passingAssessment.Steps, 1)
	assert.Equal(t, provider.ResultPassed, passingAssessment.Steps[0].Result)
	assert.Equal(t, "all checks passed", passingAssessment.Steps[0].Message)
}

func TestToScanResponse_PassingAssessmentsExcludeErrorTargets(t *testing.T) {
	targetResults := []*PerTargetResult{
		{Target: "org/repo1", Branch: "main", Status: "scanned"},
		{Target: "org/repo2", Branch: "main", Status: "error", Error: "clone failed"},
	}
	reverseMap := map[string]string{
		"ci.action_pinning": "REQ-001",
	}

	resp := ToScanResponse(targetResults, reverseMap)

	require.Len(t, resp.Assessments, 1)
	a := resp.Assessments[0]
	require.Len(t, a.Steps, 1, "only scanned targets should contribute steps")
	assert.Equal(t, "org/repo1@main", a.Steps[0].Name,
		"step should be from the scanned target, not the error target")
	assert.Equal(t, provider.ResultPassed, a.Steps[0].Result)
}

func TestToScanResponse_NoPassingAssessmentsWithoutReverseMap(t *testing.T) {
	targetResults := []*PerTargetResult{
		{Target: "org/repo", Branch: "main", Status: "scanned"},
	}

	resp := ToScanResponse(targetResults, nil)

	assert.Empty(t, resp.Assessments,
		"no assessments should be synthesized when reverseMap is nil")
	assert.Empty(t, resp.Errors)
}

func TestToScanResponse_MessageFallsBackWhenAllPass(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "org/repo",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "ci.action_pinning", Result: "pass", Reason: ""},
			},
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "1 of 1 targets passed", resp.Assessments[0].Message,
		"passing assessments should use the pass-count summary")
}
