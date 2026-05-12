// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
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
		ToolChecker:  func() []string { return nil },
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
	checker := func() []string { return []string{"missing"} }
	dir := t.TempDir()

	srv := New(ServerOptions{
		Loader:       ldr,
		Runner:       runner,
		ToolChecker:  checker,
		WorkspaceDir: dir,
	})
	assert.Equal(t, dir, srv.opts.WorkspaceDir)
	assert.Equal(t, []string{"missing"}, srv.opts.ToolChecker())
}

// --- Describe tests ---

func TestDescribe_Healthy(t *testing.T) {
	srv := New(ServerOptions{
		ToolChecker: func() []string { return nil },
	})
	resp, err := srv.Describe(
		context.Background(), &provider.DescribeRequest{},
	)
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "0.1.0", resp.Version)
}

func TestDescribe_Unhealthy(t *testing.T) {
	srv := New(ServerOptions{
		ToolChecker: func() []string { return []string{"conftest"} },
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
		ToolChecker: func() []string { return nil },
	})
	resp, err := srv.Describe(
		context.Background(), &provider.DescribeRequest{},
	)
	require.NoError(t, err)
	assert.Contains(t, resp.RequiredTargetVariables, "url")
	assert.Contains(t, resp.RequiredTargetVariables, "input_path")
}

// --- Generate tests ---

func TestGenerate_ReturnsSuccess(t *testing.T) {
	srv := New(ServerOptions{})
	resp, err := srv.Generate(
		context.Background(), &provider.GenerateRequest{},
	)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

// --- Scan: happy path tests ---

func TestScan_LocalPath_HappyPath(t *testing.T) {
	dir := t.TempDir()
	runner := conftestRunner(conftestHappyJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return dir, nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, dir, "ghcr.io/org/bundle:dev"),
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Assessments)
}

func TestScan_RemoteURL_HappyPath(t *testing.T) {
	runner := conftestRunner(conftestHappyJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
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
	assert.NotEmpty(t, resp.Assessments)
}

func TestScan_RemoteURL_WithBranches(t *testing.T) {
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

	srv := newTestServer(t, runner, ldr)
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

	srv := newTestServer(t, runner, ldr)
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

func TestScan_ToolCheckFailure(t *testing.T) {
	srv := New(ServerOptions{
		ToolChecker:  func() []string { return []string{"conftest"} },
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
	require.NotEmpty(t, resp.Assessments)
	assert.Equal(t, "scan-status", resp.Assessments[0].RequirementID)
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
	require.NotEmpty(t, resp.Assessments)
	assert.Equal(t, "scan-status", resp.Assessments[0].RequirementID)
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
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
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
	assert.NotEmpty(t, resp.Assessments)
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

// --- Scan status tests ---

func TestScan_ScanStatusPrepended(t *testing.T) {
	runner := conftestRunner(conftestAllSuccessJSON)
	ldr := &mockLoader{
		loadFn: func(_ provider.Target, _ string) (string, error) {
			return t.TempDir(), nil
		},
	}

	srv := newTestServer(t, runner, ldr)
	req := makeScanRequest(t, []provider.Target{
		localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev"),
	})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Assessments)
	assert.Equal(t, "scan-status", resp.Assessments[0].RequirementID)
	assert.Contains(
		t, resp.Assessments[0].Message, "scanned successfully",
	)
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
		ToolChecker:  func() []string { return nil },
		WorkspaceDir: t.TempDir(),
	})
	srv2 := New(ServerOptions{
		Runner:       runner2,
		Loader:       ldr2,
		ToolChecker:  func() []string { return nil },
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
