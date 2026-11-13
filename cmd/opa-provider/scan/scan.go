// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CommandOutput keeps machine-readable stdout separate from diagnostic stderr,
// including on execution failure. Only Stdout may become conftest JSON evidence.
type CommandOutput struct {
	Stdout []byte
	Stderr []byte
}

// CommandRunner abstracts command execution with separate output streams for testing.
type CommandRunner interface {
	Run(name string, args ...string) (CommandOutput, error)
	RunWithEnv(env []string, name string, args ...string) (CommandOutput, error)
}

// ExecRunner executes commands using os/exec.
type ExecRunner struct{ WorkingDirectory string }

// Run executes the named command with the given arguments.
func (r ExecRunner) Run(name string, args ...string) (CommandOutput, error) {
	return r.RunWithEnv(nil, name, args...)
}

// RunWithEnv executes the named command with a custom environment.
func (r ExecRunner) RunWithEnv(env []string, name string, args ...string) (CommandOutput, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Dir = r.WorkingDirectory
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return CommandOutput{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, err
}

// PullBundle downloads an OPA policy bundle from an OCI registry using conftest.
func PullBundle(bundleRef, policyDir string, runner CommandRunner) error {
	name, args := constructConftestPullCommand(bundleRef, policyDir)
	output, err := runner.Run(name, args...)
	if err != nil {
		return fmt.Errorf("pulling policy bundle %q: %w (stdout: %s; stderr: %s)",
			bundleRef, err, output.Stdout, output.Stderr)
	}
	return nil
}

func constructConftestPullCommand(bundleRef, policyDir string) (string, []string) {
	return "conftest", []string{
		"pull",
		"oci://" + strings.TrimPrefix(bundleRef, "oci://"),
		"--policy", policyDir,
	}
}

// EvalPolicyWithNamespacesAndIgnore evaluates input with an optional conftest
// ignore expression and environment snapshot. A nil environment is inherited.
// It returns both streams unchanged, including on failure, and a contextual error.
// A valid noncompliant result succeeds because the command uses --no-fail.
func EvalPolicyWithNamespacesAndIgnore(
	inputPath, policyDir string,
	namespaces []string,
	runner CommandRunner,
	ignore string,
	environment []string,
) (CommandOutput, error) {
	name, args := constructConftestTestCommandWithNamespaces(
		inputPath, policyDir, namespaces,
	)
	if ignore != "" {
		args = append(args, "--ignore", ignore)
	}
	var output CommandOutput
	var err error
	if environment == nil {
		output, err = runner.Run(name, args...)
	} else {
		output, err = runner.RunWithEnv(environment, name, args...)
	}
	if err != nil {
		return output, fmt.Errorf(
			"evaluating policy on %q: %w (stdout: %s; stderr: %s)",
			inputPath, err, output.Stdout, output.Stderr,
		)
	}
	return output, nil
}

func constructConftestTestCommandWithNamespaces(
	inputPath, policyDir string,
	namespaces []string,
) (string, []string) {
	args := []string{
		"test", inputPath,
		"--policy", policyDir,
		"--output", "json",
		"--no-fail",
	}
	for _, ns := range namespaces {
		args = append(args, "--namespace", ns)
	}
	return "conftest", args
}
