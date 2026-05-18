// SPDX-License-Identifier: Apache-2.0

package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
)

type mockRunner struct {
	calls    []mockCall
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
	return m.response, m.err
}

func (m *mockRunner) RunWithEnv(env []string, name string, args ...string) ([]byte, error) {
	m.calls = append(m.calls, mockCall{env: env, name: name, args: args})
	return m.response, m.err
}

var _ scan.CommandRunner = (*mockRunner)(nil)

func TestLocalPathLoader_Load_ValidDir(t *testing.T) {
	dir := t.TempDir()
	target := provider.Target{
		Variables: map[string]string{"input_path": dir},
	}

	loader := LocalPathLoader{}
	path, err := loader.Load(target, "")
	require.NoError(t, err)
	assert.Equal(t, dir, path)
}

func TestLocalPathLoader_Load_ValidFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte("test"), 0600))

	target := provider.Target{
		Variables: map[string]string{"input_path": file},
	}

	loader := LocalPathLoader{}
	path, err := loader.Load(target, "")
	require.NoError(t, err)
	assert.Equal(t, file, path)
}

func TestLocalPathLoader_Load_Traversal(t *testing.T) {
	target := provider.Target{
		Variables: map[string]string{"input_path": "../../etc/passwd"},
	}

	loader := LocalPathLoader{}
	_, err := loader.Load(target, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "traversal")
}

func TestLocalPathLoader_Load_NotExist(t *testing.T) {
	target := provider.Target{
		Variables: map[string]string{"input_path": "/nonexistent/path/xyz"},
	}

	loader := LocalPathLoader{}
	_, err := loader.Load(target, "")
	require.Error(t, err)
}

func TestLocalPathLoader_Load_EmptyPath(t *testing.T) {
	target := provider.Target{
		Variables: map[string]string{},
	}

	loader := LocalPathLoader{}
	_, err := loader.Load(target, "")
	require.Error(t, err)
}

func TestGitLoader_Load_PublicRepo(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	loader := GitLoader{Runner: runner}
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url":    "https://github.com/org/repo",
			"branch": "main",
		},
	}

	path, err := loader.Load(target, workDir)
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	require.Len(t, runner.calls, 1)
	call := runner.calls[0]
	assert.Equal(t, "git", call.name)
	assert.Contains(t, call.args, "clone")
	assert.Contains(t, call.args, "--branch")
	assert.Contains(t, call.args, "main")
	assert.Contains(t, call.args, "--depth")
	assert.Contains(t, call.args, "1")
	assert.Empty(t, call.env, "public clone should not use env")
}

func TestGitLoader_Load_WithToken_GitHub(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	loader := GitLoader{Runner: runner}
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url":          "https://github.com/org/repo",
			"branch":       "main",
			"access_token": "ghp_secrettoken123",
		},
	}

	_, err := loader.Load(target, workDir)
	require.NoError(t, err)

	require.Len(t, runner.calls, 1)
	call := runner.calls[0]
	assert.NotEmpty(t, call.env, "authenticated clone should use RunWithEnv")

	hasConfigCount := false
	hasCredHelper := false
	hasUsername := false
	for _, e := range call.env {
		if e == "GIT_CONFIG_COUNT=1" {
			hasConfigCount = true
		}
		if e == "GIT_CONFIG_KEY_0=credential.helper" {
			hasCredHelper = true
		}
		if assert.Condition(t, func() bool {
			return len(e) > 0
		}) && len(e) > len("GIT_CONFIG_VALUE_0=") {
			if e[:len("GIT_CONFIG_VALUE_0=")] == "GIT_CONFIG_VALUE_0=" {
				assert.Contains(t, e, "x-access-token")
				hasUsername = true
			}
		}
	}
	assert.True(t, hasConfigCount, "should have GIT_CONFIG_COUNT=1")
	assert.True(t, hasCredHelper, "should have GIT_CONFIG_KEY_0=credential.helper")
	assert.True(t, hasUsername, "credential helper should use x-access-token for GitHub")
}

func TestGitLoader_Load_WithToken_GitLab(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	loader := GitLoader{Runner: runner}
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url":          "https://gitlab.com/org/repo",
			"branch":       "main",
			"access_token": "glpat_secret",
		},
	}

	_, err := loader.Load(target, workDir)
	require.NoError(t, err)

	require.Len(t, runner.calls, 1)
	call := runner.calls[0]

	for _, e := range call.env {
		if len(e) > len("GIT_CONFIG_VALUE_0=") &&
			e[:len("GIT_CONFIG_VALUE_0=")] == "GIT_CONFIG_VALUE_0=" {
			assert.Contains(t, e, "oauth2")
		}
	}
}

func TestGitLoader_Load_WithScanPath(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	loader := GitLoader{Runner: runner}
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url":       "https://github.com/org/repo",
			"branch":    "main",
			"scan_path": "configs/k8s",
		},
	}

	path, err := loader.Load(target, workDir)
	require.NoError(t, err)
	assert.Contains(t, path, filepath.Join("configs", "k8s"))
}

func TestGitLoader_Load_CloneFailure(t *testing.T) {
	runner := &mockRunner{
		response: []byte("fatal: not found"),
		err:      fmt.Errorf("exit status 128"),
	}
	loader := GitLoader{Runner: runner}
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url":    "https://github.com/org/repo",
			"branch": "main",
		},
	}

	_, err := loader.Load(target, workDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 128")
}

func TestGitLoader_Load_CredentialHelperEnvVars(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	loader := GitLoader{Runner: runner}
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url":          "https://github.com/org/repo",
			"branch":       "main",
			"access_token": "ghp_test123",
		},
	}

	_, err := loader.Load(target, workDir)
	require.NoError(t, err)

	require.Len(t, runner.calls, 1)
	call := runner.calls[0]

	envMap := make(map[string]string)
	for _, e := range call.env {
		parts := splitEnvVar(e)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	assert.Equal(t, "1", envMap["GIT_CONFIG_COUNT"])
	assert.Equal(t, "credential.helper", envMap["GIT_CONFIG_KEY_0"])
	assert.Contains(t, envMap["GIT_CONFIG_VALUE_0"], "!f()")
	assert.Contains(t, envMap["GIT_CONFIG_VALUE_0"], `test "$1" = get`)
	assert.Equal(t, "0", envMap["GIT_TERMINAL_PROMPT"])
}

func TestGitLoader_Load_DefaultBranch(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	loader := GitLoader{Runner: runner}
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url": "https://github.com/org/repo",
		},
	}

	_, err := loader.Load(target, workDir)
	require.NoError(t, err)

	require.Len(t, runner.calls, 1)
	assert.Contains(t, runner.calls[0].args, "main")
}

func TestRouter_Load_URLTarget(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	router := NewRouter(runner)
	workDir := t.TempDir()

	target := provider.Target{
		Variables: map[string]string{
			"url":    "https://github.com/org/repo",
			"branch": "main",
		},
	}

	path, err := router.Load(target, workDir)
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, "git", runner.calls[0].name)
}

func TestRouter_Load_InputPathTarget(t *testing.T) {
	dir := t.TempDir()
	runner := &mockRunner{}
	router := NewRouter(runner)

	target := provider.Target{
		Variables: map[string]string{"input_path": dir},
	}

	path, err := router.Load(target, "")
	require.NoError(t, err)
	assert.Equal(t, dir, path)
	assert.Empty(t, runner.calls, "local path should not invoke runner")
}

func TestRouter_Load_NeitherSet(t *testing.T) {
	runner := &mockRunner{}
	router := NewRouter(runner)

	target := provider.Target{
		Variables: map[string]string{},
	}

	_, err := router.Load(target, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url or input_path")
}

func splitEnvVar(s string) []string {
	idx := 0
	for i, c := range s {
		if c == '=' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}
