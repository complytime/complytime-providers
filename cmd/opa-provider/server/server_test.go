// SPDX-License-Identifier: Apache-2.0

package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
)

// --- Mocks ---

type mockRunner struct {
	calls    []mockCall
	callFn   func(name string, args []string) ([]byte, error)
	response []byte
	err      error
}

type mockCall struct {
	env  []string
	name string
	args []string
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	m.calls = append(m.calls, mockCall{name: name, args: args})
	if m.callFn != nil {
		return m.callFn(name, args)
	}
	return m.response, m.err
}

func (m *mockRunner) RunWithEnv(env []string, name string, args ...string) ([]byte, error) {
	m.calls = append(m.calls, mockCall{env: env, name: name, args: args})
	if m.callFn != nil {
		return m.callFn(name, args)
	}
	return m.response, m.err
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
) *ProviderServer {
	t.Helper()
	if ldr == nil {
		ldr = &mockLoader{
			loadFn: func(_ provider.Target, _ string) (string, error) {
				return t.TempDir(), nil
			},
		}
	}
	return New(ServerOptions{
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
	cfg := config.NewConfig(workDir)
	require.NoError(t, generate.WriteScanConfig(
		cfg.GeneratedDirPath(),
		[]string{"main"},
		map[string]string{"main": "test-req"},
		t.TempDir(),
	))
}

// --- Constructor tests ---

func TestNew_ReturnsProviderServer(t *testing.T) {
	srv := New(ServerOptions{})
	require.NotNil(t, srv)
}

func TestNew_DefaultOptions(t *testing.T) {
	srv := New(ServerOptions{})
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

	srv := New(ServerOptions{
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
	srv := New(ServerOptions{
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
	srv := New(ServerOptions{
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
	srv := New(ServerOptions{
		ToolChecker: func() ([]string, error) { return nil, nil },
	})
	resp, err := srv.Describe(
		context.Background(), &provider.DescribeRequest{},
	)
	require.NoError(t, err)
	assert.Contains(t, resp.RequiredTargetVariables, "url")
	assert.Contains(t, resp.RequiredTargetVariables, "input_path")
}

// --- Generate tests ---

func TestGenerate_EmptyConfiguration(t *testing.T) {
	srv := New(ServerOptions{})
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
	cfg := config.NewConfig(workDir)
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

	srv := New(ServerOptions{
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

	// Verify scan-config.json was written.
	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirPath())
	require.NoError(t, err)
	assert.Equal(t, []string{"kubernetes.resource_limits", "kubernetes.run_as_root"}, scanCfg.IDs)
	assert.Equal(t, "CIS-1", scanCfg.ReverseMapping["kubernetes.run_as_root"])
	assert.Equal(t, "CIS-2", scanCfg.ReverseMapping["kubernetes.resource_limits"])
}

func TestGenerate_WithoutMapping(t *testing.T) {
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"
	cfg := config.NewConfig(workDir)
	require.NoError(t, cfg.EnsureDirectories())
	policyDir := cfg.PolicyDirForBundle(bundleRef)
	require.NoError(t, os.MkdirAll(policyDir, 0750))
	// No mapping file -- should return error, not fallback.

	runner := &mockRunner{callFn: func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}}

	srv := New(ServerOptions{
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
	srv := New(ServerOptions{
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
	cfg := config.NewConfig(workDir)
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

	srv := New(ServerOptions{
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

	// Verify scan-config.json was written with the complypack dir as BundleDir.
	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirPath())
	require.NoError(t, err)
	assert.Equal(t, complypackDir, scanCfg.BundleDir,
		"scan config BundleDir should point to the complypack content path")
	assert.Contains(t, scanCfg.IDs, "kubernetes.check_one")
	assert.Equal(t, "REQ-1", scanCfg.ReverseMapping["kubernetes.check_one"])
}

func TestGenerate_ComplypackPreferredOverBundleRef(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewConfig(workDir)
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

	srv := New(ServerOptions{
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
	cfg := config.NewConfig(workDir)
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
	// ~/.complytime/complypacks/opa/1.0.0/content.tar.gz
	complypackCacheDir := filepath.Join(t.TempDir(), "opa", "1.0.0")
	require.NoError(t, os.MkdirAll(complypackCacheDir, 0750))
	archivePath := filepath.Join(complypackCacheDir, "content.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, archiveBytes, 0600))

	srv := New(ServerOptions{
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
	// (not the tar.gz path) as BundleDir.
	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirPath())
	require.NoError(t, err)
	assert.NotEqual(t, archivePath, scanCfg.BundleDir,
		"BundleDir should be the extracted directory, not the archive")
	assert.DirExists(t, scanCfg.BundleDir,
		"BundleDir should be an existing directory")
	assert.Contains(t, scanCfg.IDs, "kubernetes.check_one")
	assert.Equal(t, "REQ-1",
		scanCfg.ReverseMapping["kubernetes.check_one"])

	// Verify the mapping file exists in the extracted directory.
	mappingPath := filepath.Join(
		scanCfg.BundleDir, generate.MappingFileName,
	)
	assert.FileExists(t, mappingPath)
}

func TestGenerate_WithComplypackContentPath_TarGz_Idempotent(t *testing.T) {
	workDir := t.TempDir()
	cfg := config.NewConfig(workDir)
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

	srv := New(ServerOptions{
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
	srv := New(ServerOptions{
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

	srv := New(ServerOptions{
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
	cfg := config.NewConfig(workDir)
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

	srv := New(ServerOptions{
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
	cfg := config.NewConfig(workDir)
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

	srv := New(ServerOptions{
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

	scanCfg, err := generate.ReadScanConfig(cfg.GeneratedDirPath())
	require.NoError(t, err)
	assert.Equal(t, []string{"ns1"}, scanCfg.IDs)
	assert.Equal(t, "CIS-1", scanCfg.ReverseMapping["ns1"])
}

func TestGenerate_Scan_WithMapping(t *testing.T) {
	// Integration test: Generate with mapping → Scan with namespace filtering →
	// verify Gemara IDs in response and --namespace flags used (not --all-namespaces).
	workDir := t.TempDir()
	bundleRef := "ghcr.io/org/bundle:v1"
	cfg := config.NewConfig(workDir)
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

	srv := New(ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: workDir,
	})

	// Step 1: Generate with mapping.
	genResp, err := srv.Generate(context.Background(), &provider.GenerateRequest{
		TargetVariables: map[string]string{"opa_bundle_ref": bundleRef},
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
					"input_path":     t.TempDir(),
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

	srv := New(ServerOptions{
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

	srv := New(ServerOptions{
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

func TestScan_RemoteURL_HappyPath(t *testing.T) {
	workDir := t.TempDir()
	writeScanConfig(t, workDir)

	runner := conftestRunner(conftestHappyJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := New(ServerOptions{
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

	srv := New(ServerOptions{
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

	srv := New(ServerOptions{
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
	srv := New(ServerOptions{
		ToolChecker:  func() ([]string, error) { return []string{"conftest"}, nil },
		WorkspaceDir: t.TempDir(),
	})
	target := localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev")
	req := makeScanRequest(t, []provider.Target{target})
	_, err := srv.Scan(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conftest")
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

	srv := New(ServerOptions{
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
	// The missing-ref target falls back to the scan config's BundleDir,
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

	// Create a valid workspace so EnsureDirectories succeeds, then
	// make the results directory non-writable to trigger write errors
	// from WritePerTargetResult.
	wsDir := t.TempDir()
	srv := New(ServerOptions{
		Runner:       runner,
		Loader:       ldr,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: wsDir,
	})

	// Pre-create the results directory, then make it read-only so
	// WritePerTargetResult's os.WriteFile fails.
	resultsDir := filepath.Join(wsDir, "opa", "results")
	require.NoError(t, os.MkdirAll(resultsDir, 0750))
	require.NoError(t, os.Chmod(resultsDir, 0500))
	t.Cleanup(func() { _ = os.Chmod(resultsDir, 0750) })

	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev"),
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err, "write errors should not cause gRPC-level failure")
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Errors, "write errors should appear in resp.Errors")
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

	srv1 := New(ServerOptions{
		Runner:       runner1,
		Loader:       ldr1,
		ToolChecker:  func() ([]string, error) { return nil, nil },
		WorkspaceDir: t.TempDir(),
	})
	srv2 := New(ServerOptions{
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
