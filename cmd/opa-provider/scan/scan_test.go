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
	name, args := constructConftestPullCommand(
		"ghcr.io/org/bundle:dev", "/tmp/policy",
	)
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

func TestEvalPolicy_Success(t *testing.T) {
	expectedJSON := []byte(
		`[{"filename":"test.yaml","namespace":"main","successes":1}]`,
	)
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
