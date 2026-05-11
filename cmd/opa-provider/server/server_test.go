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

func setupTest(t *testing.T) (func(), *mockRunner) {
	t.Helper()
	origRunner := ScanRunner
	origSkip := SkipToolCheck
	origWorkspace := TestWorkspaceDir
	SkipToolCheck = true
	TestWorkspaceDir = t.TempDir()

	runner := &mockRunner{}
	ScanRunner = runner

	cleanup := func() {
		ScanRunner = origRunner
		SkipToolCheck = origSkip
		TestWorkspaceDir = origWorkspace
	}
	return cleanup, runner
}

func makeScanRequest(t *testing.T, targets []provider.Target) *provider.ScanRequest {
	t.Helper()
	return &provider.ScanRequest{Targets: targets}
}

func localTarget(t *testing.T, inputPath, bundleRef string) provider.Target {
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

func TestNew_ReturnsProviderServer(t *testing.T) {
	srv := New()
	require.NotNil(t, srv)
}

func TestDescribe_Healthy(t *testing.T) {
	origSkip := SkipToolCheck
	SkipToolCheck = true
	defer func() { SkipToolCheck = origSkip }()

	srv := New()
	resp, err := srv.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "0.1.0", resp.Version)
}

func TestDescribe_Variables(t *testing.T) {
	origSkip := SkipToolCheck
	SkipToolCheck = true
	defer func() { SkipToolCheck = origSkip }()

	srv := New()
	resp, err := srv.Describe(context.Background(), &provider.DescribeRequest{})
	require.NoError(t, err)
	assert.Contains(t, resp.RequiredTargetVariables, "url")
	assert.Contains(t, resp.RequiredTargetVariables, "input_path")
}

func TestGenerate_ReturnsSuccess(t *testing.T) {
	srv := New()
	resp, err := srv.Generate(context.Background(), &provider.GenerateRequest{})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestScan_LocalPath_HappyPath(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	dir := t.TempDir()
	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "conftest" && len(args) > 0 && args[0] == "test" {
			return []byte(conftestHappyJSON), nil
		}
		return nil, fmt.Errorf("unexpected command: %s %v", name, args)
	}

	srv := New()
	req := makeScanRequest(t, []provider.Target{localTarget(t, dir, "ghcr.io/org/bundle:dev")})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Assessments)
}

func TestScan_RemoteURL_HappyPath(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "git" {
			return []byte("Cloning..."), nil
		}
		if name == "conftest" && len(args) > 0 && args[0] == "test" {
			return []byte(conftestHappyJSON), nil
		}
		return nil, fmt.Errorf("unexpected command: %s %v", name, args)
	}

	srv := New()
	target := remoteTarget("ghcr.io/org/bundle:dev", "https://github.com/org/repo")
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Assessments)
}

func TestScan_RemoteURL_WithBranches(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	gitCloneCount := 0
	conftestTestCount := 0
	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "git" {
			gitCloneCount++
			return []byte("Cloning..."), nil
		}
		if name == "conftest" && len(args) > 0 && args[0] == "test" {
			conftestTestCount++
			return []byte(conftestHappyJSON), nil
		}
		return nil, fmt.Errorf("unexpected command: %s %v", name, args)
	}

	srv := New()
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
	assert.Equal(t, 2, gitCloneCount, "should clone for each branch")
	assert.Equal(t, 2, conftestTestCount, "should test for each branch")
}

func TestScan_RemoteURL_WithScanPath(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	var conftestTestPath string
	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "git" {
			return []byte("Cloning..."), nil
		}
		if name == "conftest" && len(args) > 0 && args[0] == "test" {
			conftestTestPath = args[1]
			return []byte(conftestAllSuccessJSON), nil
		}
		return nil, fmt.Errorf("unexpected command: %s %v", name, args)
	}

	srv := New()
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
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "git" || (name == "conftest" && len(args) > 0 && args[0] == "test") {
			return []byte(conftestAllSuccessJSON), nil
		}
		return nil, fmt.Errorf("unexpected command")
	}

	srv := New()
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

	// Find the git clone call and verify token was in env
	for _, call := range runner.calls {
		if call.name == "git" {
			require.NotEmpty(t, call.env, "git clone should use RunWithEnv")
			hasToken := false
			for _, e := range call.env {
				if e == "GITHUB_TOKEN=ghp_secrettoken123" {
					hasToken = true
				}
			}
			assert.True(t, hasToken, "GITHUB_TOKEN should be in env")
			for _, arg := range call.args {
				assert.NotContains(t, arg, "ghp_secrettoken123", "token should not be in args")
			}
		}
	}
}

func TestScan_RemoteURL_UnauthenticatedClone(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "git" || (name == "conftest" && len(args) > 0 && args[0] == "test") {
			return []byte(conftestAllSuccessJSON), nil
		}
		return nil, fmt.Errorf("unexpected command")
	}

	srv := New()
	target := remoteTarget("ghcr.io/org/bundle:dev", "https://github.com/org/repo")
	req := makeScanRequest(t, []provider.Target{target})
	_, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)

	for _, call := range runner.calls {
		if call.name == "git" {
			assert.Empty(t, call.env, "unauthenticated clone should use Run, not RunWithEnv")
		}
	}
}

func TestScan_NoTargets(t *testing.T) {
	cleanup, _ := setupTest(t)
	defer cleanup()

	srv := New()
	req := makeScanRequest(t, nil)
	_, err := srv.Scan(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one target")
}

func TestScan_MissingBundleRef(t *testing.T) {
	cleanup, _ := setupTest(t)
	defer cleanup()

	srv := New()
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"input_path": t.TempDir(),
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	_, err := srv.Scan(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "opa_bundle_ref")
}

func TestScan_BothURLAndInputPath(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}

	srv := New()
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
	// Per-target error: scan continues, error captured in results
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_NeitherURLNorInputPath(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}

	srv := New()
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
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}

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
			srv := New()
			req := makeScanRequest(t, []provider.Target{tc.target})
			resp, err := srv.Scan(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, resp)
		})
	}
}

func TestScan_ConftestPullFailure(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("auth failed"), fmt.Errorf("exit status 1")
		}
		return []byte("ok"), nil
	}

	srv := New()
	target := localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev")
	req := makeScanRequest(t, []provider.Target{target})
	_, err := srv.Scan(context.Background(), req)
	assert.Error(t, err)
}

func TestScan_ConftestTestFailure(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "conftest" && len(args) > 0 && args[0] == "test" {
			return []byte("eval error"), fmt.Errorf("exit status 2")
		}
		return nil, fmt.Errorf("unexpected command")
	}

	srv := New()
	target := localTarget(t, t.TempDir(), "ghcr.io/org/bundle:dev")
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_GitCloneFailure(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "git" {
			return []byte("fatal: not found"), fmt.Errorf("exit status 128")
		}
		return nil, fmt.Errorf("unexpected")
	}

	srv := New()
	target := remoteTarget("ghcr.io/org/bundle:dev", "https://github.com/org/repo")
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestScan_MultipleTargets_PartialFailure(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	callCount := 0
	runner.callFn = func(name string, args []string) ([]byte, error) {
		if name == "conftest" && len(args) > 0 && args[0] == "pull" {
			return []byte("ok"), nil
		}
		if name == "conftest" && len(args) > 0 && args[0] == "test" {
			callCount++
			if callCount == 1 {
				return []byte("error"), fmt.Errorf("exit status 2")
			}
			return []byte(conftestAllSuccessJSON), nil
		}
		return nil, fmt.Errorf("unexpected command")
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	srv := New()
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
			err := validateTargetVariables(tc.url, tc.inputPath, tc.branches, tc.scanPath, tc.accessToken)
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
		{"both url and input_path", "https://github.com/org/repo", "/path", "", "", "",
			"specify either url or input_path"},
		{"neither url nor input_path", "", "", "", "", "",
			"url or input_path"},
		{"traversal in scan_path", "https://github.com/org/repo", "", "main", "../etc", "",
			"traversal"},
		{"traversal in branches", "https://github.com/org/repo", "", "main/../etc", "", "",
			"traversal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTargetVariables(tc.url, tc.inputPath, tc.branches, tc.scanPath, tc.accessToken)
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

func TestScan_InvalidURL(t *testing.T) {
	cleanup, runner := setupTest(t)
	defer cleanup()

	runner.callFn = func(name string, args []string) ([]byte, error) {
		return []byte("ok"), nil
	}

	srv := New()
	target := provider.Target{
		TargetID: "test",
		Variables: map[string]string{
			"url":            "http://github.com/org/repo",
			"opa_bundle_ref": "ghcr.io/org/bundle:dev",
		},
	}
	req := makeScanRequest(t, []provider.Target{target})
	resp, err := srv.Scan(context.Background(), req)
	// Per-target validation error, scan continues
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestProviderInterface(t *testing.T) {
	var _ provider.Provider = (*ProviderServer)(nil)
}
