// SPDX-License-Identifier: Apache-2.0

package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
	"github.com/complytime/complytime-providers/internal/evidence"
	"github.com/complytime/complytime-providers/internal/genindex"
)

// --- Mocks ---

func TestScan_ThreeScansRetainResultEvidence(t *testing.T) {
	for _, kind := range []string{"local-file", "local-directory", "enclosing-directory", "remote"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "workspace % café")
			require.NoError(t, os.Mkdir(root, 0755))
			cfg := config.NewWorkspaceConfig(root)
			require.NoError(t, cfg.EnsureDirectories())
			policy := filepath.Join(root, "policies")
			require.NoError(t, os.Mkdir(policy, 0755))
			require.NoError(t, generate.WriteScanConfig(cfg.GeneratedDirPath(), []string{"main", "other"},
				map[string]string{"main": "req-1", "other": "req-2"}, policy))
			legacy := filepath.Join(cfg.ResultsDirPath(), "legacy-main.json")
			require.NoError(t, os.WriteFile(legacy, []byte("legacy summary"), 0600))
			input := filepath.Join(root, "input")
			require.NoError(t, os.Mkdir(input, 0755))
			inputFile := filepath.Join(input, "input.json")
			require.NoError(t, os.WriteFile(inputFile, []byte(`{"old":true}`), 0600))
			target := localTarget(t, input, "")
			target.TargetID = "local/target"
			if kind == "local-file" {
				target.Variables["input_path"] = inputFile
			}
			if kind == "enclosing-directory" {
				t.Setenv(conftestIgnoreEnv, "")
				t.Setenv(conftestConfigFileEnv, "")
				target.Variables["input_path"] = root
			}
			if kind == "remote" {
				target = remoteTarget("", "https://user:secret@example.com/org/repo?token=secret#secret")
				target.Variables["branches"] = "main,release/1"
				target.Variables["scan_path"] = "manifests"
			}
			target.Variables[evidence.WorkspaceVariable] = root
			var currentOutput []byte
			runner := &mockRunner{outputFn: func(_ string, args []string) (scan.CommandOutput, error) {
				assert.Contains(t, args, "--no-fail")
				if kind == "enclosing-directory" {
					assert.Contains(t, args, "--ignore")
				}
				return scan.CommandOutput{Stdout: currentOutput, Stderr: []byte("diagnostic warning\n")}, nil
			}}
			cloneRoots := map[string]bool{}
			ldr := &mockLoader{loadFn: func(target provider.Target, workDir string) (string, error) {
				clone := filepath.Join(workDir, "example-com-org-repo", target.Variables["branch"])
				cloneRoots[clone] = true
				// Only the known temporary working clone is replaceable, never results.
				require.NoError(t, os.RemoveAll(clone))
				scanPath := filepath.Join(clone, target.Variables["scan_path"])
				require.NoError(t, os.MkdirAll(scanPath, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(scanPath, "input.json"), currentOutput, 0600))
				return scanPath, nil
			}}
			opts := ServerOptions{Runner: runner, ToolChecker: func() ([]string, error) { return nil, nil }}
			if kind == "remote" {
				opts.Loader = ldr
			}
			srv := New(opts)
			start := time.Date(2026, 11, 10, 23, 59, 59, 0, time.UTC)
			srv.opts.Allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
				options.Clock = func() time.Time { return start }
				return evidence.Allocate(options, labels...)
			}
			retained := map[string][]byte{}
			identities := map[string]bool{}
			urls := map[string]string{}
			for iteration := range 3 {
				currentOutput = []byte(conftestAllSuccessJSON + "\n")
				if iteration > 0 {
					currentOutput = []byte(strings.Replace(conftestAllSuccessJSON, `"successes": 5`,
						`"successes": 0, "failures": [{"msg":"policy failed"}]`, 1) + "\n\n")
				}
				response, err := srv.Scan(context.Background(), &provider.ScanRequest{Targets: []provider.Target{target}})
				require.NoError(t, err)
				assert.Empty(t, response.Errors)
				require.Len(t, response.Assessments, 2)
				references := map[string]provider.MappingReference{}
				for _, ref := range response.MappingReferences {
					references[ref.ID] = ref
					assert.NotContains(t, ref.Title+ref.Description+ref.URL, "secret")
				}
				seen := map[string]bool{}
				for _, assessment := range response.Assessments {
					for _, item := range assessment.Evidence {
						require.NotNil(t, item.Source)
						coordinate := item.Source.Coordinate
						assert.True(t, strings.HasPrefix(coordinate, filepath.Join(provider.WorkspaceDir, "opa", "results")+"/"))
						assert.Equal(t, "conftest.json", filepath.Base(coordinate))
						path := filepath.Join(root, coordinate)
						stored, err := os.ReadFile(path)
						require.NoError(t, err)
						assert.Equal(t, currentOutput, stored)
						digest := sha256.Sum256(stored)
						assert.Equal(t, "sha256:"+hex.EncodeToString(digest[:]), item.Source.Digest)
						assert.Equal(t, start.Format(time.RFC3339), item.CollectedAt)
						ref, exists := references[item.Source.ReferenceID]
						require.True(t, exists)
						parsed, err := url.Parse(ref.URL)
						require.NoError(t, err)
						assert.Equal(t, "file", parsed.Scheme)
						assert.Equal(t, path, parsed.Path)
						assert.Contains(t, ref.URL, "%25")
						if !seen[coordinate] {
							assert.False(t, identities[item.ID], "different executions need different identities")
							identities[item.ID] = true
							seen[coordinate] = true
							retained[coordinate] = append([]byte(nil), stored...)
							urls[coordinate] = ref.URL
						}
						assertRetainedPrivate(t, path, 0600)
						assertRetainedPrivate(t, filepath.Dir(path), 0700)
						entries, err := os.ReadDir(filepath.Dir(path))
						require.NoError(t, err)
						assert.Len(t, entries, 2, "only result and metadata, never input snapshots")
						metadataPath := filepath.Join(filepath.Dir(path), evidence.MetadataFile)
						assertRetainedPrivate(t, metadataPath, 0600)
						metadata, err := os.ReadFile(metadataPath)
						require.NoError(t, err)
						assert.NotContains(t, string(metadata), "secret")
						var envelope map[string]any
						require.NoError(t, json.Unmarshal(metadata, &envelope))
						assert.Equal(t, item.CollectedAt, envelope["collected_at"])
						assert.Equal(t, "conftest_"+envelope["evaluation_id"].(string), item.ID)
						assert.Equal(t, "conftest-result_"+envelope["evaluation_id"].(string), ref.ID)
						assert.Contains(t, filepath.Base(filepath.Dir(path)), start.Format(evidence.EvaluationTimeFormat))
					}
				}
				assert.Len(t, seen, len(response.MappingReferences))
				for coordinate, original := range retained {
					data, err := os.ReadFile(filepath.Join(root, coordinate))
					require.NoError(t, err)
					assert.Equal(t, original, data)
				}
				require.NoError(t, os.WriteFile(inputFile, []byte(`{"changed":true}`), 0600))
			}
			require.NoError(t, os.RemoveAll(input))
			if kind == "remote" {
				assert.Len(t, cloneRoots, 2, "one replaceable clone per branch")
				assert.Len(t, retained, 6)
			} else {
				assert.Len(t, retained, 3)
			}
			legacyBytes, err := os.ReadFile(legacy)
			require.NoError(t, err)
			assert.Equal(t, "legacy summary", string(legacyBytes))
			copied := t.TempDir()
			require.NoError(t, os.CopyFS(copied, os.DirFS(root)))
			for coordinate, original := range retained {
				data, err := os.ReadFile(filepath.Join(copied, coordinate))
				require.NoError(t, err)
				assert.Equal(t, sha256.Sum256(original), sha256.Sum256(data))
				parsed, err := url.Parse(urls[coordinate])
				require.NoError(t, err)
				assert.Equal(t, filepath.Join(root, coordinate), parsed.Path)
			}
		})
	}
}

func assertRetainedPrivate(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	parent, err := os.OpenRoot(filepath.Dir(path))
	require.NoError(t, err)
	defer func() { require.NoError(t, parent.Close()) }()
	info, err := parent.Stat(filepath.Base(path))
	require.NoError(t, err)
	assert.Equal(t, mode, info.Mode().Perm())
	stat := info.Sys().(*syscall.Stat_t)
	assert.Equal(t, int64(os.Geteuid()), int64(stat.Uid))
	assert.Equal(t, int64(os.Getegid()), int64(stat.Gid))
}

type mockRunner struct {
	calls    []mockCall
	callFn   func(name string, args []string) ([]byte, error)
	outputFn func(name string, args []string) (scan.CommandOutput, error)
	response []byte
	stderr   []byte
	err      error
}

type mockCall struct {
	env  []string
	name string
	args []string
}

func (m *mockRunner) Run(name string, args ...string) (scan.CommandOutput, error) {
	m.calls = append(m.calls, mockCall{name: name, args: args})
	if m.outputFn != nil {
		return m.outputFn(name, args)
	}
	if m.callFn != nil {
		return m.callOutput(name, args)
	}
	return m.output(m.response, m.err)
}

func (m *mockRunner) RunWithEnv(env []string, name string, args ...string) (scan.CommandOutput, error) {
	m.calls = append(m.calls, mockCall{env: env, name: name, args: args})
	if m.outputFn != nil {
		return m.outputFn(name, args)
	}
	if m.callFn != nil {
		return m.callOutput(name, args)
	}
	return m.output(m.response, m.err)
}

func (m *mockRunner) callOutput(name string, args []string) (scan.CommandOutput, error) {
	data, err := m.callFn(name, args)
	return m.output(data, err)
}

func (m *mockRunner) output(data []byte, err error) (scan.CommandOutput, error) {
	// Existing failure fixtures describe tool diagnostics, now delivered on stderr.
	if err != nil {
		return scan.CommandOutput{Stderr: append(append([]byte(nil), m.stderr...), data...)}, err
	}
	return scan.CommandOutput{Stdout: data, Stderr: m.stderr}, nil
}

type mockLoader struct {
	calls  []mockLoaderCall
	loadFn func(target provider.Target, workDir string) (string, error)
}

type mockLoaderCall struct {
	target  provider.Target
	workDir string
}

func (m *mockLoader) Load(target provider.Target, workDir string) (string, error) {
	m.calls = append(m.calls, mockLoaderCall{target: target, workDir: workDir})
	if m.loadFn != nil {
		return m.loadFn(target, workDir)
	}
	return "", fmt.Errorf("mockLoader: not configured")
}

// --- Test fixtures ---

var conftestHappyJSON = `[{
  "filename": "deployment.yaml",
  "namespace": "main",
  "successes": 3,
  "failures": [
    {"msg": "Container must not run as root", "metadata": {"query": "data.kubernetes.run_as_root.deny"}}
  ],
  "warnings": [
    {"msg": "Resource limits should be set", "metadata": {"query": "data.kubernetes.resource_limits.warn"}}
  ]
}]`

var conftestAllSuccessJSON = `[{
  "filename": "deployment.yaml",
  "namespace": "main",
  "successes": 5
}]`

// --- Helpers ---

func newTestServer(
	t *testing.T, runner *mockRunner, ldr *mockLoader,
) *workspaceTestServer {
	t.Helper()
	if ldr == nil {
		ldr = &mockLoader{
			loadFn: func(_ provider.Target, _ string) (string, error) {
				return t.TempDir(), nil
			},
		}
	}
	return newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: t.TempDir(),
	})
}

func conftestRunner(testJSON string) *mockRunner {
	return &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				return []byte(testJSON), nil
			}
			return nil, fmt.Errorf("unexpected command: %s %v", name, args)
		},
	}
}

func makeScanRequest(
	t *testing.T, targets []provider.Target,
) *provider.ScanRequest {
	t.Helper()
	return &provider.ScanRequest{Targets: targets}
}

func localTarget(
	t *testing.T, inputPath, bundleRef string,
) provider.Target {
	t.Helper()
	return provider.Target{
		TargetID: "test-target",
		Variables: map[string]string{
			"input_path":     inputPath,
			"opa_bundle_ref": bundleRef,
		},
	}
}

func remoteTarget(bundleRef, url string) provider.Target {
	return provider.Target{
		TargetID: "test-target",
		Variables: map[string]string{
			"url":            url,
			"opa_bundle_ref": bundleRef,
			"branches":       "main",
		},
	}
}

// writeScanConfig writes a minimal scan config for tests that need Scan to
// work without a prior Generate call.
func writeScanConfig(t *testing.T, workDir string) {
	t.Helper()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, os.MkdirAll(cfg.GeneratedDirPath(), 0750))
	require.NoError(t, generate.WriteScanConfig(
		cfg.GeneratedDirPath(),
		[]string{"main"},
		map[string]string{"main": "test-req"},
		t.TempDir(),
	))
}

func writeIndexedScanConfig(
	t *testing.T,
	cfg *config.Config,
	contentKey string,
	ids []string,
	reverseMapping map[string]string,
	policyDir string,
) {
	t.Helper()
	generatedDir := cfg.GeneratedDirForKey(contentKey)
	require.NoError(t, os.MkdirAll(generatedDir, 0750))
	require.NoError(t, generate.WriteScanConfig(generatedDir, ids, reverseMapping, policyDir))
}

func writePolicyMapping(t *testing.T, policyDir, mapping string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(policyDir, 0750))
	require.NoError(t, os.WriteFile(
		filepath.Join(policyDir, generate.MappingFileName), []byte(mapping), 0600,
	))
}

// --- Constructor tests ---

func TestNew_ReturnsProviderServer(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{})
	require.NotNil(t, srv)
}

func TestNew_DefaultOptions(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{})
	assert.NotNil(t, srv.opts.Loader)
	assert.NotNil(t, srv.opts.Runner)
	assert.NotNil(t, srv.opts.ToolChecker)
	assert.NotEmpty(t, srv.opts.WorkspaceDir)
}

func TestNew_CustomOptions(t *testing.T) {
	ldr := &mockLoader{}
	runner := &mockRunner{}
	checker := func() ([]string, error) { return []string{"missing"}, nil }
	dir := t.TempDir()

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  checker,
		WorkspaceDir: dir,
	})
	assert.Equal(t, dir, srv.opts.WorkspaceDir)
	result, _ := srv.opts.ToolChecker()
	assert.Equal(t, []string{"missing"}, result)
}

// --- Describe tests ---

func TestDescribe_Healthy(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	resp, err := srv.Describe(
		context.Background(), &provider.DescribeRequest{},
	)
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "0.0.0-unknown", resp.Version)
}

func TestDescribe_Unhealthy(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{
		ToolChecker: func() ([]string, error) { return []string{"conftest"}, nil },
	})
	resp, err := srv.Describe(
		context.Background(), &provider.DescribeRequest{},
	)
	require.NoError(t, err)
	assert.False(t, resp.Healthy)
	assert.Contains(t, resp.ErrorMessage, "conftest")
}

func TestDescribe_Variables(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	resp, err := srv.Describe(
		context.Background(), &provider.DescribeRequest{},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.RequiredGlobalVariables,
		"no global variables should be required")
	assert.Empty(t, resp.RequiredTargetVariables,
		"url and input_path should not be in RequiredTargetVariables")
	assert.Equal(t, []string{"url|input_path"}, resp.OptionalTargetVariableGroups,
		"url and input_path should be declared as an optional group")
}

func TestDescribe_UsesToolPresenceOnly(t *testing.T) {
	runner := &mockRunner{}
	srv := newWorkspaceTestServer(t, ServerOptions{Runner: runner,
		ToolChecker: func() ([]string, error) { return nil, nil }})
	response, err := srv.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	assert.True(t, response.Healthy)
	assert.Empty(t, response.ErrorMessage)
	assert.Empty(t, runner.calls)
}

// --- Generate tests ---

func TestGenerate_EmptyConfiguration(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{})
	resp, err := srv.Generate(
		context.Background(), &provider.GenerateRequest{},
	)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "no assessment configurations provided")
}

func TestGenerate_WithMapping(t *testing.T) {
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())
	policyDir := cfg.PolicyDirForBundle(bundleRef)
	require.NoError(t, os.MkdirAll(policyDir, 0750))

	mappingData := `{
		"version": "1",
		"mappings": [
			{"id": "kubernetes.run_as_root", "requirement_id": "CIS-1"},
			{"id": "kubernetes.resource_limits", "requirement_id": "CIS-2"}
		]
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(policyDir, "complytime-mapping.json"),
		[]byte(mappingData), 0600,
	))

	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{"opa_bundle_ref": bundleRef},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
			{RequirementID: "CIS-2", PlanID: "ap-cis-2"},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify scan-config.json was written in the content-addressed directory.
	contentKey := genindex.ContentKey(
		[]string{"kubernetes.resource_limits", "kubernetes.run_as_root"}, policyDir,
	)
	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirForKey(contentKey))
	require.NoError(t, err)
	assert.Equal(t, []string{"kubernetes.resource_limits", "kubernetes.run_as_root"}, scanCfg.IDs)
	assert.Equal(t, "CIS-1", scanCfg.ReverseMapping["kubernetes.run_as_root"])
	assert.Equal(t, "CIS-2", scanCfg.ReverseMapping["kubernetes.resource_limits"])
}

func TestGenerate_MultipleTargets_WritesTargetIndexedConfigs(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	bundleRefA := "ghcr.io/org/bundle-a:v1"
	bundleRefB := "ghcr.io/org/bundle-b:v1"
	policyDirA := cfg.PolicyDirForBundle(bundleRefA)
	policyDirB := cfg.PolicyDirForBundle(bundleRefB)
	writePolicyMapping(t, policyDirA, `{"version":"1","mappings":[{"id":"namespace-a","requirement_id":"REQ-A"}]}`)
	writePolicyMapping(t, policyDirB, `{"version":"1","mappings":[{"id":"namespace-b","requirement_id":"REQ-B"}]}`)

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       &mockRunner{response: []byte("ok")},
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	targetURLA := "https://example.com/target-a"
	targetURLB := "https://example.com/target-b"
	for _, req := range []*provider.GenerateRequest{
		{
			TargetVariables: map[string]string{"url": targetURLA, "opa_bundle_ref": bundleRefA},
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "REQ-A"}},
		},
		{
			TargetVariables: map[string]string{"url": targetURLB, "opa_bundle_ref": bundleRefB},
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "REQ-B"}},
		},
	} {
		response, err := srv.Generate(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, response.Success, response.ErrorMessage)
	}

	contentKeyA := genindex.ContentKey([]string{"namespace-a"}, policyDirA)
	contentKeyB := genindex.ContentKey([]string{"namespace-b"}, policyDirB)
	assert.NotEqual(t, contentKeyA, contentKeyB)
	configA, err := generate.ReadScanConfig(cfg.GeneratedDirForKey(contentKeyA))
	require.NoError(t, err)
	configB, err := generate.ReadScanConfig(cfg.GeneratedDirForKey(contentKeyB))
	require.NoError(t, err)
	assert.Equal(t, []string{"namespace-a"}, configA.IDs)
	assert.Equal(t, policyDirA, configA.PolicyDir)
	assert.Equal(t, []string{"namespace-b"}, configB.IDs)
	assert.Equal(t, policyDirB, configB.PolicyDir)

	index, err := genindex.ReadIndex(cfg.GeneratedDirPath())
	require.NoError(t, err)
	assert.Equal(t, contentKeyA, index.Targets[genindex.TargetKey(targetURLA)])
	assert.Equal(t, contentKeyB, index.Targets[genindex.TargetKey(targetURLB)])
}

func TestGenerate_IdenticalRequirementsDifferentPolicies_UsesDistinctContentKeys(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	bundleRefA := "ghcr.io/org/bundle-a:v1"
	bundleRefB := "ghcr.io/org/bundle-b:v1"
	policyDirA := cfg.PolicyDirForBundle(bundleRefA)
	policyDirB := cfg.PolicyDirForBundle(bundleRefB)
	mapping := `{"version":"1","mappings":[{"id":"shared-namespace","requirement_id":"REQ-1"}]}`
	writePolicyMapping(t, policyDirA, mapping)
	writePolicyMapping(t, policyDirB, mapping)

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       &mockRunner{response: []byte("ok")},
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	for _, variables := range []map[string]string{
		{"url": "https://example.com/target-a", "opa_bundle_ref": bundleRefA},
		{"url": "https://example.com/target-b", "opa_bundle_ref": bundleRefB},
	} {
		response, err := srv.Generate(context.Background(), &provider.GenerateRequest{
			TargetVariables: variables,
			Configuration:   []provider.AssessmentConfiguration{{RequirementID: "REQ-1"}},
		})
		require.NoError(t, err)
		assert.True(t, response.Success, response.ErrorMessage)
	}

	contentKeyA := genindex.ContentKey([]string{"shared-namespace"}, policyDirA)
	contentKeyB := genindex.ContentKey([]string{"shared-namespace"}, policyDirB)
	assert.NotEqual(t, contentKeyA, contentKeyB)
	assert.FileExists(t, filepath.Join(cfg.GeneratedDirForKey(contentKeyA), generate.ScanConfigFileName))
	assert.FileExists(t, filepath.Join(cfg.GeneratedDirForKey(contentKeyB), generate.ScanConfigFileName))
}

func TestGenerate_InputPathTarget_IndexesInputPath(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	bundleRef := "ghcr.io/org/bundle:v1"
	policyDir := cfg.PolicyDirForBundle(bundleRef)
	writePolicyMapping(t, policyDir, `{"version":"1","mappings":[{"id":"namespace","requirement_id":"REQ-1"}]}`)
	inputPath := filepath.Join(t.TempDir(), "input.yaml")

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       &mockRunner{response: []byte("ok")},
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	response, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{
			"input_path":     inputPath,
			"opa_bundle_ref": bundleRef,
		},
		Configuration: []provider.AssessmentConfiguration{{RequirementID: "REQ-1"}},
	})
	require.NoError(t, err)
	assert.True(t, response.Success, response.ErrorMessage)

	index, err := genindex.ReadIndex(cfg.GeneratedDirPath())
	require.NoError(t, err)
	assert.Equal(t, genindex.ContentKey([]string{"namespace"}, policyDir),
		index.Targets[genindex.TargetKey(inputPath)])
}

func TestGenerate_WithoutMapping(t *testing.T) {
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())
	policyDir := cfg.PolicyDirForBundle(bundleRef)
	require.NoError(t, os.MkdirAll(policyDir, 0750))
	// No mapping file -- should return error, not fallback.

	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{"opa_bundle_ref": bundleRef},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, generate.MappingFileName)

	// Verify no scan-config.json was written.
	_, readErr := generate.ReadScanConfig(cfg.GeneratedDirPath())
	assert.Error(t, readErr, "no scan config should be written when mapping is missing")
}

func TestGenerate_MissingBundleRef(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       &mockRunner{},
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: t.TempDir(),
	})

	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "opa_bundle_ref")
}

func TestGenerate_WithComplypackContentPath(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	// Create a complypack content directory with Rego + mapping.
	complypackDir := filepath.Join(t.TempDir(), "complypack-content")
	require.NoError(t, os.MkdirAll(complypackDir, 0750))

	mappingContent := `{
		"version": "1.0.0",
		"mappings": [
			{"id": "kubernetes.check_one", "requirement_id": "REQ-1"}
		]
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(complypackDir, generate.MappingFileName),
		[]byte(mappingContent), 0600,
	))

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       &mockRunner{},
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	// Generate with ComplypackContentPath — should NOT require opa_bundle_ref
	// and should NOT call conftest pull.
	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		ComplypackContentPath: complypackDir,
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "REQ-1", PlanID: "ap-req-1"},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "generate should succeed with complypack path: %s", resp.ErrorMessage)

	// Verify scan-config.json was written with the complypack dir as PolicyDir.
	contentKey := genindex.ContentKey([]string{"kubernetes.check_one"}, complypackDir)
	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirForKey(contentKey))
	require.NoError(t, err)
	assert.Equal(t, complypackDir, scanCfg.PolicyDir,
		"scan config PolicyDir should point to the complypack content path")
	assert.Contains(t, scanCfg.IDs, "kubernetes.check_one")
	assert.Equal(t, "REQ-1", scanCfg.ReverseMapping["kubernetes.check_one"])
}

func TestGenerate_ComplypackPreferredOverBundleRef(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	complypackDir := filepath.Join(t.TempDir(), "complypack-content")
	require.NoError(t, os.MkdirAll(complypackDir, 0750))

	mappingContent := `{
		"version": "1.0.0",
		"mappings": [
			{"id": "kubernetes.check_one", "requirement_id": "REQ-1"}
		]
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(complypackDir, generate.MappingFileName),
		[]byte(mappingContent), 0600,
	))

	pullCalled := false
	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			pullCalled = true
		}
		return []byte("ok"), nil
	}}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	// Provide BOTH complypack path and opa_bundle_ref — complypack should win.
	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		ComplypackContentPath: complypackDir,
		TargetVariables:       map[string]string{"opa_bundle_ref": "ghcr.io/org/bundle:v1"},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "REQ-1", PlanID: "ap-req-1"},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.False(t, pullCalled, "conftest pull should NOT be called when complypack path is provided")
}

// buildTarGz creates a tar.gz archive containing the given files and returns
// the archive bytes. Each entry in files maps a relative path to its content.
func buildTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestGenerate_WithComplypackContentPath_TarGz(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	// Build a tar.gz archive containing the mapping file.
	mappingContent := `{
		"version": "1.0.0",
		"mappings": [
			{"id": "kubernetes.check_one", "requirement_id": "REQ-1"}
		]
	}`
	archiveBytes := buildTarGz(t, map[string][]byte{
		generate.MappingFileName: []byte(mappingContent),
	})

	// Write the archive to a file, mimicking complyctl's cache layout:
	// ~/.cache/complytime/complypacks/opa/1.0.0/content.tar.gz
	complypackCacheDir := filepath.Join(t.TempDir(), "opa", "1.0.0")
	require.NoError(t, os.MkdirAll(complypackCacheDir, 0750))
	archivePath := filepath.Join(complypackCacheDir, "content.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, archiveBytes, 0600))

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       &mockRunner{},
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	// Generate with a tar.gz ComplypackContentPath — should extract and
	// read the mapping file from the extracted directory.
	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		ComplypackContentPath: archivePath,
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "REQ-1", PlanID: "ap-req-1"},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success,
		"generate should succeed with tar.gz complypack path: %s",
		resp.ErrorMessage)

	// Verify scan-config.json was written with the extracted directory
	// (not the tar.gz path) as PolicyDir.
	contentKey := genindex.ContentKey([]string{"kubernetes.check_one"}, filepath.Join(
		complypackCacheDir, "content"))
	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirForKey(contentKey))
	require.NoError(t, err)
	assert.NotEqual(t, archivePath, scanCfg.PolicyDir,
		"PolicyDir should be the extracted directory, not the archive")
	assert.DirExists(t, scanCfg.PolicyDir,
		"PolicyDir should be an existing directory")
	assert.Contains(t, scanCfg.IDs, "kubernetes.check_one")
	assert.Equal(t, "REQ-1",
		scanCfg.ReverseMapping["kubernetes.check_one"])

	// Verify the mapping file exists in the extracted directory.
	mappingPath := filepath.Join(
		scanCfg.PolicyDir, generate.MappingFileName,
	)
	assert.FileExists(t, mappingPath)
}

func TestGenerate_WithComplypackContentPath_TarGz_Idempotent(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	mappingContent := `{
		"version": "1.0.0",
		"mappings": [
			{"id": "kubernetes.check_one", "requirement_id": "REQ-1"}
		]
	}`
	archiveBytes := buildTarGz(t, map[string][]byte{
		generate.MappingFileName: []byte(mappingContent),
	})

	complypackCacheDir := filepath.Join(t.TempDir(), "opa", "1.0.0")
	require.NoError(t, os.MkdirAll(complypackCacheDir, 0750))
	archivePath := filepath.Join(complypackCacheDir, "content.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, archiveBytes, 0600))

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       &mockRunner{},
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	req := &provider.GenerateRequest{
		ComplypackContentPath: archivePath,
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "REQ-1", PlanID: "ap-req-1"},
		},
	}

	// Run Generate twice — second run should reuse the extracted dir.
	resp1, err := srv.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp1.Success)

	resp2, err := srv.Generate(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp2.Success)
}

func TestGenerate_ToolCheckFailure(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{
		ToolChecker: func() ([]string, error) {
			return []string{"conftest"}, nil
		},
		WorkspaceDir: t.TempDir(),
	})

	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "conftest")
}

func TestGenerate_BundlePullFailure(t *testing.T) {
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			return []byte("auth failed"), fmt.Errorf("exit status 1")
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: t.TempDir(),
	})

	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{"opa_bundle_ref": "ghcr.io/org/bundle:v1"},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "pulling policy bundle")
}

func TestGenerate_MalformedMapping(t *testing.T) {
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())
	policyDir := cfg.PolicyDirForBundle(bundleRef)
	require.NoError(t, os.MkdirAll(policyDir, 0750))

	// Write an invalid mapping file (malformed JSON).
	require.NoError(t, os.WriteFile(
		filepath.Join(policyDir, generate.MappingFileName),
		[]byte("{invalid json"), 0600,
	))

	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{"opa_bundle_ref": bundleRef},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.ErrorMessage, "invalid mapping file")
}

func TestGenerate_PartialMatch(t *testing.T) {
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())
	policyDir := cfg.PolicyDirForBundle(bundleRef)
	require.NoError(t, os.MkdirAll(policyDir, 0750))

	mappingData := `{
		"version": "1",
		"mappings": [
			{"id": "ns1", "requirement_id": "CIS-1"}
		]
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(policyDir, "complytime-mapping.json"),
		[]byte(mappingData), 0600,
	))

	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{"opa_bundle_ref": bundleRef},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
			{RequirementID: "CIS-MISSING", PlanID: "ap-cis-missing"},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	contentKey := genindex.ContentKey([]string{"ns1"}, policyDir)
	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirForKey(contentKey))
	require.NoError(t, err)
	assert.Equal(t, []string{"ns1"}, scanCfg.IDs)
	assert.Equal(t, "CIS-1", scanCfg.ReverseMapping["ns1"])
}

func TestGenerate_Scan_WithMapping(t *testing.T) {
	// Integration test: Generate with mapping → Scan with namespace filtering →
	// verify Gemara IDs in response and --namespace flags used (not --all-namespaces).
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())
	policyDir := cfg.PolicyDirForBundle(bundleRef)
	require.NoError(t, os.MkdirAll(policyDir, 0750))

	mappingData := `{
		"version": "1",
		"mappings": [
			{"id": "kubernetes.run_as_root", "requirement_id": "CIS-K8S-5.2.6"}
		]
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(policyDir, "complytime-mapping.json"),
		[]byte(mappingData), 0600,
	))

	var usedNamespaceFlags bool
	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "conftest" && len(args) > 0 && args[0] == "test" {
			for _, a := range args {
				if a == "--namespace" {
					usedNamespaceFlags = true
				}
				if a == "--all-namespaces" {
					t.Error("expected --namespace flags, got --all-namespaces")
				}
			}
			return []byte(conftestHappyJSON), nil
		}
		return nil, fmt.Errorf("unexpected: %s", name)
	}}

	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	inputPath := t.TempDir()
	// Step 1: Generate with mapping.
	genResp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{
			"input_path":     inputPath,
			"opa_bundle_ref": bundleRef,
		},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-K8S-5.2.6", PlanID: "ap-cis-k8s-5.2.6"},
		},
	})
	require.NoError(t, err)
	assert.True(t, genResp.Success)

	// Step 2: Scan.
	scanResp, err := srv.Scan(context.Background(), &provider.ScanRequest{
		Targets: []provider.Target{
			{
				TargetID: "local-test",
				Variables: map[string]string{
					"input_path":     inputPath,
					"opa_bundle_ref": bundleRef,
				},
			},
		},
	})
	require.NoError(t, err)
	assert.True(t, usedNamespaceFlags, "should use --namespace flags, not --all-namespaces")

	// Verify Gemara IDs (not Rego-derived IDs) in response.
	var reqIDs []string
	for _, a := range scanResp.Assessments {
		reqIDs = append(reqIDs, a.RequirementID)
	}
	assert.Contains(t, reqIDs, "CIS-K8S-5.2.6")
	assert.NotContains(t, reqIDs, "kubernetes.run_as_root")
}

func TestMergeVariables(t *testing.T) {
	tests := []struct {
		name     string
		global   map[string]string
		target   map[string]string
		expected map[string]string
	}{
		{
			"global only",
			map[string]string{"opa_bundle_ref": "ghcr.io/bundle:v1"},
			nil,
			map[string]string{"opa_bundle_ref": "ghcr.io/bundle:v1"},
		},
		{
			"target overrides global",
			map[string]string{"opa_bundle_ref": "global-ref"},
			map[string]string{"opa_bundle_ref": "target-ref"},
			map[string]string{"opa_bundle_ref": "target-ref"},
		},
		{
			"both nil",
			nil,
			nil,
			map[string]string{},
		},
		{
			"merge distinct keys",
			map[string]string{"key1": "val1"},
			map[string]string{"key2": "val2"},
			map[string]string{"key1": "val1", "key2": "val2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeVariables(tc.global, tc.target)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerate_Scan_MissingMappingReturnsError(t *testing.T) {
	// Integration test: Generate without mapping → error response.
	// Scan should not be reached because Generate fails.
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"

	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		return nil, fmt.Errorf("unexpected: %s", name)
	}}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	genResp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{"opa_bundle_ref": bundleRef},
		Configuration: []provider.AssessmentConfiguration{
			{RequirementID: "CIS-1", PlanID: "ap-cis-1"},
		},
	})
	require.NoError(t, err)
	assert.False(t, genResp.Success, "Generate should fail when mapping file is missing")
	assert.Contains(t, genResp.ErrorMessage, generate.MappingFileName)
}

// --- Scan: happy path tests ---

func TestScan_LocalPath_HappyPath(t *testing.T) {
	dir := t.TempDir()
	workDir := t.TempDir()
	writeScanConfig(t, workDir)

	runner := conftestRunner(conftestHappyJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return dir, nil
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, dir, "ghcr.io/org/bundle:dev"),
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Assessments)
}

func TestScan_LocalPath_DeletedAfterEvaluationRetainsResults(t *testing.T) {
	// Input provenance may disappear after conftest finishes; result bytes remain evidence.
	workDir := t.TempDir()
	writeScanConfig(t, workDir)

	inputPath := filepath.Join(t.TempDir(), "vanished")
	require.NoError(t, os.WriteFile(inputPath, []byte("{}"), 0600))
	runner := &mockRunner{callFn: func(_ string, _ []string) ([]byte, error) {
		require.NoError(t, os.Remove(inputPath))
		return []byte(conftestHappyJSON), nil
	}}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return inputPath, nil
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, inputPath, "ghcr.io/org/bundle:dev"),
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Empty(t, resp.Errors)
	require.NotEmpty(t, resp.Assessments)
	item := resp.Assessments[0].Evidence[0]
	assert.Equal(t, "conftest.json", filepath.Base(item.Source.Coordinate))
	stored, err := os.ReadFile(filepath.Join(workDir, item.Source.Coordinate))
	require.NoError(t, err)
	assert.Equal(t, conftestHappyJSON, string(stored))
	assert.NoFileExists(t, inputPath)
}

func TestScan_RemoteURL_HappyPath(t *testing.T) {
	workDir := t.TempDir()
	writeScanConfig(t, workDir)

	runner := conftestRunner(conftestHappyJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	target := remoteTarget(
		"ghcr.io/org/bundle:dev", "https://github.com/org/repo",
	)
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Assessments)
}

func TestScan_IndexResolvesURLAndInputPathTargets(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewWorkspaceConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())

	urlTarget := remoteTarget("ghcr.io/org/bundle-a:v1", "https://example.com/repo-a")
	urlTarget.TargetID = "url-target"
	inputDir := t.TempDir()
	inputTarget := localTarget(t, inputDir, "ghcr.io/org/bundle-b:v1")
	inputTarget.TargetID = "input-target"

	policyDirA := t.TempDir()
	policyDirB := t.TempDir()
	contentKeyA := "aaaaaaaaaaaaaaaa"
	contentKeyB := "bbbbbbbbbbbbbbbb"
	writeIndexedScanConfig(
		t, cfg, contentKeyA, []string{"namespace-a"},
		map[string]string{"namespace-a": "REQ-A", "pass-a": "REQ-A-PASS"}, policyDirA,
	)
	writeIndexedScanConfig(
		t, cfg, contentKeyB, []string{"namespace-b"},
		map[string]string{"namespace-b": "REQ-B", "clear-b": "REQ-B-CLEAR"}, policyDirB,
	)
	require.NoError(t, genindex.WriteIndex(cfg.GeneratedDirPath(), genindex.GenerateIndex{
		Version: 1,
		Targets: map[string]string{
			genindex.TargetKey(urlTarget.Variables["url"]):          contentKeyA,
			genindex.TargetKey(inputTarget.Variables["input_path"]): contentKeyB,
		},
	}))

	var policyDirs []string
	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		if name != "conftest" || len(args) == 0 || args[0] != "test" {
			return nil, fmt.Errorf("unexpected command: %s %v", name, args)
		}
		for index, argument := range args {
			if argument == "--policy" && index+1 < len(args) {
				policyDirs = append(policyDirs, args[index+1])
			}
		}
		for _, argument := range args {
			if argument == "namespace-a" {
				return []byte(`[{"filename":"input-a","namespace":"namespace-a","failures":[{"msg":"failed a","metadata":{"query":"data.namespace-a.deny"}}]}]`), nil
			}
		}
		return []byte(`[{"filename":"input-b","namespace":"namespace-b","failures":[{"msg":"failed b","metadata":{"query":"data.namespace-b.deny"}}]}]`), nil
	}}
	remoteInputDir := t.TempDir()
	ldr := &mockLoader{loadFn: func(target provider.Target, _ string) (string, error) {
		if target.Variables["url"] != "" {
			return remoteInputDir, nil
		}
		return target.Variables["input_path"], nil
	}}
	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	resp, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{urlTarget, inputTarget}))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{policyDirA, policyDirB}, policyDirs)

	assessments := make(map[string]provider.AssessmentLog)
	for _, assessment := range resp.Assessments {
		assessments[assessment.RequirementID] = assessment
	}
	assert.Contains(t, assessments, "REQ-A")
	assert.Contains(t, assessments, "REQ-B")
	assert.Contains(t, assessments, "REQ-A-PASS")
	assert.Contains(t, assessments, "REQ-B-CLEAR")
	assert.Len(t, assessments["REQ-A-PASS"].Steps, 1)
	assert.Equal(t, "https://example.com/repo-a@main", assessments["REQ-A-PASS"].Steps[0].Name)
	assert.Len(t, assessments["REQ-B-CLEAR"].Steps, 1)
	assert.Equal(t, inputDir, assessments["REQ-B-CLEAR"].Steps[0].Name)
}

func TestScan_PresentIndexFailuresDoNotUseLegacyConfig(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		index         genindex.GenerateIndex
		target        provider.Target
		errorFragment string
	}{
		{
			name: "invalid content key",
			index: genindex.GenerateIndex{Version: 1, Targets: map[string]string{
				genindex.TargetKey("https://example.com/target"): "../legacy",
			}},
			target:        remoteTarget("ghcr.io/org/bundle:v1", "https://example.com/target"),
			errorFragment: "validating content key",
		},
		{
			name:          "missing target",
			index:         genindex.GenerateIndex{Version: 1, Targets: map[string]string{}},
			target:        remoteTarget("ghcr.io/org/bundle:v1", "https://example.com/target"),
			errorFragment: "missing from generate index",
		},
		{
			name: "missing content artifact",
			index: genindex.GenerateIndex{Version: 1, Targets: map[string]string{
				genindex.TargetKey("https://example.com/target"): "cccccccccccccccc",
			}},
			target:        remoteTarget("ghcr.io/org/bundle:v1", "https://example.com/target"),
			errorFragment: "reading generated scan config",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			workDir := t.TempDir()
			cfg := config.NewWorkspaceConfig(workDir)
			require.NoError(t, cfg.EnsureDirectories())
			writeScanConfig(t, workDir)
			require.NoError(t, genindex.WriteIndex(cfg.GeneratedDirPath(), testCase.index))

			runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
				return nil, fmt.Errorf("legacy config must not be evaluated: %s %v", name, args)
			}}
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner:       runner,
				ToolChecker:  func() ([]string, error) { return nil, nil },
				WorkspaceDir: workDir,
			})

			_, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{testCase.target}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.target.TargetID)
			assert.Contains(t, err.Error(), testCase.errorFragment)
			assert.Empty(t, runner.calls)
		})
	}
}

func TestScan_UnusableIndexDoesNotUseLegacyConfig(t *testing.T) {
	for name, indexContents := range map[string]string{
		"malformed":           "{not json",
		"unsupported version": `{"version":2,"targets":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			workDir := t.TempDir()
			cfg := config.NewWorkspaceConfig(workDir)
			require.NoError(t, cfg.EnsureDirectories())
			writeScanConfig(t, workDir)
			require.NoError(t, os.WriteFile(
				filepath.Join(cfg.GeneratedDirPath(), "generate-index.json"), []byte(indexContents), 0600,
			))

			runner := &mockRunner{}
			srv := newWorkspaceTestServer(t, ServerOptions{
				Runner:       runner,
				ToolChecker:  func() ([]string, error) { return nil, nil },
				WorkspaceDir: workDir,
			})
			target := remoteTarget("ghcr.io/org/bundle:v1", "https://example.com/target")

			_, err := srv.Scan(context.Background(), makeScanRequest(t, []provider.Target{target}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), target.TargetID)
			assert.Contains(t, err.Error(), "reading generate index")
			assert.Empty(t, runner.calls)
		})
	}
}

func TestScan_RemoteURL_WithBranches(t *testing.T) {
	workDir := t.TempDir()
	writeScanConfig(t, workDir)

	conftestTestCount := 0
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				conftestTestCount++
				return []byte(conftestHappyJSON), nil
			}
			return nil, fmt.Errorf(
				"unexpected command: %s %v", name, args,
			)
		},
	}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"url":            "https://github.com/org/repo",
			"opa_bundle_ref": "ghcr.io/org/bundle:dev",
			"branches":       "main, develop",
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, ldr.calls, 2, "should load for each branch")
	assert.Equal(t, 2, conftestTestCount, "should test for each branch")
}

func TestScan_RemoteURL_WithScanPath(t *testing.T) {
	workDir := t.TempDir()
	writeScanConfig(t, workDir)

	scanSubPath := filepath.Join(t.TempDir(), "configs", "k8s")
	require.NoError(t, os.MkdirAll(scanSubPath, 0750))
	var conftestTestPath string
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				conftestTestPath = args[1]
				return []byte(conftestAllSuccessJSON), nil
			}
			return nil, fmt.Errorf(
				"unexpected command: %s %v", name, args,
			)
		},
	}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return scanSubPath, nil
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"url":            "https://github.com/org/repo",
			"opa_bundle_ref": "ghcr.io/org/bundle:dev",
			"branches":       "main",
			"scan_path":      "configs/k8s",
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	_, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, conftestTestPath, filepath.Join("configs", "k8s"))
}

func TestScan_RemoteURL_WithAccessToken(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{
		loadFn: func(
			target provider.Target, _ string,
		) (string, error) {
			assert.Equal(
				t, "ghp_secrettoken123",
				target.Variables["access_token"],
			)
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"url":            "https://github.com/org/repo",
			"opa_bundle_ref": "ghcr.io/org/bundle:dev",
			"branches":       "main",
			"access_token":   "ghp_secrettoken123",
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	_, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
}

func TestScan_RemoteURL_UnauthenticatedClone(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{
		loadFn: func(
			target provider.Target, _ string,
		) (string, error) {
			assert.Empty(t, target.Variables["access_token"])
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	target := remoteTarget(
		"ghcr.io/org/bundle:dev", "https://github.com/org/repo",
	)
	req := makeScanRequest(t, []provider.Target{target})
	_, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
}

// --- Scan: error path tests ---

func TestScan_NoTargets(t *testing.T) {
	srv := newTestServer(t, &mockRunner{}, nil)
	req := makeScanRequest(t, nil)
	_, err := srv.Scan(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one target")
}

func TestScan_WithoutPriorGenerate_ReturnsError(t *testing.T) {
	runner := conftestRunner(conftestHappyJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	// No writeScanConfig -- Scan runs without a prior Generate call.
	srv := newTestServer(t, runner, ldr)
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev"),
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// The error should be reported in resp.Errors since there is no scan config.
	require.NotEmpty(t, resp.Errors)
	assert.Contains(t, resp.Errors[0],
		generate.MappingFileName,
		"error should mention the mapping file name")
}

func TestScan_ToolCheckFailure(t *testing.T) {
	srv := newWorkspaceTestServer(t, ServerOptions{
		ToolChecker:  func() ([]string, error) { return []string{"conftest"}, nil },
		WorkspaceDir: t.TempDir(),
	})
	target := localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev")
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Errors, 1)
	assert.Contains(t, resp.Errors[0], target.TargetID)
	assert.Contains(t, resp.Errors[0], "install conftest and make it available on PATH")
	assert.Empty(t, resp.Assessments)
}

func TestScan_MissingBundleRef(t *testing.T) {
	srv := newTestServer(t, &mockRunner{}, nil)
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"input_path": t.TempDir(),
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Errors,
		"missing bundle ref should produce an error in resp.Errors")
}

func TestScan_BothURLAndInputPath(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newTestServer(t, runner, nil)
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"url":            "https://github.com/org/repo",
			"input_path":     "/some/path",
			"opa_bundle_ref": "ghcr.io/org/bundle:dev",
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_NeitherURLNorInputPath(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newTestServer(t, runner, nil)
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"opa_bundle_ref": "ghcr.io/org/bundle:dev",
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_PathTraversal(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	tests := []struct {
		name   string
		target provider.Target
	}{
		{
			name: "traversal in scan_path",
			target: provider.Target{
				TargetID: "test",
				Variables: map[string]string{
					"url":            "https://github.com/org/repo",
					"opa_bundle_ref": "ghcr.io/org/bundle:dev",
					"scan_path":      "../../../etc",
				},
			},
		},
		{
			name: "traversal in branch",
			target: provider.Target{
				TargetID: "test",
				Variables: map[string]string{
					"url":            "https://github.com/org/repo",
					"opa_bundle_ref": "ghcr.io/org/bundle:dev",
					"branches":       "main/../../../etc",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t, runner, nil)
			req := makeScanRequest(t, []provider.Target{tc.target})
			resp, err := srv.Scan(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, resp)
		})
	}
}

func TestScan_ConftestPullFailure(t *testing.T) {
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				return []byte("auth failed"), fmt.Errorf("exit status 1")
			}
			return []byte("ok"), nil
		},
	}

	srv := newTestServer(t, runner, nil)
	target := localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev")
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Errors,
		"conftest pull failure should produce an error in resp.Errors")
}

func TestScan_ConftestTestFailure(t *testing.T) {
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				return []byte("eval error"), fmt.Errorf("exit status 2")
			}
			return nil, fmt.Errorf("unexpected command")
		},
	}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	target := localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev")
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_LoaderFailure(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return "", fmt.Errorf("clone failed: repository not found")
		},
	}

	srv := newTestServer(t, runner, ldr)
	target := remoteTarget(
		"ghcr.io/org/bundle:dev", "https://github.com/org/repo",
	)
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_MultipleTargets_PartialFailure(t *testing.T) {
	evalCount := 0
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				evalCount++
				if evalCount == 1 {
					return []byte("error"), fmt.Errorf("exit status 2")
				}
				return []byte(conftestAllSuccessJSON), nil
			}
			return nil, fmt.Errorf("unexpected command")
		},
	}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	srv := newTestServer(t, runner, ldr)
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, dir1, "ghcr.io/org/bundle:dev"),
		{
			TargetID: "target2",
			Variables: map[string]string{
				"input_path":     dir2,
				"opa_bundle_ref": "ghcr.io/org/bundle:dev",
			},
		},
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_InvalidURL(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	srv := newTestServer(t, runner, nil)
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"url":            "http://github.com/org/repo",
			"opa_bundle_ref": "ghcr.io/org/bundle:dev",
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// --- Per-target bundle tests ---

func TestScan_PerTargetBundles_DifferentRefs(t *testing.T) {
	var pullRefs []string
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				pullRefs = append(pullRefs, args[1])
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				return []byte(conftestAllSuccessJSON), nil
			}
			return nil, fmt.Errorf(
				"unexpected command: %s %v", name, args,
			)
		},
	}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle-a:v1"),
		{
			TargetID: "target2",
			Variables: map[string]string{
				"input_path":     t.TempDir(),
				"opa_bundle_ref": "ghcr.io/org/bundle-b:v2",
			},
		},
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, pullRefs, 2, "should pull two different bundles")
}

func TestScan_PerTargetBundles_SameRef(t *testing.T) {
	pullCount := 0
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				pullCount++
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				return []byte(conftestAllSuccessJSON), nil
			}
			return nil, fmt.Errorf(
				"unexpected command: %s %v", name, args,
			)
		},
	}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev"),
		{
			TargetID: "target2",
			Variables: map[string]string{
				"input_path":     t.TempDir(),
				"opa_bundle_ref": "ghcr.io/org/bundle:dev",
			},
		},
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(
		t, 1, pullCount, "should pull bundle only once due to cache",
	)
}

func TestScan_PerTargetBundles_MissingRef(t *testing.T) {
	workDir := t.TempDir()
	writeScanConfig(t, workDir)

	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newWorkspaceTestServer(t, ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev"),
		{
			TargetID:  "missing-ref",
			Variables: map[string]string{"input_path": t.TempDir()},
		},
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	// The missing-ref target falls back to the scan config's PolicyDir,
	// so both targets scan successfully with no findings.
	assert.Empty(t, resp.Errors)
}

func TestScan_BundlePullFailure_PartialScan(t *testing.T) {
	pullCount := 0
	runner := &mockRunner{
		callFn: func(name string, args []string) ([]byte, error) {
			if name == "conftest" && len(args) > 0 && args[0] == "pull" {
				pullCount++
				if pullCount == 1 {
					return []byte("auth failed"),
						fmt.Errorf("exit status 1")
				}
				return []byte("ok"), nil
			}
			if name == "conftest" && len(args) > 0 && args[0] == "test" {
				return []byte(conftestAllSuccessJSON), nil
			}
			return nil, fmt.Errorf(
				"unexpected command: %s %v", name, args,
			)
		},
	}
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle-a:v1"),
		{
			TargetID: "target2",
			Variables: map[string]string{
				"input_path":     t.TempDir(),
				"opa_bundle_ref": "ghcr.io/org/bundle-b:v2",
			},
		},
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_WriteError_PopulatesRespErrors(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	wsDir := t.TempDir()
	writeScanConfig(t, wsDir)
	srv := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner,
		Loader:       ldr,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: wsDir,
	})

	srv.opts.Allocate = func(options evidence.StorageOptions, labels ...string) (*evidence.Evaluation, error) {
		options.Operations.Write = func(*os.File, []byte) (int, error) {
			return 0, fmt.Errorf("injected required-output write failure")
		}
		return evidence.Allocate(options, labels...)
	}

	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev"),
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err, "write errors should not cause gRPC-level failure")
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Errors, "write errors should appear in resp.Errors")
	assert.Contains(t, resp.Errors[0], "injected required-output write failure")
	assert.Empty(t, resp.Assessments)
	assert.Empty(t, resp.MappingReferences)
}

// --- Validation tests ---

func TestValidateTargetVariables_Valid(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		inputPath   string
		branches    string
		scanPath    string
		accessToken string
	}{
		{"url only", "https://github.com/org/repo", "", "main", "", ""},
		{"input_path only", "", "/some/path", "", "", ""},
		{"url with token", "https://github.com/org/repo", "", "main", "", "ghp_token"},
		{"url with scan_path", "https://github.com/org/repo", "", "main", "configs", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTargetVariables(
				tc.url, tc.inputPath, tc.branches,
				tc.scanPath, tc.accessToken,
			)
			assert.NoError(t, err)
		})
	}
}

func TestValidateTargetVariables_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		inputPath   string
		branches    string
		scanPath    string
		accessToken string
		errContains string
	}{
		{
			"both url and input_path",
			"https://github.com/org/repo", "/path", "", "", "",
			"specify either url or input_path",
		},
		{
			"neither url nor input_path",
			"", "", "", "", "",
			"url or input_path",
		},
		{
			"traversal in scan_path",
			"https://github.com/org/repo", "", "main", "../etc", "",
			"traversal",
		},
		{
			"traversal in branches",
			"https://github.com/org/repo", "", "main/../etc", "", "",
			"traversal",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTargetVariables(
				tc.url, tc.inputPath, tc.branches,
				tc.scanPath, tc.accessToken,
			)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"main", []string{"main"}},
		{"main, develop", []string{"main", "develop"}},
		{"main,develop,feature/x", []string{"main", "develop", "feature/x"}},
		{" main , develop ", []string{"main", "develop"}},
		{"", nil},
		{",,,", nil},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := splitCSV(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestProviderInterface(t *testing.T) {
	var _ provider.Provider = (*ProviderServer)(nil)
}

// --- No-globals test ---

func TestScan_NoGlobals(t *testing.T) {
	runner1 := conftestRunner(conftestHappyJSON)
	runner2 := conftestRunner(conftestAllSuccessJSON)
	ldr1 := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}
	ldr2 := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv1 := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner1,
		Loader:       ldr1,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: t.TempDir(),
	})
	srv2 := newWorkspaceTestServer(t, ServerOptions{
		Runner:       runner2,
		Loader:       ldr2,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: t.TempDir(),
	})

	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev"),
	})
	resp1, err := srv1.Scan(context.Background(), req)
	require.NoError(t, err)
	resp2, err := srv2.Scan(context.Background(), req)
	require.NoError(t, err)

	require.NotNil(t, resp1)
	require.NotNil(t, resp2)
}
