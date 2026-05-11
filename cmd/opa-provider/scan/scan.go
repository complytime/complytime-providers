// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"fmt"
	"os"
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

// CloneRepository clones a git repository at a specific branch with shallow depth.
// If accessToken is non-empty, it is injected via environment variable.
func CloneRepository(url, branch, cloneDir, accessToken string, runner CommandRunner) error {
	name, args := constructGitCloneCommand(url, branch, cloneDir)

	var output []byte
	var err error
	if accessToken != "" {
		env := buildTokenEnv(accessToken, detectPlatform(url))
		output, err = runner.RunWithEnv(env, name, args...)
	} else {
		output, err = runner.Run(name, args...)
	}

	if err != nil {
		return fmt.Errorf("cloning repository %q branch %q: %w (output: %s)", url, branch, err, string(output))
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

func constructGitCloneCommand(url, branch, cloneDir string) (string, []string) {
	return "git", []string{
		"clone",
		"--branch", branch,
		"--depth", "1",
		url, cloneDir,
	}
}

// buildTokenEnv creates an environment with the platform-specific token variable
// and GIT_TERMINAL_PROMPT=0 to prevent interactive auth prompts.
func buildTokenEnv(accessToken, platform string) []string {
	tokenVar := "GITHUB_TOKEN" //nolint:gosec // env var name, not a credential
	if platform == "gitlab" {
		tokenVar = "GITLAB_TOKEN"
	}

	env := os.Environ()
	filtered := make([]string, 0, len(env)+2)
	tokenPrefix := tokenVar + "="
	for _, e := range env {
		if !strings.HasPrefix(e, tokenPrefix) && !strings.HasPrefix(e, "GIT_TERMINAL_PROMPT=") {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, "GIT_TERMINAL_PROMPT=0")
	if accessToken != "" {
		filtered = append(filtered, tokenVar+"="+accessToken)
	}
	return filtered
}

func detectPlatform(repoURL string) string {
	lower := strings.ToLower(repoURL)
	if strings.Contains(lower, "gitlab") {
		return "gitlab"
	}
	return "github"
}
