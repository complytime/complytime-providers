// SPDX-License-Identifier: Apache-2.0

package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/config"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/convert"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/generate"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/scan"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/toolcheck"
)

func TestMain(m *testing.M) {
	// Skip tool check for most tests since snappy/ampel may not be installed
	SkipToolCheck = true
	os.Exit(m.Run())
}

func makeTestConfigurations() []provider.AssessmentConfiguration {
	return []provider.AssessmentConfiguration{
		{PlanID: "ap-bp-1.01", RequirementID: "BP-1.01"},
	}
}

func makeTestAttestation() []byte {
	stmt := map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]any{
			{
				"name": "test-subject",
				"digest": map[string]string{
					"sha256": "abc123def456",
				},
			},
		},
		"predicateType": "http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml",
		"predicate":     map[string]any{},
	}
	data, _ := json.Marshal(stmt)
	return data
}

func makeAmpelResultAttestation() []byte {
	stmt := map[string]any{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": []map[string]any{
			{
				"name":   "test-subject",
				"digest": map[string]string{"sha256": "abc123def456"},
			},
		},
		"predicateType": "https://carabiner.dev/ampel/resultset/v0",
		"predicate": map[string]any{
			"status": "PASS",
			"results": []map[string]any{
				{
					"status": "PASS",
					"policy": map[string]string{"id": "BP-1.01"},
					"eval_results": []map[string]any{
						{
							"id":         "01",
							"status":     "PASS",
							"assessment": map[string]string{"message": "OK"},
						},
					},
					"meta": map[string]string{"description": "Check PR"},
				},
			},
		},
	}
	data, _ := json.Marshal(stmt)
	return data
}

// writeGranularPolicies creates granular policy files in the given directory
// so that Generate can load and match them.
func writeGranularPolicies(t *testing.T, dir string, policyIDs ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0750))
	for _, id := range policyIDs {
		p := convert.AmpelPolicy{
			ID: id,
			Meta: convert.PolicyMeta{
				Description: "Test policy " + id,
				Controls:    []convert.PolicyControl{{Framework: "repo-branch-protection", Class: "source-code", ID: "BP-1"}},
			},
			Tenets: []convert.AmpelTenet{
				{
					ID:         "01",
					Code:       "true",
					Predicates: convert.PredicateSpec{Types: []string{"http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml"}},
					Assessment: convert.TenetMessage{Message: "OK"},
					Error:      convert.TenetError{Message: "FAIL", Guidance: "Fix it"},
				},
			},
		}
		data, err := json.MarshalIndent(p, "", "  ")
		require.NoError(t, err)
		filename := filepath.Join(dir, id+".json")
		require.NoError(t, os.WriteFile(filename, data, 0600))
	}
}

// setupServer creates a temp directory, changes to it, and sets up
// granular policy files for testing.
func setupServer(t *testing.T) (*ProviderServer, string) {
	t.Helper()
	dir := t.TempDir()

	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	require.NoError(t, config.EnsureDirectories())

	s := New()

	// Write granular policy files to the default granular policy dir
	policyDir := config.GranularPolicyDirPath()
	writeGranularPolicies(t, policyDir, "BP-1.01")

	return s, dir
}

// setupServerWithGenerate creates a server and runs Generate to prepare
// policy artifacts for scanning.
func setupServerWithGenerate(t *testing.T) (*ProviderServer, string) {
	t.Helper()
	s, dir := setupServer(t)

	// Generate a policy bundle so paths exist
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	return s, dir
}

// --- Describe tests (US4) ---

func TestDescribe_Healthy(t *testing.T) {
	s := New()
	resp, err := s.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	require.True(t, resp.Healthy)
	require.Equal(t, "0.0.0-unknown", resp.Version)
	require.Equal(t, []string{"url", "specs"}, resp.RequiredTargetVariables)
}

// --- Generate tests (US1) ---

func TestGenerate_ValidConfiguration(t *testing.T) {
	s, dir := setupServer(t)
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Empty(t, resp.ErrorMessage)

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01")
	require.Contains(t, string(data), "complytime-ampel-policy")
}

func TestGenerate_EmptyConfiguration(t *testing.T) {
	s, _ := setupServer(t)
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{},
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.ErrorMessage, "no assessment configurations")
}

func TestGenerate_NoMatchingPolicies(t *testing.T) {
	s, dir := setupServer(t)

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "ap-nonexistent-rule", RequirementID: "nonexistent-rule"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "should succeed with no matches (no error)")

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	_, err = os.Stat(outputPath)
	require.True(t, os.IsNotExist(err), "no policy file should be created when no rules match")
}

func TestGenerate_OverwritesExistingPolicy(t *testing.T) {
	s, dir := setupServer(t)

	// Add a second granular policy
	policyDir := config.GranularPolicyDirPath()
	writeGranularPolicies(t, policyDir, "BP-3.01")

	configs1 := makeTestConfigurations()
	configs2 := []provider.AssessmentConfiguration{
		{PlanID: "ap-bp-3.01", RequirementID: "BP-3.01"},
	}

	resp1, err := s.Generate(context.Background(), &provider.GenerateRequest{Configuration: configs1})
	require.NoError(t, err)
	require.True(t, resp1.Success)

	resp2, err := s.Generate(context.Background(), &provider.GenerateRequest{Configuration: configs2})
	require.NoError(t, err)
	require.True(t, resp2.Success)

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-3.01")
}

func TestGenerate_CustomPolicyDir(t *testing.T) {
	s, dir := setupServer(t)

	// Write granular policies to a custom directory
	customDir := filepath.Join(dir, "custom-policies")
	writeGranularPolicies(t, customDir, "BP-1.01")

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration:   makeTestConfigurations(),
		GlobalVariables: map[string]string{"ampel_policy_dir": customDir},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Empty(t, resp.ErrorMessage)

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir, config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01")
}

func TestGenerate_MissingToolReturnsError(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"nonexistent-ampel-tool-xyz"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.ErrorMessage, "nonexistent-ampel-tool-xyz")
}

func TestGenerate_WritesScanConfig(t *testing.T) {
	s, _ := setupServer(t)

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	cfg, err := generate.ReadScanConfig(config.ScanConfigDirPath())
	require.NoError(t, err)
	require.Equal(t, []string{"BP-1.01"}, cfg.RequirementIDs)
	require.NotEmpty(t, cfg.GeneratedAt)
}

func TestGenerate_NoMatchingPolicies_NoScanConfig(t *testing.T) {
	s, _ := setupServer(t)

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "ap-nonexistent", RequirementID: "nonexistent"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	_, err = generate.ReadScanConfig(config.ScanConfigDirPath())
	require.Error(t, err, "scan config should not exist when no policies match")
}

// --- Generate tests: complypack content path ---

// makePolicyTarGz creates a tar.gz archive containing granular policy
// JSON files suitable for complypack content.
func makePolicyTarGz(t *testing.T, policyIDs ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, id := range policyIDs {
		p := convert.AmpelPolicy{
			ID: id,
			Meta: convert.PolicyMeta{
				Description: "Test policy " + id,
				Controls: []convert.PolicyControl{
					{Framework: "repo-branch-protection", Class: "source-code", ID: "BP-1"},
				},
			},
			Tenets: []convert.AmpelTenet{
				{
					ID:         "01",
					Code:       "true",
					Predicates: convert.PredicateSpec{Types: []string{"http://github.com/carabiner-dev/snappy/specs/branch-rules.yaml"}},
					Assessment: convert.TenetMessage{Message: "OK"},
					Error:      convert.TenetError{Message: "FAIL", Guidance: "Fix it"},
				},
			},
		}
		data, err := json.MarshalIndent(p, "", "  ")
		require.NoError(t, err)

		hdr := &tar.Header{
			Name:     id + ".json",
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
			Mode:     0644,
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err = tw.Write(data)
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestGenerate_ComplypackContentPath_Directory(t *testing.T) {
	s, dir := setupServer(t)

	// Write policies to a complypack-style directory (not the default dir).
	complypackDir := filepath.Join(dir, "complypack-content")
	writeGranularPolicies(t, complypackDir, "BP-1.01")

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration:         makeTestConfigurations(),
		ComplypackContentPath: complypackDir,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Empty(t, resp.ErrorMessage)

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir,
		config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01")
}

func TestGenerate_ComplypackContentPath_TarGz(t *testing.T) {
	s, dir := setupServer(t)

	archive := makePolicyTarGz(t, "BP-1.01")
	archivePath := filepath.Join(dir, "content.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, archive, 0600))

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration:         makeTestConfigurations(),
		ComplypackContentPath: archivePath,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Empty(t, resp.ErrorMessage)

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir,
		config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01")
}

func TestGenerate_ComplypackPrecedenceOverPolicyDir(t *testing.T) {
	s, dir := setupServer(t)

	// Write BP-1.01 to complypack dir, BP-3.01 to custom ampel_policy_dir.
	complypackDir := filepath.Join(dir, "complypack-content")
	writeGranularPolicies(t, complypackDir, "BP-1.01")

	customDir := filepath.Join(dir, "custom-policies")
	writeGranularPolicies(t, customDir, "BP-3.01")

	// Both are set — complypack should win.
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration:         makeTestConfigurations(),
		ComplypackContentPath: complypackDir,
		GlobalVariables:       map[string]string{"ampel_policy_dir": customDir},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir,
		config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01",
		"complypack content should be used, not ampel_policy_dir")
	require.NotContains(t, string(data), "BP-3.01",
		"ampel_policy_dir content should NOT be used when complypack is set")
}

func TestGenerate_BackwardCompat_EmptyComplypackPath(t *testing.T) {
	s, dir := setupServer(t)

	// No ComplypackContentPath — should fall back to default dir (set by setupServer).
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	outputPath := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir,
		config.GeneratedPolicyDir, convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01")
}

// --- Scan tests (US2) ---

// mockScanRunner returns different outputs for snappy vs ampel calls.
type mockScanRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
	err          error
}

func (m *mockScanRunner) Run(name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockScanRunner) RunWithEnv(_ []string, name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockScanRunner) run(name string, args ...string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if name == "snappy" {
		return m.snappyOutput, nil
	}
	// Write ampel output to the results path specified by --results-path
	for i, arg := range args {
		if arg == "--results-path" && i+1 < len(args) {
			_ = os.WriteFile(args[i+1], m.ampelOutput, 0600)
			break
		}
	}
	return nil, nil
}

func TestScan_ValidTargets(t *testing.T) {
	s, dir := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 1)
	require.Equal(t, provider.ResultPassed, resp.Assessments[0].Steps[0].Result)

	// Verify snappy attestation and ampel intoto result files were created
	resultsDir := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir, config.DefaultResultsDir)
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.Len(t, files, 3) // snappy attestation + ampel intoto result + per-repo result
}

func TestScan_EmptySpecs_ReturnsError(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url": "https://github.com/myorg/repo1",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required variable 'specs'")
}

func TestScan_MultipleSpecs(t *testing.T) {
	s, dir := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "github/branch-rules.yaml,github/custom-check.yaml",
			}},
		},
	})
	require.NoError(t, err)
	// 2 specs × 1 branch with same requirement ID = 1 assessment with 2 steps
	require.Len(t, scanResp.Assessments, 1)
	require.Len(t, scanResp.Assessments[0].Steps, 2)

	// Verify 5 output files (2 snappy + 2 ampel + 1 per-repo result)
	resultsDir := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir, config.DefaultResultsDir)
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.Len(t, files, 5)
}

func TestScan_ScanError_ContinuesScanning(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	// Mock runner that fails for first target's snappy call, succeeds for second
	callCount := 0
	origRunner := ScanRunner
	ScanRunner = &mockCallCountRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
		failOnCall:   1,
		callCount:    &callCount,
	}
	defer func() { ScanRunner = origRunner }()

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
			{TargetID: "myorg-repo2", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo2",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)
	// The successful target produces 1 assessment; the error target goes to resp.Errors
	require.Len(t, scanResp.Assessments, 1)
	require.Len(t, scanResp.Errors, 1)
	require.Contains(t, scanResp.Errors[0], "myorg/repo1@main")
}

type mockCallCountRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
	failOnCall   int
	callCount    *int
}

func (m *mockCallCountRunner) Run(name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockCallCountRunner) RunWithEnv(_ []string, name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockCallCountRunner) run(name string, args ...string) ([]byte, error) {
	*m.callCount++
	// Fail on the snappy call for the first target
	if *m.callCount <= 1 && m.failOnCall == 1 {
		return nil, fmt.Errorf("connection refused")
	}
	if name == "snappy" {
		return m.snappyOutput, nil
	}
	// Write ampel output to the results path specified by --results-path
	for i, arg := range args {
		if arg == "--results-path" && i+1 < len(args) {
			_ = os.WriteFile(args[i+1], m.ampelOutput, 0600)
			break
		}
	}
	return nil, nil
}

func TestScan_MissingURLVariable(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required variable 'url'")
}

func TestScan_MissingSpecsVariable(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{
				"url": "https://github.com/myorg/repo1",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required variable 'specs'")
}

func TestScan_BranchesDefault(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
				// branches omitted — should default to "main"
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 1)
}

func TestScan_CommaSeparatedBranches(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":      "https://github.com/myorg/repo1",
				"specs":    "builtin:github/branch-rules.yaml",
				"branches": "main,develop",
			}},
		},
	})
	require.NoError(t, err)
	// 2 branches × 1 spec = 2 scan results, merged into assessments
	require.NotEmpty(t, resp.Assessments)
}

func TestScan_PlatformHintVariable(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "corp-repo", Variables: map[string]string{
				"url":      "https://git.corp.com/myorg/repo1",
				"specs":    "builtin:github/branch-rules.yaml",
				"platform": "github",
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Assessments, 1)
}

func TestScan_BranchValidation(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  []byte("{}"),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{
				"url":      "https://github.com/myorg/repo1",
				"specs":    "builtin:github/branch-rules.yaml",
				"branches": "main;rm -rf /",
			}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid characters")
}

func TestScan_EmptyTargets(t *testing.T) {
	s := New()
	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no targets")
}

// Tool check integration tests

func TestGenerate_ToolCheckError_IncludesToolName(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"missing-snappy-test", "missing-ampel-test"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, resp.ErrorMessage, "missing-snappy-test")
	require.Contains(t, resp.ErrorMessage, "missing-ampel-test")
	require.Contains(t, resp.ErrorMessage, "PATH")
}

func TestScan_MissingToolReturnsError(t *testing.T) {
	s, _ := setupServer(t)

	origSkip := SkipToolCheck
	SkipToolCheck = false
	origTools := toolcheck.RequiredTools
	toolcheck.RequiredTools = []string{"nonexistent-ampel-tool-xyz"}
	defer func() {
		SkipToolCheck = origSkip
		toolcheck.RequiredTools = origTools
	}()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "test", Variables: map[string]string{"github_token": "ghp_test"}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent-ampel-tool-xyz")
}

func TestScan_SynthesizesPassingAssessments(t *testing.T) {
	s, dir := setupServer(t)

	// Write two granular policies
	policyDir := config.GranularPolicyDirPath()
	writeGranularPolicies(t, policyDir, "BP-3.01")

	// Generate with both BP-1.01 and BP-3.01
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "ap-bp-1.01", RequirementID: "BP-1.01"},
			{PlanID: "ap-bp-3.01", RequirementID: "BP-3.01"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify scan config has both IDs
	cfg, err := generate.ReadScanConfig(config.ScanConfigDirPath())
	require.NoError(t, err)
	require.Len(t, cfg.RequirementIDs, 2)

	// Mock scan produces findings for BP-1.01 only (via ampel result attestation)
	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  makeAmpelResultAttestation(), // only has BP-1.01
	}
	defer func() { ScanRunner = origRunner }()

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)
	// Should have 2 assessments: BP-1.01 from findings, BP-3.01 synthesized
	require.Len(t, scanResp.Assessments, 2)

	reqIDs := make(map[string]bool)
	for _, a := range scanResp.Assessments {
		reqIDs[a.RequirementID] = true
	}
	require.True(t, reqIDs["BP-1.01"], "BP-1.01 should appear from findings")
	require.True(t, reqIDs["BP-3.01"], "BP-3.01 should appear as synthetic passing")

	// Verify synthetic assessment for BP-3.01
	for _, a := range scanResp.Assessments {
		if a.RequirementID == "BP-3.01" {
			require.NotEmpty(t, a.Steps, "synthetic assessment should have at least one step")
			require.Equal(t, provider.ResultPassed, a.Steps[0].Result)
		}
	}

	// Verify result files still written
	resultsDir := filepath.Join(dir, provider.WorkspaceDir, config.ProviderDir, config.DefaultResultsDir)
	files, err := os.ReadDir(resultsDir)
	require.NoError(t, err)
	require.NotEmpty(t, files)
}

func TestScan_MissingScanConfig_FallsBack(t *testing.T) {
	s, _ := setupServer(t)

	// Do NOT run Generate — scan config won't exist
	require.NoError(t, config.EnsureDirectories())

	// Write a minimal policy file so scan doesn't fail on missing policy
	policyDir := config.GeneratedPolicyDirPath()
	bundle := convert.MergeToBundle([]*convert.AmpelPolicy{
		{
			ID:   "BP-1.01",
			Meta: convert.PolicyMeta{Description: "Test"},
			Tenets: []convert.AmpelTenet{{
				ID: "01", Code: "true",
				Predicates: convert.PredicateSpec{Types: []string{"http://example.com/spec"}},
				Assessment: convert.TenetMessage{Message: "OK"},
				Error:      convert.TenetError{Message: "FAIL"},
			}},
		},
	})
	require.NoError(t, convert.WritePolicy(bundle, policyDir))

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  makeAmpelResultAttestation(),
	}
	defer func() { ScanRunner = origRunner }()

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err, "scan should succeed even without scan config")
	// Without scan config, only findings-derived assessments
	require.Len(t, scanResp.Assessments, 1)
	require.Equal(t, "BP-1.01", scanResp.Assessments[0].RequirementID)
}

// Ensure unused imports are used
var _ = scan.ExecRunner{}
var _ = convert.PolicyFileName
