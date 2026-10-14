// SPDX-License-Identifier: Apache-2.0

package results

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"

	"github.com/complytime/complytime-providers/internal/evidence"
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

func TestToScanResponse_PassingRequirementsFromReverseMap(t *testing.T) {
	// When all checks pass for a requirement (no findings), the reverseMap
	// should still produce a passing assessment so complyctl can resolve it.
	// Synthetic assessments include a passing step per scanned target for
	// evaluation log step identity.
	results := []*PerTargetResult{
		{
			Target:       "target1",
			Branch:       "main",
			Status:       "scanned",
			SuccessCount: 3,
		},
	}
	reverseMap := map[string]string{
		"kubernetes.run_as_root":     "CIS-K8S-5.2.6",
		"kubernetes.resource_limits": "CIS-K8S-5.4.1",
	}

	resp := ToScanResponse(results, reverseMap)
	require.Len(t, resp.Assessments, 2)

	ids := make(map[string]bool)
	for _, a := range resp.Assessments {
		ids[a.RequirementID] = true
		require.Len(t, a.Steps, 1, "synthetic assessment should have one step per target")
		assert.Equal(t, "target1@main", a.Steps[0].Name)
		assert.Equal(t, provider.ResultPassed, a.Steps[0].Result)
	}
	assert.True(t, ids["CIS-K8S-5.2.6"])
	assert.True(t, ids["CIS-K8S-5.4.1"])
	assert.Empty(t, resp.Errors)
}

func TestToScanResponse_MixedPassAndFail(t *testing.T) {
	// One requirement has findings, another has none (all passed).
	results := []*PerTargetResult{
		{
			Target: "target1",
			Branch: "main",
			Status: "scanned",
			Findings: []Finding{
				{RequirementID: "kubernetes.run_as_root", Result: "fail", Reason: "violation"},
			},
			SuccessCount: 2,
		},
	}
	reverseMap := map[string]string{
		"kubernetes.run_as_root":     "CIS-K8S-5.2.6",
		"kubernetes.resource_limits": "CIS-K8S-5.4.1",
	}

	resp := ToScanResponse(results, reverseMap)
	require.Len(t, resp.Assessments, 2)

	byID := make(map[string]provider.AssessmentLog)
	for _, a := range resp.Assessments {
		byID[a.RequirementID] = a
	}

	// The failed requirement should have steps.
	failedAssessment := byID["CIS-K8S-5.2.6"]
	require.Len(t, failedAssessment.Steps, 1)
	assert.Equal(t, provider.ResultFailed, failedAssessment.Steps[0].Result)

	// The passing requirement should have a synthetic step with the target name.
	passedAssessment := byID["CIS-K8S-5.4.1"]
	require.Len(t, passedAssessment.Steps, 1)
	assert.Equal(t, "target1@main", passedAssessment.Steps[0].Name)
	assert.Equal(t, provider.ResultPassed, passedAssessment.Steps[0].Result)
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

func TestToScanResponse_EvidencePerAssessment(t *testing.T) {
	now := time.Now()
	results := []*PerTargetResult{
		{
			Target:    "org/repo",
			Branch:    "main",
			InputPath: "/tmp/workspace/repos/org-repo/main",
			ScannedAt: now,
			Status:    "scanned",
			Remarks:   "Collected on host \"testhost\" (machine-id: abc123)",
			Findings: []Finding{
				{
					RequirementID: "kubernetes.run_as_root",
					Result:        "fail",
					Reason:        "violation",
				},
			},
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.Assessments, 1)

	al := resp.Assessments[0]
	require.Len(t, al.Evidence, 1,
		"each assessment log should have one evidence entry")

	ev := al.Evidence[0]
	assert.Equal(t, evidence.TypeConftestResult, ev.Type)
	assert.NotEmpty(t, ev.CollectedAt)
	require.NotNil(t, ev.Source)
	assert.NotEmpty(t, ev.Source.Coordinate,
		"coordinate must carry the input path")
	assert.Equal(t,
		"/tmp/workspace/repos/org-repo/main", ev.Source.Coordinate)
	assert.Empty(t, ev.Source.Digest,
		"OPA evaluates directories; digest should be empty")
	assert.True(t,
		strings.HasPrefix(ev.ID, evidence.IDPrefixConftest),
		"evidence ID should start with conftest prefix")
	assert.True(t,
		strings.HasPrefix(ev.Source.ReferenceID, evidence.RefPrefixConftest),
		"reference ID should start with conftest-input- prefix")
	assert.Contains(t, ev.Source.Remarks, "Collected on host")
}

func TestToScanResponse_EvidenceWithBranch(t *testing.T) {
	now := time.Now()
	results := []*PerTargetResult{
		{
			Target:    "org/repo",
			Branch:    "develop",
			InputPath: "/workspace/repos/org-repo/develop",
			ScannedAt: now,
			Status:    "scanned",
			Findings: []Finding{
				{
					RequirementID: "ci.action_pinning",
					Result:        "fail",
					Reason:        "unpinned",
				},
			},
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.Assessments, 1)
	require.Len(t, resp.Assessments[0].Evidence, 1)

	ev := resp.Assessments[0].Evidence[0]
	assert.Equal(t, "conftest-org-repo-develop", ev.ID)
	assert.Equal(t, "conftest-input-org-repo-develop", ev.Source.ReferenceID)
}

func TestToScanResponse_EvidenceOnSyntheticAssessment(t *testing.T) {
	// Synthetic assessments (all checks passed) should also carry evidence.
	now := time.Now()
	results := []*PerTargetResult{
		{
			Target:       "target1",
			Branch:       "main",
			InputPath:    "/workspace/repos/target1/main",
			ScannedAt:    now,
			Status:       "scanned",
			SuccessCount: 3,
		},
	}
	reverseMap := map[string]string{
		"kubernetes.run_as_root": "CIS-K8S-5.2.6",
	}

	resp := ToScanResponse(results, reverseMap)
	require.Len(t, resp.Assessments, 1)
	require.Len(t, resp.Assessments[0].Evidence, 1,
		"synthetic assessments should carry evidence")

	ev := resp.Assessments[0].Evidence[0]
	assert.Equal(t, evidence.TypeConftestResult, ev.Type)
	assert.NotEmpty(t, ev.Source.Coordinate)
}

func TestToScanResponse_ErrorTargetNoEvidence(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target: "org/repo",
			Branch: "main",
			Status: "error",
			Error:  "clone failed",
		},
	}

	resp := ToScanResponse(results, nil)
	assert.Empty(t, resp.Assessments,
		"error-only targets produce no assessments and no evidence")
}

func TestToScanResponse_MappingReferences(t *testing.T) {
	now := time.Now()
	results := []*PerTargetResult{
		{
			Target:    "org/repo",
			Branch:    "main",
			InputPath: "/workspace/repos/org-repo/main",
			ScannedAt: now,
			Status:    "scanned",
			Findings: []Finding{
				{
					RequirementID: "ci.action_pinning",
					Result:        "fail",
					Reason:        "violation",
				},
			},
		},
		{
			Target:    "org/repo2",
			Branch:    "develop",
			InputPath: "/workspace/repos/org-repo2/develop",
			ScannedAt: now,
			Status:    "scanned",
			Findings: []Finding{
				{
					RequirementID: "ci.action_pinning",
					Result:        "fail",
					Reason:        "unpinned",
				},
			},
		},
	}

	resp := ToScanResponse(results, nil)

	require.Len(t, resp.MappingReferences, 2,
		"each unique target/branch should produce one MappingReference")

	refIDs := make(map[string]bool)
	for _, mr := range resp.MappingReferences {
		refIDs[mr.ID] = true
		assert.True(t,
			strings.HasPrefix(mr.ID, evidence.RefPrefixConftest),
			"MappingReference ID should start with conftest-input- prefix")
		assert.NotEmpty(t, mr.Title)
	}

	// Verify each evidence source reference matches a MappingReference.
	for _, al := range resp.Assessments {
		for _, ev := range al.Evidence {
			assert.True(t, refIDs[ev.Source.ReferenceID],
				"Evidence.Source.ReferenceID %q should match a MappingReference.ID",
				ev.Source.ReferenceID)
		}
	}
}

func TestToScanResponse_MappingReferencesSkipErrors(t *testing.T) {
	results := []*PerTargetResult{
		{
			Target:    "org/repo",
			Branch:    "main",
			InputPath: "/workspace/repos/org-repo/main",
			ScannedAt: time.Now(),
			Status:    "scanned",
			Findings: []Finding{
				{
					RequirementID: "ci.rule",
					Result:        "fail",
					Reason:        "bad",
				},
			},
		},
		{
			Target: "org/broken",
			Branch: "main",
			Status: "error",
			Error:  "clone failed",
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.MappingReferences, 1,
		"error targets should not produce MappingReferences")
	assert.Contains(t, resp.MappingReferences[0].ID, "org-repo")
}

func TestToScanResponse_MappingReferencesDeduplication(t *testing.T) {
	now := time.Now()
	// Same target/branch appearing in multiple findings should
	// produce only one MappingReference.
	results := []*PerTargetResult{
		{
			Target:    "org/repo",
			Branch:    "main",
			InputPath: "/workspace/repos/org-repo/main",
			ScannedAt: now,
			Status:    "scanned",
			Findings: []Finding{
				{RequirementID: "rule.a", Result: "fail", Reason: "a"},
				{RequirementID: "rule.b", Result: "fail", Reason: "b"},
			},
		},
	}

	resp := ToScanResponse(results, nil)
	require.Len(t, resp.MappingReferences, 1,
		"duplicate target/branch should produce only one MappingReference")
}
