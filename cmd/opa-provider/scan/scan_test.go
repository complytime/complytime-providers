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

func TestConstructConftestPullCommand_WithOCIPrefix(t *testing.T) {
	name, args := constructConftestPullCommand(
		"oci://ghcr.io/org/bundle:dev", "/tmp/policy",
	)
	assert.Equal(t, "conftest", name)
	assert.Contains(t, args, "oci://ghcr.io/org/bundle:dev")

	// Verify no double oci:// prefix.
	for _, arg := range args {
		assert.NotContains(t, arg, "oci://oci://",
			"should not double-prepend oci:// prefix")
	}
}

func TestConstructConftestTestCommandWithNamespaces(t *testing.T) {
	name, args := constructConftestTestCommandWithNamespaces(
		"/tmp/input", "/tmp/policy",
		[]string{"kubernetes.run_as_root", "kubernetes.resource_limits"},
	)
	assert.Equal(t, "conftest", name)
	assert.Contains(t, args, "test")
	assert.Contains(t, args, "/tmp/input")
	assert.Contains(t, args, "--policy")
	assert.Contains(t, args, "/tmp/policy")
	assert.Contains(t, args, "--output")
	assert.Contains(t, args, "json")
	assert.Contains(t, args, "--no-fail")
	assert.NotContains(t, args, "--all-namespaces")

	// Verify namespace flags appear in pairs.
	nsCount := 0
	for i, arg := range args {
		if arg == "--namespace" {
			nsCount++
			require.Less(t, i+1, len(args), "namespace flag must have a value")
		}
	}
	assert.Equal(t, 2, nsCount)
}

func TestEvalPolicyWithNamespacesAndIgnore_Success(t *testing.T) {
	expectedJSON := []byte(
		`[{"filename":"test.yaml","namespace":"kubernetes.run_as_root","successes":1}]`,
	)
	runner := &mockRunner{response: expectedJSON}
	out, err := EvalPolicyWithNamespacesAndIgnore(
		"/tmp/input", "/tmp/policy",
		[]string{"kubernetes.run_as_root"}, runner, "", nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, out)
	require.Len(t, runner.calls, 1)
	assert.Contains(t, runner.calls[0].args, "--namespace")
	assert.NotContains(t, runner.calls[0].args, "--all-namespaces")
}

func TestEvalPolicyWithNamespacesAndIgnore_Failure(t *testing.T) {
	runner := &mockRunner{
		response: []byte("error"),
		err:      fmt.Errorf("exit status 2"),
	}
	output, err := EvalPolicyWithNamespacesAndIgnore(
		"/tmp/input", "/tmp/policy", []string{"ns1"}, runner, "", nil,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evaluating policy")
	assert.Equal(t, []byte("error"), output)
}

func TestEvalPolicyWithNamespacesAndIgnore_UsesSnapshot(t *testing.T) {
	runner := &mockRunner{response: []byte(`[]`)}
	environment := []string{"CONFTEST_IGNORE=existing", "OTHER=value"}
	output, err := EvalPolicyWithNamespacesAndIgnore(
		"input", "policy", []string{"main"}, runner, "combined", environment,
	)
	require.NoError(t, err)
	assert.Equal(t, []byte(`[]`), output)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, environment, runner.calls[0].env)
	assert.Contains(t, runner.calls[0].args, "--ignore")
	assert.Contains(t, runner.calls[0].args, "combined")
}
