package results

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestParseAmpelOutput_Pass(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-pass.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "pass", result.Status)
	require.Len(t, result.Findings, 2)
	for _, f := range result.Findings {
		require.Equal(t, "pass", f.Result)
	}
	require.Equal(t, "check-BP-1.01", result.Findings[0].TenetID)
	require.Equal(t, "check-BP-3.01", result.Findings[1].TenetID)
}

func TestParseAmpelOutput_Fail(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-fail.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "fail", result.Status)
	require.Len(t, result.Findings, 2)

	var failCount int
	for _, f := range result.Findings {
		if f.Result == "fail" {
			failCount++
		}
	}
	require.Equal(t, 1, failCount)
}

func TestParseAmpelOutput_DSSEEnvelope(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-dsse-fail.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "fail", result.Status)
	require.Len(t, result.Findings, 2)

	var passCount, failCount int
	for _, f := range result.Findings {
		switch f.Result {
		case "pass":
			passCount++
		case "fail":
			failCount++
		}
	}
	require.Equal(t, 1, passCount, "expected 1 passing finding")
	require.Equal(t, 1, failCount, "expected 1 failing finding")
}

func TestParseAmpelOutput_Error(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-error.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "error", result.Status)
	require.NotEmpty(t, result.Error)
}

func TestParseAmpelOutput_Empty(t *testing.T) {
	_, err := ParseAmpelOutput([]byte{}, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestParseAmpelOutput_MalformedJSON(t *testing.T) {
	_, err := ParseAmpelOutput([]byte("{invalid json"), "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing")
}

func TestParseAmpelOutput_ControlCharsStripped(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "PASS",
			Results: []ampelPolicyResult{
				{
					Status: "PASS",
					Policy: ampelPolicyRef{ID: "BP-1.01"},
					EvalResults: []ampelEvalResult{
						{
							ID:         "01",
							Status:     "PASS",
							Assessment: &ampelAssessment{Message: "OK\x07bell"},
						},
					},
					Meta: ampelResultMeta{Description: "Test\x00Title\x01With\x02Controls"},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	result, err := ParseAmpelOutput(data, "repo", "main")
	require.NoError(t, err)
	require.Equal(t, "TestTitleWithControls", result.Findings[0].Title)
	require.Equal(t, "OKbell", result.Findings[0].Reason)
}

func TestParseAmpelOutput_OversizedField(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "PASS",
			Results: []ampelPolicyResult{
				{
					Status: "PASS",
					Policy: ampelPolicyRef{ID: "BP-1.01"},
					EvalResults: []ampelEvalResult{
						{
							ID:         "01",
							Status:     "PASS",
							Assessment: &ampelAssessment{Message: "OK"},
						},
					},
					Meta: ampelResultMeta{Description: strings.Repeat("x", maxFieldSize+1)},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	_, err = ParseAmpelOutput(data, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum size")
}

func TestParseAmpelOutput_NonPrintablePolicyID(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "PASS",
			Results: []ampelPolicyResult{
				{
					Status: "PASS",
					Policy: ampelPolicyRef{ID: "SC-CODE\x80-01"},
					EvalResults: []ampelEvalResult{
						{ID: "01", Status: "PASS", Assessment: &ampelAssessment{Message: "OK"}},
					},
					Meta: ampelResultMeta{Description: "Test"},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	_, err = ParseAmpelOutput(data, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-printable")
}

func TestParseAmpelOutput_OversizedErrorField(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "ERROR",
			Error: &ampelError{
				Message: strings.Repeat("x", maxFieldSize+1),
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	_, err = ParseAmpelOutput(data, "repo", "main")
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds maximum size")
}

func TestWritePerRepoResult(t *testing.T) {
	dir := t.TempDir()
	result := &PerRepoResult{
		Repository: "https://github.com/myorg/repo1",
		Branch:     "main",
		Status:     "pass",
		Findings: []Finding{
			{TenetID: "t1", Title: "Test", Result: "pass", Reason: "OK"},
		},
	}
	err := WritePerRepoResult(result, dir)
	require.NoError(t, err)

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Contains(t, files[0].Name(), "myorg-repo1-main.json")
}

func TestWritePerRepoResult_Overwrites(t *testing.T) {
	dir := t.TempDir()
	r1 := &PerRepoResult{Repository: "https://github.com/org/repo", Branch: "main", Status: "pass"}
	r2 := &PerRepoResult{Repository: "https://github.com/org/repo", Branch: "main", Status: "fail"}

	require.NoError(t, WritePerRepoResult(r1, dir))
	require.NoError(t, WritePerRepoResult(r2, dir))

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	data, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	require.NoError(t, err)
	require.Contains(t, string(data), `"fail"`)
}

func TestToScanResponse(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository: "https://gitlab.com/myorg/repo2",
			Branch:     "main",
			Status:     "fail",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "fail", Reason: "Not configured"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	// Same requirement ID → grouped into one assessment with two steps
	require.Len(t, resp.Assessments, 1)
	require.Empty(t, resp.Errors)
	assessment := resp.Assessments[0]
	require.Equal(t, "BP-1.01", assessment.RequirementID)
	require.Len(t, assessment.Steps, 2)
	require.Equal(t, provider.ConfidenceLevelHigh, assessment.Confidence)

	// Sort by Name for deterministic assertions
	steps := assessment.Steps
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].Name < steps[j].Name
	})

	require.Equal(t, "myorg/repo1@main", steps[0].Name)
	require.Equal(t, provider.ResultPassed, steps[0].Result)

	require.Equal(t, "myorg/repo2@main", steps[1].Name)
	require.Equal(t, provider.ResultFailed, steps[1].Result)
}

func TestToScanResponse_MultipleChecks(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "pass", Reason: "OK"},
				{TenetID: "check-BP-2.01", Title: "Check 2", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "fail",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "fail", Reason: "Not configured"},
				{TenetID: "check-BP-2.01", Title: "Check 2", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	// Two distinct requirement IDs → two assessments
	require.Len(t, resp.Assessments, 2)
	require.Empty(t, resp.Errors)

	// Each assessment should have 2 steps (one per repo)
	for _, a := range resp.Assessments {
		require.Len(t, a.Steps, 2, "RequirementID %s should have 2 steps", a.RequirementID)
	}
}

func TestToScanResponse_ErrorRepo(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "error",
			Error:      "connection refused",
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Empty(t, resp.Assessments,
		"operational errors should not produce assessments")
	require.Len(t, resp.Errors, 1)
	require.Contains(t, resp.Errors[0], "connection refused")
	require.Contains(t, resp.Errors[0], "myorg/repo1@main")
}

func TestToScanResponse_ErrorRepoWithFindings(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "error",
			Error:      "partial failure",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "fail", Reason: "violation"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)
	require.Equal(t, "BP-1.01", resp.Assessments[0].RequirementID)
	require.Len(t, resp.Assessments[0].Steps, 1)
	require.Equal(t, provider.ResultError, resp.Assessments[0].Steps[0].Result,
		"findings on error repos should map to ResultError via mapResult")
	require.Empty(t, resp.Errors,
		"error repos with findings should not duplicate into resp.Errors")
}

func TestToScanResponse_PassingRequirements(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "pass", Reason: "OK"},
			},
		},
	}
	// BP-1.01 has findings, BP-2.01 does not — should get a synthetic passing assessment
	allReqIDs := []string{"BP-1.01", "BP-2.01"}

	resp := ToScanResponse(repoResults, allReqIDs)
	require.Len(t, resp.Assessments, 2)

	// Find the synthetic assessment
	var synthetic *provider.AssessmentLog
	for i := range resp.Assessments {
		if resp.Assessments[i].RequirementID == "BP-2.01" {
			synthetic = &resp.Assessments[i]
			break
		}
	}
	require.NotNil(t, synthetic, "expected synthetic assessment for BP-2.01")
	require.Len(t, synthetic.Steps, 1)
	require.Equal(t, provider.ResultPassed, synthetic.Steps[0].Result)
	require.Equal(t, "myorg/repo1@main", synthetic.Steps[0].Name)
	require.Equal(t, "all checks passed", synthetic.Steps[0].Message)
	require.Contains(t, synthetic.Message, "1 of 1 repositories passed")
}

func TestToScanResponse_NilRequirementIDs_FindingsOnly(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1, "nil allRequirementIDs should produce findings-only assessments")
	require.Equal(t, "BP-1.01", resp.Assessments[0].RequirementID)
}

func TestToScanResponse_ErrorReposExcludedFromSyntheticSteps(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings:   []Finding{},
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "error",
			Error:      "connection refused",
		},
	}
	allReqIDs := []string{"BP-1.01"}

	resp := ToScanResponse(repoResults, allReqIDs)
	require.Len(t, resp.Assessments, 1)
	require.Equal(t, "BP-1.01", resp.Assessments[0].RequirementID)
	// Only repo1 should have a synthetic step (repo2 is error)
	require.Len(t, resp.Assessments[0].Steps, 1)
	require.Equal(t, "myorg/repo1@main", resp.Assessments[0].Steps[0].Name)
	require.Equal(t, provider.ResultPassed, resp.Assessments[0].Steps[0].Result)
}

func TestToScanResponse_MixedFindingsAndPassing(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "fail",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "fail", Reason: "violation"},
				{TenetID: "check-BP-2.01", Title: "Check 2", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "pass", Reason: "OK"},
				{TenetID: "check-BP-2.01", Title: "Check 2", Result: "pass", Reason: "OK"},
			},
		},
	}
	// BP-3.01 has no findings at all — should get synthetic assessment
	allReqIDs := []string{"BP-1.01", "BP-2.01", "BP-3.01"}

	resp := ToScanResponse(repoResults, allReqIDs)
	require.Len(t, resp.Assessments, 3)

	// BP-1.01 and BP-2.01 from findings, BP-3.01 synthetic
	reqIDs := make(map[string]bool)
	for _, a := range resp.Assessments {
		reqIDs[a.RequirementID] = true
	}
	require.True(t, reqIDs["BP-1.01"])
	require.True(t, reqIDs["BP-2.01"])
	require.True(t, reqIDs["BP-3.01"])

	// Find BP-3.01 (synthetic) and verify
	for _, a := range resp.Assessments {
		if a.RequirementID == "BP-3.01" {
			require.Len(t, a.Steps, 2, "synthetic should have one step per non-error repo")
			for _, step := range a.Steps {
				require.Equal(t, provider.ResultPassed, step.Result)
			}
		}
	}
}

func TestToScanResponse_DuplicateRequirementIDs(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings:   []Finding{},
		},
	}
	allReqIDs := []string{"BP-1.01", "BP-1.01"}

	resp := ToScanResponse(repoResults, allReqIDs)
	require.Len(t, resp.Assessments, 1, "duplicate requirement IDs should not produce duplicate assessments")
	require.Equal(t, "BP-1.01", resp.Assessments[0].RequirementID)
}

func TestToScanResponse_EmptyRequirementIDs(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check 1", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, []string{})
	require.Len(t, resp.Assessments, 1, "empty allRequirementIDs should produce findings-only assessments")
	require.Equal(t, "BP-1.01", resp.Assessments[0].RequirementID)
}

func TestBuildSyntheticSteps(t *testing.T) {
	repoResults := []*PerRepoResult{
		{Repository: "https://github.com/myorg/repo1", Branch: "main", Status: "pass"},
		{Repository: "https://github.com/myorg/repo2", Branch: "develop", Status: "error", Error: "timeout"},
		{Repository: "https://gitlab.com/myorg/repo3", Branch: "main", Status: "pass"},
	}

	steps := buildSyntheticSteps(repoResults)
	require.Len(t, steps, 2, "error repo should be excluded")
	require.Equal(t, "myorg/repo1@main", steps[0].Name)
	require.Equal(t, provider.ResultPassed, steps[0].Result)
	require.Equal(t, "all checks passed", steps[0].Message)
	require.Equal(t, "myorg/repo3@main", steps[1].Name)
}

func TestMapResult(t *testing.T) {
	require.Equal(t, provider.ResultPassed, mapResult("pass", "pass"))
	require.Equal(t, provider.ResultFailed, mapResult("fail", "fail"))
	require.Equal(t, provider.ResultError, mapResult("pass", "error"))
	require.Equal(t, provider.ResultError, mapResult("unknown", "pass"))
	require.Equal(t, provider.ResultSkipped, mapResult("skip", "pass"))
}
