// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"fmt"
	"os/exec"
	"strings"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
	RunWithEnv(env []string, name string, args ...string) ([]byte, error)
}

// ExecRunner executes commands using os/exec.
type ExecRunner struct{}

// Run executes the named command with the given arguments.
func (r ExecRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// RunWithEnv executes the named command with a custom environment.
func (r ExecRunner) RunWithEnv(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

// PullBundle downloads an OPA policy bundle from an OCI registry using conftest.
func PullBundle(bundleRef, policyDir string, runner CommandRunner) error {
	name, args := constructConftestPullCommand(bundleRef, policyDir)
	output, err := runner.Run(name, args...)
	if err != nil {
		return fmt.Errorf("pulling policy bundle %q: %w (output: %s)", bundleRef, err, string(output))
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
// It returns the command output, including on failure, and a contextual error.
func EvalPolicyWithNamespacesAndIgnore(
	inputPath, policyDir string,
	namespaces []string,
	runner CommandRunner,
	ignore string,
	environment []string,
) ([]byte, error) {
	name, args := constructConftestTestCommandWithNamespaces(
		inputPath, policyDir, namespaces,
	)
	if ignore != "" {
		args = append(args, "--ignore", ignore)
	}
	var output []byte
	var err error
	if environment == nil {
		output, err = runner.Run(name, args...)
	} else {
		output, err = runner.RunWithEnv(environment, name, args...)
	}
	if err != nil {
		return output, fmt.Errorf(
			"evaluating policy on %q: %w (output: %s)",
			inputPath, err, string(output),
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
