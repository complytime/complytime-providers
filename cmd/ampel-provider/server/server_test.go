// SPDX-License-Identifier: Apache-2.0

package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/config"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/convert"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/generate"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/scan"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/targets"
	"github.com/complytime/complytime-providers/cmd/ampel-provider/toolcheck"
	"github.com/complytime/complytime-providers/internal/evidence"
	"github.com/complytime/complytime-providers/internal/genindex"
)

func TestMain(m *testing.M) {
	// Skip tool check for most tests since snappy/ampel may not be installed
	SkipToolCheck = true
	os.Exit(m.Run())
}

func setupRetentionServer(t *testing.T) (*ProviderServer, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace space % é")
	require.NoError(t, os.Mkdir(root, 0o700))
	policyDir := filepath.Join(root, "policies")
	writeGranularPolicies(t, policyDir, "BP-1.01", "BP-3.01")
	server := New()
	server.logger = hclog.NewNullLogger()
	response, err := server.Generate(context.Background(), &provider.GenerateRequest{
		GlobalVariables: map[string]string{"workspace": root, "ampel_policy_dir": "policies"},
		TargetVariables: map[string]string{"url": "https://github.com/myorg/repo1"},
		Configuration:   []provider.AssessmentConfiguration{{RequirementID: "BP-1.01"}, {RequirementID: "BP-3.01"}},
	})
	require.NoError(t, err)
	require.True(t, response.Success, response.ErrorMessage)
	for name, data := range legacyAttestations(root) {
		require.NoError(t, os.WriteFile(name, data, 0o600))
	}
	previous := ScanRunner
	t.Cleanup(func() { ScanRunner = previous })
	return server, root
}

func legacyAttestations(root string) map[string][]byte {
	prefix := targets.SanitizeRepoURL("https://github.com/myorg/repo1") + "-main-" +
		scan.SanitizeSpecName("builtin:github/branch-rules.yaml")
	directory := filepath.Join(root, config.ResultsDirPath())
	return map[string][]byte{
		filepath.Join(directory, prefix+"-ampel.intoto.json"):  makeAmpelResultAttestation(),
		filepath.Join(directory, prefix+"-snappy.intoto.json"): makeTestAttestation(),
	}
}

func assertLegacyAttestations(t *testing.T, root string) {
	t.Helper()
	for name, original := range legacyAttestations(root) {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		require.Equal(t, original, data)
	}
}

func retentionRequest(root string) *provider.ScanRequest {
	return &provider.ScanRequest{Targets: []provider.Target{{TargetID: "repository", Variables: map[string]string{
		"workspace": root, "url": "https://github.com/myorg/repo1", "branches": "main,release/1",
		"specs": "builtin:github/branch-rules.yaml,team/check.yaml",
	}}}}
}

func TestScan_ThreeScansRetainIndependentAttestations(t *testing.T) {
	server, root := setupRetentionServer(t)
	request := retentionRequest(root)
	resultsDir := filepath.Join(root, config.ResultsDirPath())
	legacy := filepath.Join(resultsDir, targets.SanitizeRepoURL(request.Targets[0].Variables["url"])+"-main.json")
	require.NoError(t, os.WriteFile(legacy, []byte("legacy summary"), 0o600))
	start := time.Date(2026, 11, 10, 23, 59, 59, 0, time.UTC)
	server.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
		options.Clock = func() time.Time { return start }
		return evidence.Allocate(options, labels...)
	}
	retained := make(map[string][]byte)
	identities := make(map[string]bool)
	leaves := make(map[string]bool)
	for scanNumber := range 3 {
		output := makeAmpelResultAttestation()
		if scanNumber > 0 {
			output = bytes.ReplaceAll(output, []byte(`"PASS"`), []byte(`"FAIL"`))
		}
		ScanRunner = &mockScanRunner{snappyOutput: makeTestAttestation(), ampelOutput: output}
		response, err := server.Scan(context.Background(), request)
		require.NoError(t, err)
		require.Empty(t, response.Errors)
		require.Len(t, response.Assessments, 2)
		require.Len(t, response.MappingReferences, 8)
		references := make(map[string]provider.MappingReference)
		for _, reference := range response.MappingReferences {
			require.False(t, identities[reference.ID], "different evaluations must not deduplicate")
			identities[reference.ID] = true
			references[reference.ID] = reference
		}
		for _, assessment := range response.Assessments {
			require.Len(t, assessment.Evidence, 8)
			for _, item := range assessment.Evidence {
				reference, found := references[item.Source.ReferenceID]
				require.True(t, found)
				path := filepath.Join(root, item.Source.Coordinate)
				locator, err := url.Parse(reference.URL)
				require.NoError(t, err)
				require.Equal(t, "file", locator.Scheme)
				require.Equal(t, path, locator.Path)
				require.Contains(t, reference.URL, "%25")
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				digest, err := evidence.FileDigest(path)
				require.NoError(t, err)
				require.Equal(t, digest, item.Source.Digest)
				require.Equal(t, start.Format(time.RFC3339), item.CollectedAt)
				require.Regexp(t, `^(ampel|snappy)_[a-f0-9]{32}$`, item.ID)
				require.Contains(t, filepath.Base(filepath.Dir(path)), start.Format(evidence.EvaluationTimeFormat))
				retained[path] = data
				leaves[filepath.Dir(path)] = true
			}
		}
		for path, original := range retained {
			current, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, original, current)
		}
		expectedFiles := []string{legacy}
		for path := range legacyAttestations(root) {
			expectedFiles = append(expectedFiles, path)
		}
		for path := range retained {
			expectedFiles = append(expectedFiles, path)
		}
		for leaf := range leaves {
			expectedFiles = append(expectedFiles, filepath.Join(leaf, evidence.MetadataFile))
		}
		var actualFiles []string
		err = filepath.WalkDir(resultsDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				actualFiles = append(actualFiles, path)
			}
			return nil
		})
		require.NoError(t, err)
		require.ElementsMatch(t, expectedFiles, actualFiles, "new scans must not write summaries")
	}
	require.Len(t, retained, 24)
	require.Len(t, leaves, 12)
	assertLegacyAttestations(t, root)
	for leaf := range leaves {
		entries, err := os.ReadDir(leaf)
		require.NoError(t, err)
		require.Len(t, entries, 3, "no per-assessment copies or legacy summaries")
		var metadata map[string]any
		data, err := os.ReadFile(filepath.Join(leaf, evidence.MetadataFile))
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &metadata))
		require.Equal(t, start.Format(time.RFC3339), metadata["collected_at"])
		target := metadata["target"].(map[string]any)
		require.Contains(t, []string{"main", "release/1"}, target["branch"])
	}
	data, err := os.ReadFile(legacy)
	require.NoError(t, err)
	require.Equal(t, "legacy summary", string(data))
	copyRoot := filepath.Join(t.TempDir(), "copied-workspace")
	require.NoError(t, os.CopyFS(copyRoot, os.DirFS(root)))
	for path := range retained {
		coordinate, err := filepath.Rel(root, path)
		require.NoError(t, err)
		originalDigest, err := evidence.FileDigest(path)
		require.NoError(t, err)
		copiedDigest, err := evidence.FileDigest(filepath.Join(copyRoot, coordinate))
		require.NoError(t, err)
		require.Equal(t, originalDigest, copiedDigest)
	}
}

func TestScan_RetainedArtifactFailureChannels(t *testing.T) {
	for _, failure := range []string{"allocation", "write", "close", "missing-output", "invalid-output", "digest", "identity", "ownership-mode", "metadata-write", "metadata-close"} {
		t.Run(failure, func(t *testing.T) {
			server, root := setupRetentionServer(t)
			request := retentionRequest(root)
			request.Targets[0].Variables["branches"] = "main"
			request.Targets[0].Variables["specs"] = "builtin:github/branch-rules.yaml"
			runner := &mockScanRunner{snappyOutput: makeTestAttestation(), ampelOutput: makeAmpelResultAttestation()}
			ScanRunner = runner
			var warnings bytes.Buffer
			server.logger = hclog.New(&hclog.LoggerOptions{Output: &warnings, Level: hclog.Warn})
			server.allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				if failure == "allocation" {
					return nil, syscall.ENOSPC
				}
				reads := 0
				options.Operations.Read = func(file *os.File) ([]byte, error) {
					if file.Name() == evidence.SnappyFile {
						reads++
					}
					if failure == "digest" && reads == 2 {
						return nil, io.ErrUnexpectedEOF
					}
					return io.ReadAll(file)
				}
				options.Operations.Write = func(file *os.File, data []byte) (int, error) {
					if failure == "write" || (failure == "metadata-write" && file.Name() == evidence.MetadataFile) {
						return 0, syscall.ENOSPC
					}
					return file.Write(data)
				}
				options.Operations.Close = func(file *os.File) error {
					if err := file.Close(); err != nil {
						return err
					}
					if failure == "close" || (failure == "metadata-close" && file.Name() == evidence.MetadataFile) {
						return syscall.EIO
					}
					return nil
				}
				options.Operations.Chmod = func(file *os.File, mode os.FileMode) error {
					if failure == "ownership-mode" && file.Name() == evidence.AmpelFile {
						return syscall.EPERM
					}
					return file.Chmod(mode)
				}
				return evidence.Allocate(options, labels...)
			}
			if failure == "identity" {
				server.identity = func() (evidence.Identity, error) { return evidence.Identity{}, syscall.EIO }
			}
			if failure == "invalid-output" {
				runner.ampelOutput = []byte("{malformed")
			}
			if failure == "missing-output" {
				runner.ampelOutput = nil
			}
			response, err := server.Scan(context.Background(), request)
			switch failure {
			case "identity", "digest", "ownership-mode":
				require.Error(t, err)
				require.Nil(t, response)
			case "metadata-write", "metadata-close":
				require.NoError(t, err)
				require.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 2)
				require.Len(t, response.MappingReferences, 2)
				require.Contains(t, warnings.String(), "supporting evaluation metadata")
			default:
				require.NoError(t, err)
				require.Len(t, response.Errors, 1)
				require.Empty(t, response.Assessments)
				require.Empty(t, response.MappingReferences)
			}
			assertLegacyAttestations(t, root)
		})
	}
}

func TestPublicOperationalError_ExcludesCredentials(t *testing.T) {
	locator := url.URL{Scheme: "https", Host: "example.com", Path: "/repository",
		User: url.UserPassword("account", "url-password"), RawQuery: "token=query-token", Fragment: "private"}
	message := publicOperationalError(fmt.Errorf("request failed for %s using access-value", locator.String()), "access-value")
	require.Contains(t, message, "https://example.com/repository")
	require.Contains(t, message, "[redacted]")
	for _, secret := range []string{"url-password", "query-token", "private", "access-value"} {
		require.NotContains(t, message, secret)
	}
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
		TargetVariables: map[string]string{
			"url": "https://github.com/myorg/repo1",
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	return s, dir
}

func generatedArtifactDir(t *testing.T) string {
	t.Helper()
	index, err := genindex.ReadIndex(config.GeneratedDirPath())
	require.NoError(t, err)
	require.Len(t, index.Targets, 1)
	for _, contentKey := range index.Targets {
		return config.GeneratedDirForKey(contentKey)
	}
	t.Fatal("expected generated artifact directory")
	return ""
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
	s, _ := setupServer(t)
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Empty(t, resp.ErrorMessage)

	outputPath := filepath.Join(generatedArtifactDir(t), convert.PolicyFileName)
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
	s, _ := setupServer(t)

	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{PlanID: "ap-nonexistent-rule", RequirementID: "nonexistent-rule"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "should succeed with no matches (no error)")

	entries, err := os.ReadDir(config.GeneratedDirPath())
	require.NoError(t, err)
	require.Empty(t, entries, "no generated artifacts should be created when no rules match")
}

func TestGenerate_OverwritesExistingPolicy(t *testing.T) {
	s, _ := setupServer(t)

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

	outputPath := filepath.Join(generatedArtifactDir(t), convert.PolicyFileName)
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

	outputPath := filepath.Join(generatedArtifactDir(t), convert.PolicyFileName)
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

	cfg, err := generate.ReadScanConfig(generatedArtifactDir(t))
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

	_, err = generate.ReadScanConfig(config.GeneratedDirPath())
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

	outputPath := filepath.Join(generatedArtifactDir(t), convert.PolicyFileName)
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

	outputPath := filepath.Join(generatedArtifactDir(t), convert.PolicyFileName)
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

	outputPath := filepath.Join(generatedArtifactDir(t), convert.PolicyFileName)
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "BP-1.01",
		"complypack content should be used, not ampel_policy_dir")
	require.NotContains(t, string(data), "BP-3.01",
		"ampel_policy_dir content should NOT be used when complypack is set")
}

func TestGenerate_BackwardCompat_EmptyComplypackPath(t *testing.T) {
	s, _ := setupServer(t)

	// No ComplypackContentPath — should fall back to default dir (set by setupServer).
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	outputPath := filepath.Join(generatedArtifactDir(t), convert.PolicyFileName)
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

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, scanResp.Assessments, 1)
	require.Equal(t, provider.ResultPassed, scanResp.Assessments[0].Steps[0].Result)

	// Resolve the reported evidence, not a former fixed filename.
	evidencePath := filepath.Join(dir, scanResp.Assessments[0].Evidence[0].Source.Coordinate)
	files, err := os.ReadDir(filepath.Dir(evidencePath))
	require.NoError(t, err)
	require.Len(t, files, 3) // two attestations and supporting metadata, no summary
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

	require.Len(t, scanResp.MappingReferences, 4)
	leaves := make(map[string]bool)
	for _, item := range scanResp.Assessments[0].Evidence {
		path := filepath.Join(dir, item.Source.Coordinate)
		require.FileExists(t, path)
		leaves[filepath.Dir(path)] = true
	}
	require.Len(t, leaves, 2)
	for leaf := range leaves {
		files, err := os.ReadDir(leaf)
		require.NoError(t, err)
		require.Len(t, files, 3)
	}
}

func TestScan_ScanError_ContinuesScanning(t *testing.T) {
	s, _ := setupServerWithGenerate(t)
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
		TargetVariables: map[string]string{
			"url": "https://github.com/myorg/repo2",
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

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
	resp, err := s.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: makeTestConfigurations(),
		TargetVariables: map[string]string{
			"url": "https://git.corp.com/myorg/repo1",
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	ampelOutput := makeAmpelResultAttestation()

	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	defer func() { ScanRunner = origRunner }()

	scanResp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "corp-repo", Variables: map[string]string{
				"url":      "https://git.corp.com/myorg/repo1",
				"specs":    "builtin:github/branch-rules.yaml",
				"platform": "github",
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, scanResp.Assessments, 1)
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
		TargetVariables: map[string]string{
			"url": "https://github.com/myorg/repo1",
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify scan config has both IDs
	cfg, err := generate.ReadScanConfig(generatedArtifactDir(t))
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

// mockDigestFailRunner simulates a scenario where scan artifacts are
// produced successfully but become unreadable before digest computation.
// During the ampel command, it deletes the snappy attestation file.
type mockDigestFailRunner struct {
	snappyOutput []byte
	ampelOutput  []byte
}

func (m *mockDigestFailRunner) Run(name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockDigestFailRunner) RunWithEnv(_ []string, name string, args ...string) ([]byte, error) {
	return m.run(name, args...)
}

func (m *mockDigestFailRunner) run(name string, args ...string) ([]byte, error) {
	if name == "snappy" {
		return m.snappyOutput, nil
	}
	for i, arg := range args {
		if arg == "--results-path" && i+1 < len(args) {
			resultsPath := args[i+1]
			// Write valid ampel output
			_ = os.WriteFile(resultsPath, m.ampelOutput, 0600)
			// Delete the snappy attestation to simulate file loss between
			// scan and digest computation
			snappyPath := filepath.Join(filepath.Dir(resultsPath), "snappy.intoto.json")
			_ = os.Remove(snappyPath)
			break
		}
	}
	return nil, nil
}

func TestScan_DigestFailure_ReturnsError(t *testing.T) {
	s, _ := setupServerWithGenerate(t)

	origRunner := ScanRunner
	ScanRunner = &mockDigestFailRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  makeAmpelResultAttestation(),
	}
	defer func() { ScanRunner = origRunner }()

	_, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.Error(t, err, "scan should fail when digest computation fails")
	require.Contains(t, err.Error(), "evidence digest failed",
		"error should indicate the digest failure")
}

// testPolicyResult describes a single policy evaluation for the
// makeAmpelAttestation builder.
type testPolicyResult struct {
	policyID    string
	status      string
	evalID      string
	evalStatus  string
	message     string
	description string
}

// makeAmpelAttestation builds an ampel ResultSet in-toto attestation
// with the given overall status and per-policy results. Use this
// instead of one-off helpers to avoid duplicating the envelope
// structure across tests.
func makeAmpelAttestation(
	overallStatus string, policies []testPolicyResult,
) []byte {
	results := make([]map[string]any, len(policies))
	for i, p := range policies {
		results[i] = map[string]any{
			"status": p.status,
			"policy": map[string]string{"id": p.policyID},
			"eval_results": []map[string]any{
				{
					"id":         p.evalID,
					"status":     p.evalStatus,
					"assessment": map[string]string{"message": p.message},
				},
			},
			"meta": map[string]string{"description": p.description},
		}
	}
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
			"status":  overallStatus,
			"results": results,
		},
	}
	data, _ := json.Marshal(stmt)
	return data
}

// withMockScanRunner installs a mockScanRunner with the given ampel
// output and a default snappy attestation. The original runner is
// restored via t.Cleanup.
func withMockScanRunner(t *testing.T, ampelOutput []byte) {
	t.Helper()
	origRunner := ScanRunner
	ScanRunner = &mockScanRunner{
		snappyOutput: makeTestAttestation(),
		ampelOutput:  ampelOutput,
	}
	t.Cleanup(func() { ScanRunner = origRunner })
}

func TestScan_AllSkipConvertedToError(t *testing.T) {
	s, _ := setupServerWithGenerate(t)
	withMockScanRunner(t, makeAmpelAttestation("SKIP", []testPolicyResult{
		{
			policyID: "BP-1.01", status: "SKIP",
			evalID: "when", evalStatus: "SKIP",
			message:     "No matching predicate type found for policy",
			description: "Validate branch protection",
		},
	}))

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)

	// All-SKIP should be converted to an operational error,
	// not produce assessments with skip results.
	require.Len(t, resp.Errors, 1,
		"all-skip repo should produce an operational error")
	require.Contains(t, resp.Errors[0], "no policies matched",
		"error should explain the root cause")
}

func TestScan_MixedSkipPass_NotConvertedToError(t *testing.T) {
	s, _ := setupServerWithGenerate(t)
	withMockScanRunner(t, makeAmpelAttestation("PASS", []testPolicyResult{
		{
			policyID: "BP-1.01", status: "PASS",
			evalID: "01", evalStatus: "PASS",
			message:     "PR required",
			description: "Validate branch protection",
		},
		{
			policyID: "BP-3.01", status: "SKIP",
			evalID: "when", evalStatus: "SKIP",
			message:     "No matching predicate type found for policy",
			description: "Validate force push blocking",
		},
	}))

	resp, err := s.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{TargetID: "myorg-repo1", Variables: map[string]string{
				"url":   "https://github.com/myorg/repo1",
				"specs": "builtin:github/branch-rules.yaml",
			}},
		},
	})
	require.NoError(t, err)

	// Mixed SKIP+PASS should NOT be converted to error
	require.Empty(t, resp.Errors,
		"mixed skip+pass should not produce operational errors")
	require.NotEmpty(t, resp.Assessments,
		"mixed skip+pass should produce assessments")

	// Verify individual findings are preserved with correct results
	var hasPass, hasSkip bool
	for _, a := range resp.Assessments {
		for _, step := range a.Steps {
			if step.Result == provider.ResultPassed {
				hasPass = true
			}
			if step.Result == provider.ResultSkipped {
				hasSkip = true
			}
		}
	}
	require.True(t, hasPass,
		"mixed result should preserve passing findings")
	require.True(t, hasSkip,
		"mixed result should preserve skipped findings")
}

func TestGenerate_MultipleTargets_WritesIndexedArtifacts(t *testing.T) {
	s, _ := setupServer(t)
	writeGranularPolicies(t, config.GranularPolicyDirPath(), "BP-2.01")

	for _, request := range []*provider.GenerateRequest{
		{
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "BP-1.01"}},
			TargetVariables: map[string]string{"url": "https://github.com/example/one"},
		},
		{
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "BP-2.01"}},
			TargetVariables: map[string]string{"url": "https://github.com/example/two"},
		},
	} {
		response, err := s.Generate(context.Background(), request)
		require.NoError(t, err)
		require.True(t, response.Success)
	}

	index, err := genindex.ReadIndex(config.GeneratedDirPath())
	require.NoError(t, err)
	require.Len(t, index.Targets, 2)
	firstKey := index.Targets[genindex.TargetKey("https://github.com/example/one")]
	secondKey := index.Targets[genindex.TargetKey("https://github.com/example/two")]
	require.NotEqual(t, firstKey, secondKey)
	for _, contentKey := range []string{firstKey, secondKey} {
		require.FileExists(t, filepath.Join(config.GeneratedDirForKey(contentKey), convert.PolicyFileName))
		require.FileExists(t, filepath.Join(config.GeneratedDirForKey(contentKey), generate.ScanConfigFileName))
	}
}

func TestGenerate_IdenticalRequirementsDifferentPolicySources_UsesDistinctContentKeys(t *testing.T) {
	s, dir := setupServer(t)
	otherPolicyDir := filepath.Join(dir, "other-policies")
	writeGranularPolicies(t, otherPolicyDir, "BP-1.01")
	requests := []*provider.GenerateRequest{
		{
			Configuration:   makeTestConfigurations(),
			TargetVariables: map[string]string{"url": "https://github.com/example/default"},
		},
		{
			Configuration:   makeTestConfigurations(),
			GlobalVariables: map[string]string{"ampel_policy_dir": otherPolicyDir},
			TargetVariables: map[string]string{"url": "https://github.com/example/custom"},
		},
	}
	for _, request := range requests {
		response, err := s.Generate(context.Background(), request)
		require.NoError(t, err)
		require.True(t, response.Success)
	}
	index, err := genindex.ReadIndex(config.GeneratedDirPath())
	require.NoError(t, err)
	require.NotEqual(t,
		index.Targets[genindex.TargetKey("https://github.com/example/default")],
		index.Targets[genindex.TargetKey("https://github.com/example/custom")],
	)
}

func TestScan_MultipleTargets_UsesTargetRequirementIDs(t *testing.T) {
	s, _ := setupServer(t)
	writeGranularPolicies(t, config.GranularPolicyDirPath(), "BP-2.01")
	for _, request := range []*provider.GenerateRequest{
		{
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "BP-1.01"}},
			TargetVariables: map[string]string{"url": "https://github.com/example/one"},
		},
		{
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "BP-2.01"}},
			TargetVariables: map[string]string{"url": "https://github.com/example/two"},
		},
	} {
		response, err := s.Generate(context.Background(), request)
		require.NoError(t, err)
		require.True(t, response.Success)
	}

	originalRunner := ScanRunner
	ScanRunner = &mockScanRunner{snappyOutput: makeTestAttestation(), ampelOutput: makeAmpelResultAttestation()}
	t.Cleanup(func() { ScanRunner = originalRunner })
	response, err := s.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{
		{TargetID: "one", Variables: map[string]string{
			"url": "https://github.com/example/one", "specs": "builtin:github/branch-rules.yaml",
		}},
		{TargetID: "two", Variables: map[string]string{
			"url": "https://github.com/example/two", "specs": "builtin:github/branch-rules.yaml",
		}},
	}})
	require.NoError(t, err)
	for _, assessment := range response.Assessments {
		if assessment.RequirementID == "BP-2.01" {
			require.Len(t, assessment.Steps, 1)
			require.Contains(t, assessment.Steps[0].Name, "example/two")
			return
		}
	}
	t.Fatal("expected target-scoped synthetic assessment for BP-2.01")
}

func TestScan_PresentIndexMissingTarget_ReturnsError(t *testing.T) {
	s, _ := setupServer(t)
	require.NoError(t, genindex.UpdateIndex(
		config.GeneratedDirPath(), genindex.TargetKey("https://github.com/example/other"), "0123456789abcdef",
	))
	_, err := s.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{{
		TargetID: "missing", Variables: map[string]string{
			"url": "https://github.com/example/missing", "specs": "builtin:github/branch-rules.yaml",
		},
	}}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing from generate index")
}

func TestScan_PresentIndexInvalidOrUnreadableArtifact_ReturnsError(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		contentKey string
		expected   string
	}{
		{name: "invalid content key", contentKey: "invalid", expected: "validating content key"},
		{name: "missing artifact", contentKey: "0123456789abcdef", expected: "reading generated scan config"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			s, _ := setupServer(t)
			url := "https://github.com/example/target"
			require.NoError(t, genindex.UpdateIndex(
				config.GeneratedDirPath(), genindex.TargetKey(url), testCase.contentKey,
			))
			_, err := s.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{{
				TargetID: "target", Variables: map[string]string{
					"url": url, "specs": "builtin:github/branch-rules.yaml",
				},
			}}})
			require.Error(t, err)
			require.Contains(t, err.Error(), testCase.expected)
		})
	}
}

func TestScan_PresentIndexMalformedOrUnsupported_ReturnsError(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		contents string
		expected string
	}{
		{name: "malformed", contents: `{`, expected: "parse generate index"},
		{name: "unsupported version", contents: `{"version":2,"targets":{}}`, expected: "unsupported generate index version"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			s, _ := setupServer(t)
			require.NoError(t, os.WriteFile(
				filepath.Join(config.GeneratedDirPath(), "generate-index.json"), []byte(testCase.contents), 0o600,
			))

			_, err := s.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{{
				TargetID: "target", Variables: map[string]string{
					"url": "https://github.com/example/target", "specs": "builtin:github/branch-rules.yaml",
				},
			}}})
			require.ErrorContains(t, err, testCase.expected)
		})
	}
}

// Ensure unused imports are used
var _ = scan.ExecRunner{}
var _ = convert.PolicyFileName
