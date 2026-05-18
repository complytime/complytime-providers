// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"fmt"
	"os/exec"
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

// EvalPolicy evaluates configuration files against OPA policies using conftest.
func EvalPolicy(inputPath, policyDir string, runner CommandRunner) ([]byte, error) {
	name, args := constructConftestTestCommand(inputPath, policyDir)
	output, err := runner.Run(name, args...)
	if err != nil {
		return nil, fmt.Errorf("evaluating policy on %q: %w (output: %s)", inputPath, err, string(output))
	}
	return output, nil
}

func constructConftestPullCommand(bundleRef, policyDir string) (string, []string) {
	return "conftest", []string{
		"pull",
		"oci://" + bundleRef,
		"--policy", policyDir,
	}
}

func constructConftestTestCommand(inputPath, policyDir string) (string, []string) {
	return "conftest", []string{
		"test", inputPath,
		"--policy", policyDir,
		"--output", "json",
		"--all-namespaces",
		"--no-fail",
	}
}
