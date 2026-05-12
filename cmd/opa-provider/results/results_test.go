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

	resp := ToScanResponse(results)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "docker.network_encryption", resp.Assessments[0].RequirementID)
	assert.Len(t, resp.Assessments[0].Steps, 2)
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

	resp := ToScanResponse(results)
	require.Len(t, resp.Assessments, 1)
	assert.Equal(t, "scan-error", resp.Assessments[0].RequirementID)
	require.Len(t, resp.Assessments[0].Steps, 1)
	assert.Equal(t, provider.ResultError, resp.Assessments[0].Steps[0].Result)
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

	resp1 := ToScanResponse(results)
	resp2 := ToScanResponse(results)
	require.Equal(t, len(resp1.Assessments), len(resp2.Assessments))
	for i := range resp1.Assessments {
		assert.Equal(t, resp1.Assessments[i].RequirementID, resp2.Assessments[i].RequirementID)
	}
}

func TestToScanResponse_Empty(t *testing.T) {
	resp := ToScanResponse(nil)
	assert.Empty(t, resp.Assessments)
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
