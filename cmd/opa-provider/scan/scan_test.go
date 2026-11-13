// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	calls    []mockCall
	response []byte
	stderr   []byte
	err      error
}

type mockCall struct {
	env  []string
	name string
	args []string
}

func (m *mockRunner) Run(name string, args ...string) (CommandOutput, error) {
	m.calls = append(m.calls, mockCall{name: name, args: args})
	return CommandOutput{Stdout: m.response, Stderr: m.stderr}, m.err
}

func (m *mockRunner) RunWithEnv(env []string, name string, args ...string) (CommandOutput, error) {
	m.calls = append(m.calls, mockCall{env: env, name: name, args: args})
	return CommandOutput{Stdout: m.response, Stderr: m.stderr}, m.err
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
	runner := &mockRunner{response: expectedJSON, stderr: []byte("diagnostic warning\n")}
	out, err := EvalPolicyWithNamespacesAndIgnore(
		"/tmp/input", "/tmp/policy",
		[]string{"kubernetes.run_as_root"}, runner, "", nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedJSON, out.Stdout)
	assert.Equal(t, runner.stderr, out.Stderr)
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
	assert.Equal(t, []byte("error"), output.Stdout)
}

func TestEvalPolicyWithNamespacesAndIgnore_UsesSnapshot(t *testing.T) {
	runner := &mockRunner{response: []byte(`[]`), stderr: []byte("diagnostic warning\n")}
	environment := []string{"CONFTEST_IGNORE=existing", "OTHER=value"}
	output, err := EvalPolicyWithNamespacesAndIgnore(
		"input", "policy", []string{"main"}, runner, "combined", environment,
	)
	require.NoError(t, err)
	assert.Equal(t, []byte(`[]`), output.Stdout)
	assert.Equal(t, runner.stderr, output.Stderr)
	require.Len(t, runner.calls, 1)
	assert.Equal(t, environment, runner.calls[0].env)
	assert.Contains(t, runner.calls[0].args, "--ignore")
	assert.Contains(t, runner.calls[0].args, "combined")
}

const runnerStdout = " [ {\"filename\":\"input.json\",\"namespace\":\"main\",\"successes\":0} ]\n"

func TestExecRunner_OutputHelper(t *testing.T) {
	if os.Getenv("OPA_TEST_RUNNER_HELPER") != "1" {
		return
	}
	if _, err := fmt.Fprint(os.Stdout, runnerStdout); err != nil {
		os.Exit(2)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		os.Exit(2)
	}
	if _, err := fmt.Fprintf(os.Stderr, "%s\n%s\n", workingDirectory, os.Getenv("CONFTEST_IGNORE")); err != nil {
		os.Exit(2)
	}
	if os.Getenv("OPA_TEST_RUNNER_FAILURE") == "1" {
		os.Exit(7)
	}
	os.Exit(0)
}

func TestExecRunner_SeparateStreamsEnvironmentAndWorkingDirectory(t *testing.T) {
	binary, err := os.Executable()
	require.NoError(t, err)
	for _, failure := range []string{"0", "1"} {
		for _, snapshot := range []bool{false, true} {
			t.Run(fmt.Sprintf("failure=%s/snapshot=%t", failure, snapshot), func(t *testing.T) {
				t.Setenv("OPA_TEST_RUNNER_HELPER", "1")
				t.Setenv("OPA_TEST_RUNNER_FAILURE", failure)
				t.Setenv("CONFTEST_IGNORE", "captured-rule")
				environment := os.Environ()
				runner := ExecRunner{WorkingDirectory: t.TempDir()}
				var output CommandOutput
				var err error
				if snapshot {
					t.Setenv("CONFTEST_IGNORE", "changed-after-capture")
					output, err = runner.RunWithEnv(environment, binary, "-test.run=^TestExecRunner_OutputHelper$")
				} else {
					output, err = runner.Run(binary, "-test.run=^TestExecRunner_OutputHelper$")
				}
				if failure == "1" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				assert.Equal(t, runnerStdout, string(output.Stdout))
				assert.Equal(t, runner.WorkingDirectory+"\ncaptured-rule\n", string(output.Stderr))
			})
		}
	}
}
