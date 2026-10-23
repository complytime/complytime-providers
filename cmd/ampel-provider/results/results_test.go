// SPDX-License-Identifier: Apache-2.0

package results

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestAllFindingsSkipped(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     bool
	}{
		{
			name:     "EmptyFindings",
			findings: []Finding{},
			want:     false,
		},
		{
			name: "AllSkip",
			findings: []Finding{
				{TenetID: "check-BP-1.01", Result: "skip"},
				{TenetID: "check-BP-3.01", Result: "skip"},
			},
			want: true,
		},
		{
			name: "MixedPassAndSkip",
			findings: []Finding{
				{TenetID: "check-BP-1.01", Result: "pass"},
				{TenetID: "check-BP-3.01", Result: "skip"},
			},
			want: false,
		},
		{
			name: "AllPass",
			findings: []Finding{
				{TenetID: "check-BP-1.01", Result: "pass"},
				{TenetID: "check-BP-3.01", Result: "pass"},
			},
			want: false,
		},
		{
			name: "AllFail",
			findings: []Finding{
				{TenetID: "check-BP-1.01", Result: "fail"},
			},
			want: false,
		},
		{
			name: "SingleSkip",
			findings: []Finding{
				{TenetID: "check-BP-1.01", Result: "skip"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PerRepoResult{Findings: tt.findings}
			require.Equal(t, tt.want, r.AllFindingsSkipped())
		})
	}
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

	require.Equal(t, "check-BP-1.01", result.Findings[0].TenetID)
	require.Equal(t, "pass", result.Findings[0].Result)

	require.Equal(t, "check-BP-2.01", result.Findings[1].TenetID)
	require.Equal(t, "fail", result.Findings[1].Result)
	require.Equal(t,
		"Minimum approvals is 1, expected at least 2",
		result.Findings[1].Reason,
	)
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

func TestParseAmpelOutput_Skip(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-skip.json")
	result, err := ParseAmpelOutput(
		data, "https://github.com/myorg/repo1", "main",
	)
	require.NoError(t, err)
	// Overall status is PASS when mixed PASS+SKIP results exist
	require.Equal(t, "pass", result.Status)
	require.Len(t, result.Findings, 2)

	// First finding: PASS
	require.Equal(t, "check-BP-1.01", result.Findings[0].TenetID)
	require.Equal(t, "pass", result.Findings[0].Result)
	require.Equal(t,
		"Direct pushes are disabled in default branch. PR required.",
		result.Findings[0].Reason,
	)

	// Second finding: SKIP with reason from assessment.message
	require.Equal(t, "check-BP-3.01", result.Findings[1].TenetID)
	require.Equal(t, "skip", result.Findings[1].Result)
	require.Equal(t,
		"No matching predicate type found for policy",
		result.Findings[1].Reason,
	)
}

func TestParseAmpelOutput_AllSkip(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-all-skip.json")
	result, err := ParseAmpelOutput(
		data, "https://github.com/myorg/repo1", "main",
	)
	require.NoError(t, err)
	// Overall status is SKIP when all policies skipped
	require.Equal(t, "skip", result.Status)
	require.Len(t, result.Findings, 2)

	for _, f := range result.Findings {
		require.Equal(t, "skip", f.Result,
			"finding %s should be skip", f.TenetID)
		require.Equal(t,
			"No matching predicate type found for policy",
			f.Reason,
		)
	}
}

func TestParseAmpelOutput_Error(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-error.json")
	result, err := ParseAmpelOutput(data, "https://github.com/myorg/repo1", "main")
	require.NoError(t, err)
	require.Equal(t, "error", result.Status)
	require.Equal(t,
		"failed to fetch branch protection data: "+
			"API rate limit exceeded",
		result.Error,
	)
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

func TestParseAmpelOutput_GuidanceExtraction(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-fail.json")
	result, err := ParseAmpelOutput(
		data, "https://github.com/myorg/repo1", "main",
	)
	require.NoError(t, err)
	require.Len(t, result.Findings, 2)

	// First finding is a pass — no guidance expected
	require.Empty(t, result.Findings[0].Guidance)

	// Second finding is a fail with guidance from the fixture
	require.Equal(t,
		"Set minimum approvals to 2 or more",
		result.Findings[1].Guidance,
	)
}

func TestParseAmpelOutput_GuidanceEmptyOnPass(t *testing.T) {
	data := loadFixture(t, "testdata/ampel-verify-pass.json")
	result, err := ParseAmpelOutput(
		data, "https://github.com/myorg/repo1", "main",
	)
	require.NoError(t, err)
	for _, f := range result.Findings {
		require.Empty(t, f.Guidance,
			"passing tenet %s should have empty guidance", f.TenetID)
	}
}

func TestParseAmpelOutput_GuidanceEmptyOnFailWithoutGuidance(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "FAIL",
			Results: []ampelPolicyResult{
				{
					Status: "FAIL",
					Policy: ampelPolicyRef{ID: "BP-1.01"},
					EvalResults: []ampelEvalResult{
						{
							ID:     "01",
							Status: "FAIL",
							Error:  &ampelError{Message: "Something failed"},
						},
					},
					Meta: ampelResultMeta{Description: "Test"},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	result, err := ParseAmpelOutput(data, "repo", "main")
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	require.Empty(t, result.Findings[0].Guidance,
		"failing tenet without error.guidance should have empty guidance")
}

func TestParseAmpelOutput_GuidanceControlCharsStripped(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "FAIL",
			Results: []ampelPolicyResult{
				{
					Status: "FAIL",
					Policy: ampelPolicyRef{ID: "BP-1.01"},
					EvalResults: []ampelEvalResult{
						{
							ID:     "01",
							Status: "FAIL",
							Error: &ampelError{
								Message:  "Failed",
								Guidance: "Fix\x00this\x07now",
							},
						},
					},
					Meta: ampelResultMeta{Description: "Test"},
				},
			},
		},
	}
	data, err := json.Marshal(stmt)
	require.NoError(t, err)

	result, err := ParseAmpelOutput(data, "repo", "main")
	require.NoError(t, err)
	require.Equal(t, "Fixthisnow", result.Findings[0].Guidance)
}

func TestParseAmpelOutput_OversizedGuidance(t *testing.T) {
	stmt := ampelResultStatement{
		Predicate: ampelResultSetPred{
			Status: "FAIL",
			Results: []ampelPolicyResult{
				{
					Status: "FAIL",
					Policy: ampelPolicyRef{ID: "BP-1.01"},
					EvalResults: []ampelEvalResult{
						{
							ID:     "01",
							Status: "FAIL",
							Error: &ampelError{
								Message:  "Failed",
								Guidance: strings.Repeat("g", maxFieldSize+1),
							},
						},
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
	require.Contains(t, err.Error(), "guidance field exceeds maximum size")
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

func TestWritePerRepoResult_InvalidDir(t *testing.T) {
	result := &PerRepoResult{
		Repository: "https://github.com/org/repo",
		Branch:     "main",
		Status:     "pass",
	}
	err := WritePerRepoResult(result, "/dev/null/impossible")
	require.Error(t, err)
	require.Contains(t, err.Error(), "creating results directory")
}

func TestToScanResponse(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "pass", Reason: "OK",
				},
			},
		},
		{
			Repository: "https://gitlab.com/myorg/repo2",
			Branch:     "main",
			Status:     "fail",
			Findings: []Finding{
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "fail", Reason: "Not configured",
					Guidance: "Enable branch protection",
				},
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

	// Message uses the first non-passing step's reason
	require.Equal(t, "Not configured", assessment.Message)
	// Recommendation uses the first non-passing step's guidance
	require.Equal(t, "Enable branch protection",
		assessment.Recommendation)

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
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "pass", Reason: "PR required",
				},
				{
					TenetID: "check-BP-2.01", Title: "Check 2",
					Result: "pass", Reason: "Approvals OK",
				},
			},
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "fail",
			Findings: []Finding{
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "fail", Reason: "Not configured",
					Guidance: "Enable branch protection",
				},
				{
					TenetID: "check-BP-2.01", Title: "Check 2",
					Result: "pass", Reason: "Approvals OK",
				},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	// Two distinct requirement IDs → two assessments
	require.Len(t, resp.Assessments, 2)
	require.Empty(t, resp.Errors)

	// Each assessment should have 2 steps (one per repo)
	for _, a := range resp.Assessments {
		require.Len(t, a.Steps, 2,
			"RequirementID %s should have 2 steps", a.RequirementID)
	}

	// BP-1.01 has a failing step → message override with guidance
	require.Equal(t, "Not configured", resp.Assessments[0].Message)
	require.Equal(t, "Enable branch protection",
		resp.Assessments[0].Recommendation)

	// BP-2.01 all pass → uses first step's message, no recommendation
	require.Equal(t, "Approvals OK", resp.Assessments[1].Message)
	require.Empty(t, resp.Assessments[1].Recommendation)
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
	// Synthetic steps all pass → message override uses first step's message
	require.Equal(t, "all checks passed", synthetic.Message)
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
	require.Len(t, resp.Assessments, 1,
		"nil allRequirementIDs should produce findings-only assessments")
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

func TestToScanResponse_AllPassing(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "pass", Reason: "PR required",
				},
			},
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "pass", Reason: "PR required",
				},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)
	assessment := resp.Assessments[0]

	// All passing → uses the first step's assessment message
	require.Equal(t, "PR required", assessment.Message)
	// No recommendation when all pass
	require.Empty(t, assessment.Recommendation)
}

func TestToScanResponse_Recommendation(t *testing.T) {
	t.Run("FailureWithGuidance", func(t *testing.T) {
		repoResults := []*PerRepoResult{
			{
				Repository: "https://github.com/myorg/repo1",
				Branch:     "main",
				Status:     "fail",
				Findings: []Finding{
					{
						TenetID:  "check-BP-1.01",
						Title:    "Check 1",
						Result:   "fail",
						Reason:   "Approvals too low",
						Guidance: "Set approvals >= 2",
					},
				},
			},
		}
		resp := ToScanResponse(repoResults, nil)
		require.Len(t, resp.Assessments, 1)
		require.Equal(t, "Set approvals >= 2",
			resp.Assessments[0].Recommendation)
	})

	t.Run("FailureWithoutGuidance", func(t *testing.T) {
		repoResults := []*PerRepoResult{
			{
				Repository: "https://github.com/myorg/repo1",
				Branch:     "main",
				Status:     "fail",
				Findings: []Finding{
					{
						TenetID: "check-BP-1.01",
						Title:   "Check 1",
						Result:  "fail",
						Reason:  "Something wrong",
					},
				},
			},
		}
		resp := ToScanResponse(repoResults, nil)
		require.Len(t, resp.Assessments, 1)
		require.Empty(t, resp.Assessments[0].Recommendation)
	})

	t.Run("AllPass", func(t *testing.T) {
		repoResults := []*PerRepoResult{
			{
				Repository: "https://github.com/myorg/repo1",
				Branch:     "main",
				Status:     "pass",
				Findings: []Finding{
					{
						TenetID: "check-BP-1.01",
						Title:   "Check 1",
						Result:  "pass",
						Reason:  "All good",
					},
				},
			},
		}
		resp := ToScanResponse(repoResults, nil)
		require.Len(t, resp.Assessments, 1)
		require.Empty(t, resp.Assessments[0].Recommendation)
	})
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
			require.Len(t, a.Steps, 2,
				"synthetic should have one step per non-error repo")
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
	require.Len(t, resp.Assessments, 1,
		"duplicate requirement IDs should not produce duplicate assessments")
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
	require.Len(t, resp.Assessments, 1,
		"empty allRequirementIDs should produce findings-only assessments")
	require.Equal(t, "BP-1.01", resp.Assessments[0].RequirementID)
}

func TestToScanResponse_EmptyMessageFallback(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "pass", Reason: "",
				},
			},
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "pass",
			Findings: []Finding{
				{
					TenetID: "check-BP-1.01", Title: "Check 1",
					Result: "pass", Reason: "",
				},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)
	// All step messages empty → falls back to count string with "checks"
	require.Equal(t, "2 of 2 checks passed",
		resp.Assessments[0].Message)
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

func TestToScanResponse_AllReposErroredSkipsSynthetic(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "error",
			Error:      "GitHub API authentication failed",
		},
	}
	allReqIDs := []string{"block-force-push"}

	resp := ToScanResponse(repoResults, allReqIDs)
	// When all repos errored, synthetic assessments are skipped because
	// they cannot carry evidence (spec requires at least one Evidence
	// entry per AssessmentLog).
	require.Empty(t, resp.Assessments,
		"all-error repos must not produce synthetic assessments")
	// The operational error is still reported.
	require.Len(t, resp.Errors, 1)
	require.Contains(t, resp.Errors[0], "GitHub API authentication failed")
}

func TestToScanResponse_NoReposNoAssessments(t *testing.T) {
	resp := ToScanResponse([]*PerRepoResult{}, []string{"block-force-push"})
	// Empty repo list produces no assessments (no evidence available).
	require.Empty(t, resp.Assessments,
		"empty repo list must not produce synthetic assessments")
}

func TestToScanResponse_EvidencePerAssessment(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/myorg-repo1-main-rules-ampel.intoto.json",
			SnappyAttestationPath: "/out/myorg-repo1-main-rules-snappy.intoto.json",
			AmpelDigest:           "sha256:aaa111",
			SnappyDigest:          "sha256:bbb222",
			Remarks:               "Collected on host \"scanhost\" (machine-id: abc123)",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)
	ev := resp.Assessments[0].Evidence
	require.Len(t, ev, 2, "expected two evidence entries per assessment")

	// Ampel evidence
	require.True(t, strings.HasPrefix(ev[0].ID, "ampel_"))
	require.Equal(t, "IntotoAttestation", ev[0].Type)
	require.NotEmpty(t, ev[0].CollectedAt)
	require.NotNil(t, ev[0].Source)
	require.True(t, strings.HasPrefix(ev[0].Source.ReferenceID, "ampel_"))
	require.Equal(t, "/out/myorg-repo1-main-rules-ampel.intoto.json", ev[0].Source.Coordinate)
	require.Equal(t, "sha256:aaa111", ev[0].Source.Digest)
	require.Contains(t, ev[0].Source.Remarks, "Collected on host")

	// Snappy evidence
	require.True(t, strings.HasPrefix(ev[1].ID, "snappy_"))
	require.Equal(t, "IntotoAttestation", ev[1].Type)
	require.NotEmpty(t, ev[1].CollectedAt)
	require.NotNil(t, ev[1].Source)
	require.True(t, strings.HasPrefix(ev[1].Source.ReferenceID, "snappy_"))
	require.Equal(t, "/out/myorg-repo1-main-rules-snappy.intoto.json", ev[1].Source.Coordinate)
	require.Equal(t, "sha256:bbb222", ev[1].Source.Digest)
	require.Contains(t, ev[1].Source.Remarks, "Collected on host")
}

func TestToScanResponse_MappingReferences(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/ampel.intoto.json",
			SnappyAttestationPath: "/out/snappy.intoto.json",
			AmpelDigest:           "sha256:aaa",
			SnappyDigest:          "sha256:bbb",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.MappingReferences, 2)

	// Check that every Evidence.Source.ReferenceID matches a MappingReference.ID
	refIDs := make(map[string]bool)
	for _, mr := range resp.MappingReferences {
		refIDs[mr.ID] = true
	}
	for _, a := range resp.Assessments {
		for _, ev := range a.Evidence {
			require.True(t, refIDs[ev.Source.ReferenceID],
				"evidence ReferenceID %q should match a MappingReference.ID", ev.Source.ReferenceID)
		}
	}
}

func TestToScanResponse_ErrorRepoNoEvidence(t *testing.T) {
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "error",
			Error:      "API unavailable",
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Empty(t, resp.MappingReferences)
	for _, a := range resp.Assessments {
		require.Empty(t, a.Evidence)
	}
}

func TestToScanResponse_AllErrorsSkipsSynthetic(t *testing.T) {
	// When all repos errored, synthetic assessments must be
	// skipped because they cannot carry evidence. The spec requires
	// at least one Evidence entry per AssessmentLog.
	repoResults := []*PerRepoResult{
		{
			Repository: "https://github.com/myorg/repo1",
			Branch:     "main",
			Status:     "error",
			Error:      "API unavailable",
		},
		{
			Repository: "https://github.com/myorg/repo2",
			Branch:     "main",
			Status:     "error",
			Error:      "auth denied",
		},
	}

	allReqIDs := []string{"REQ-001", "REQ-002"}
	resp := ToScanResponse(repoResults, allReqIDs)

	// No assessments should be produced when all repos errored
	require.Empty(t, resp.Assessments,
		"all-error repos must not produce synthetic assessments")
	// Errors should still be present
	require.Len(t, resp.Errors, 2,
		"errors from failed repos must be reported")
}

func TestToScanResponse_MultiRepoEvidence(t *testing.T) {
	// Two repos contributing findings for the same requirement ID must
	// both produce evidence entries on the assessment.
	repoResults := []*PerRepoResult{
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			SpecLabel:             "branch-rules",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/repo1-main-branch-rules-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo1-main-branch-rules-snappy.intoto.json",
			AmpelDigest:           "sha256:aaa111",
			SnappyDigest:          "sha256:bbb222",
			Remarks:               "Collected on host \"h1\"",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository:            "https://github.com/myorg/repo2",
			Branch:                "main",
			SpecLabel:             "branch-rules",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "fail",
			AmpelAttestationPath:  "/out/repo2-main-branch-rules-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo2-main-branch-rules-snappy.intoto.json",
			AmpelDigest:           "sha256:ccc333",
			SnappyDigest:          "sha256:ddd444",
			Remarks:               "Collected on host \"h1\"",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "fail", Reason: "FAIL"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)

	ev := resp.Assessments[0].Evidence
	// 2 repos × 2 attestation types = 4 evidence entries
	require.Len(t, ev, 4,
		"assessment should include evidence from both contributing repos")

	// Verify evidence IDs are unique (no collision between repos)
	ids := make(map[string]bool)
	for _, e := range ev {
		require.False(t, ids[e.ID], "duplicate evidence ID: %s", e.ID)
		ids[e.ID] = true
	}

	// Verify both repos' coordinates are present
	var coordinates []string
	for _, e := range ev {
		coordinates = append(coordinates, e.Source.Coordinate)
	}
	require.Contains(t, coordinates, "/out/repo1-main-branch-rules-ampel.intoto.json")
	require.Contains(t, coordinates, "/out/repo2-main-branch-rules-ampel.intoto.json")

	// Verify mapping references also include both repos
	require.Len(t, resp.MappingReferences, 4,
		"two repos × two attestation types = four mapping references")
}

func TestToScanResponse_SpecLabelInEvidenceID(t *testing.T) {
	// Evidence IDs must include the spec label to avoid collisions when
	// the same repo/branch is scanned with different spec files.
	repoResults := []*PerRepoResult{
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			SpecLabel:             "branch-rules",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/repo1-main-branch-rules-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo1-main-branch-rules-snappy.intoto.json",
			AmpelDigest:           "sha256:aaa",
			SnappyDigest:          "sha256:bbb",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			SpecLabel:             "custom-check",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/repo1-main-custom-check-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo1-main-custom-check-snappy.intoto.json",
			AmpelDigest:           "sha256:ccc",
			SnappyDigest:          "sha256:ddd",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)

	ev := resp.Assessments[0].Evidence
	// 2 spec combos × 2 attestation types = 4 evidence entries
	require.Len(t, ev, 4)

	// Verify IDs include spec label and are unique
	ids := make(map[string]bool)
	for _, e := range ev {
		require.False(t, ids[e.ID], "duplicate evidence ID: %s", e.ID)
		ids[e.ID] = true
	}
	// Verify expected IDs using the readable encoding
	prefix1 := repoEvidencePrefix(
		"https://github.com/myorg/repo1", "main", "branch-rules",
	)
	prefix2 := repoEvidencePrefix(
		"https://github.com/myorg/repo1", "main", "custom-check",
	)
	require.True(t, ids["ampel_"+prefix1])
	require.True(t, ids["ampel_"+prefix2])
	require.True(t, ids["snappy_"+prefix1])
	require.True(t, ids["snappy_"+prefix2])

	// Mapping references should also be unique per spec
	require.Len(t, resp.MappingReferences, 4)
}

func TestToScanResponse_SameBasenameSpecs(t *testing.T) {
	// Specs with the same basename in different directories must
	// produce distinct evidence IDs when their SpecLabels include
	// the directory path (via the updated SanitizeSpecName).
	repoResults := []*PerRepoResult{
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			SpecLabel:             "team-a-check",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/repo1-main-team-a-check-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo1-main-team-a-check-snappy.intoto.json",
			AmpelDigest:           "sha256:aaa",
			SnappyDigest:          "sha256:bbb",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			SpecLabel:             "team-b-check",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/repo1-main-team-b-check-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo1-main-team-b-check-snappy.intoto.json",
			AmpelDigest:           "sha256:ccc",
			SnappyDigest:          "sha256:ddd",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)

	ev := resp.Assessments[0].Evidence
	require.Len(t, ev, 4, "2 specs x 2 attestation types = 4 evidence entries")

	ids := make(map[string]bool)
	for _, e := range ev {
		require.False(t, ids[e.ID],
			"duplicate evidence ID from same-basename specs: %s", e.ID)
		ids[e.ID] = true
	}

	// Verify distinct IDs for each spec variant using readable encoding
	prefix1 := repoEvidencePrefix(
		"https://github.com/myorg/repo1", "main", "team-a-check",
	)
	prefix2 := repoEvidencePrefix(
		"https://github.com/myorg/repo1", "main", "team-b-check",
	)
	require.True(t, ids["ampel_"+prefix1])
	require.True(t, ids["ampel_"+prefix2])
}

func TestToScanResponse_DeepNestedSpecCollision(t *testing.T) {
	// Specs sharing the same immediate parent and basename but
	// differing in ancestry must produce distinct evidence IDs.
	// SanitizeSpecName encodes slashes as "--", so
	// "dept-a/team/check.yaml" → "dept-a--team--check" and
	// "dept-b/team/check.yaml" → "dept-b--team--check".
	repoResults := []*PerRepoResult{
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			SpecLabel:             "dept-a--team--check",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/repo1-main-a-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo1-main-a-snappy.intoto.json",
			AmpelDigest:           "sha256:aaa",
			SnappyDigest:          "sha256:bbb",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			SpecLabel:             "dept-b--team--check",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/repo1-main-b-ampel.intoto.json",
			SnappyAttestationPath: "/out/repo1-main-b-snappy.intoto.json",
			AmpelDigest:           "sha256:ccc",
			SnappyDigest:          "sha256:ddd",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	require.Len(t, resp.Assessments, 1)

	ev := resp.Assessments[0].Evidence
	require.Len(t, ev, 4, "2 specs x 2 attestation types = 4 evidence entries")

	ids := make(map[string]bool)
	for _, e := range ev {
		require.False(t, ids[e.ID],
			"duplicate evidence ID from deep-nested specs: %s", e.ID)
		ids[e.ID] = true
	}
}

func TestToScanResponse_MappingReferenceFileURL(t *testing.T) {
	// Mapping reference URLs must be proper file:// URLs with
	// absolute paths, not file:// + relative path.
	repoResults := []*PerRepoResult{
		{
			Repository:            "https://github.com/myorg/repo1",
			Branch:                "main",
			ScannedAt:             time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
			Status:                "pass",
			AmpelAttestationPath:  "/out/ampel.intoto.json",
			SnappyAttestationPath: "/out/snappy.intoto.json",
			AmpelDigest:           "sha256:aaa",
			SnappyDigest:          "sha256:bbb",
			Findings: []Finding{
				{TenetID: "check-BP-1.01", Title: "Check", Result: "pass", Reason: "OK"},
			},
		},
	}

	resp := ToScanResponse(repoResults, nil)
	for _, mr := range resp.MappingReferences {
		require.True(t, strings.HasPrefix(mr.URL, "file:///"),
			"mapping reference URL must be file:///absolute: got %q", mr.URL)
	}
}
