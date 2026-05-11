// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestConstructConftestPullCommand(t *testing.T) {
	name, args := constructConftestPullCommand("ghcr.io/org/bundle:dev", "/tmp/policy")
	assert.Equal(t, "conftest", name)
	assert.Contains(t, args, "pull")
	assert.Contains(t, args, "oci://ghcr.io/org/bundle:dev")
	assert.Contains(t, args, "--policy")
	assert.Contains(t, args, "/tmp/policy")
}

func TestConstructConftestTestCommand(t *testing.T) {
	name, args := constructConftestTestCommand("/tmp/input", "/tmp/policy")
	assert.Equal(t, "conftest", name)
	assert.Contains(t, args, "test")
	assert.Contains(t, args, "/tmp/input")
	assert.Contains(t, args, "--policy")
	assert.Contains(t, args, "/tmp/policy")
	assert.Contains(t, args, "--output")
	assert.Contains(t, args, "json")
	assert.Contains(t, args, "--all-namespaces")
	assert.Contains(t, args, "--no-fail")
}

func TestConstructGitCloneCommand(t *testing.T) {
	name, args := constructGitCloneCommand("https://github.com/org/repo", "main", "/tmp/clone")
	assert.Equal(t, "git", name)
	assert.Contains(t, args, "clone")
	assert.Contains(t, args, "--branch")
	assert.Contains(t, args, "main")
	assert.Contains(t, args, "--depth")
	assert.Contains(t, args, "1")
	assert.Contains(t, args, "https://github.com/org/repo")
	assert.Contains(t, args, "/tmp/clone")
}

func TestPullBundle_Success(t *testing.T) {
	runner := &mockRunner{response: []byte("ok")}
	err := PullBundle("ghcr.io/org/bundle:dev", "/tmp/policy", runner)
	assert.NoError(t, err)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, "conftest", runner.calls[0].name)
	assert.Contains(t, runner.calls[0].args, "pull")
}

func TestPullBundle_Failure(t *testing.T) {
	runner := &mockRunner{
		response: []byte("auth failed"),
		err:      fmt.Errorf("exit status 1"),
	}
	err := PullBundle("ghcr.io/org/bundle:dev", "/tmp/policy", runner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pulling policy bundle")
}

func TestCloneRepository_Success(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	err := CloneRepository("https://github.com/org/repo", "main", "/tmp/clone", "", runner)
	assert.NoError(t, err)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, "git", runner.calls[0].name)
	assert.Empty(t, runner.calls[0].env)
}

func TestCloneRepository_WithToken(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	err := CloneRepository("https://github.com/org/repo", "main", "/tmp/clone", "ghp_secret123", runner)
	assert.NoError(t, err)
	require.Len(t, runner.calls, 1)
	assert.NotEmpty(t, runner.calls[0].env)

	// Verify token is in env, not in args
	hasToken := false
	for _, e := range runner.calls[0].env {
		if e == "GITHUB_TOKEN=ghp_secret123" {
			hasToken = true
		}
	}
	assert.True(t, hasToken, "GITHUB_TOKEN should be in environment")

	for _, arg := range runner.calls[0].args {
		assert.NotContains(t, arg, "ghp_secret123", "token should not appear in args")
	}
}

func TestCloneRepository_WithoutToken(t *testing.T) {
	runner := &mockRunner{response: []byte("Cloning...")}
	err := CloneRepository("https://github.com/org/repo", "main", "/tmp/clone", "", runner)
	assert.NoError(t, err)
	require.Len(t, runner.calls, 1)
	assert.Empty(t, runner.calls[0].env)
}

func TestCloneRepository_Failure(t *testing.T) {
	runner := &mockRunner{
		response: []byte("fatal: repository not found"),
		err:      fmt.Errorf("exit status 128"),
	}
	err := CloneRepository("https://github.com/org/repo", "main", "/tmp/clone", "", runner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloning repository")
}

func TestEvalPolicy_Success(t *testing.T) {
	expectedJSON := []byte(`[{"filename":"test.yaml","namespace":"main","successes":1}]`)
	runner := &mockRunner{response: expectedJSON}
	out, err := EvalPolicy("/tmp/input", "/tmp/policy", runner)
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, out)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, "conftest", runner.calls[0].name)
}

func TestEvalPolicy_Failure(t *testing.T) {
	runner := &mockRunner{
		response: []byte("error"),
		err:      fmt.Errorf("exit status 2"),
	}
	_, err := EvalPolicy("/tmp/input", "/tmp/policy", runner)
	assert.Error(t, err)
}

func TestBuildTokenEnv_GitHub(t *testing.T) {
	env := buildTokenEnv("ghp_secret", "github")
	hasToken := false
	hasPrompt := false
	for _, e := range env {
		if e == "GITHUB_TOKEN=ghp_secret" {
			hasToken = true
		}
		if e == "GIT_TERMINAL_PROMPT=0" {
			hasPrompt = true
		}
	}
	assert.True(t, hasToken, "should contain GITHUB_TOKEN")
	assert.True(t, hasPrompt, "should contain GIT_TERMINAL_PROMPT=0")
}

func TestBuildTokenEnv_GitLab(t *testing.T) {
	env := buildTokenEnv("glpat_secret", "gitlab")
	hasToken := false
	for _, e := range env {
		if e == "GITLAB_TOKEN=glpat_secret" {
			hasToken = true
		}
	}
	assert.True(t, hasToken, "should contain GITLAB_TOKEN")
}

func TestBuildTokenEnv_NoToken(t *testing.T) {
	env := buildTokenEnv("", "github")
	hasPrompt := false
	for _, e := range env {
		if e == "GIT_TERMINAL_PROMPT=0" {
			hasPrompt = true
		}
		assert.NotContains(t, e, "GITHUB_TOKEN=\n")
	}
	assert.True(t, hasPrompt, "should contain GIT_TERMINAL_PROMPT=0")
}
