// SPDX-License-Identifier: Apache-2.0

package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
)

// DataLoader loads input data for a scan target into the filesystem.
type DataLoader interface {
	Load(target provider.Target, workDir string) (string, error)
}

// LocalPathLoader loads data from a local filesystem path.
type LocalPathLoader struct{}

// Load validates and returns the local input path from target variables.
func (l LocalPathLoader) Load(target provider.Target, _ string) (string, error) {
	inputPath := target.Variables[VarInputPath]
	if inputPath == "" {
		return "", fmt.Errorf("input_path variable is required but empty")
	}

	if strings.Contains(inputPath, "..") {
		return "", fmt.Errorf("input path %q contains directory traversal", inputPath)
	}

	if _, err := os.Stat(inputPath); err != nil {
		return "", fmt.Errorf("input path %q: %w", inputPath, err)
	}

	return inputPath, nil
}

// GitLoader clones a git repository and returns the path to scan.
type GitLoader struct {
	Runner scan.CommandRunner
}

// Load clones the repository specified in target variables and returns the
// path to the cloned directory (or subdirectory if scan_path is set).
func (g GitLoader) Load(target provider.Target, workDir string) (string, error) {
	repoURL := target.Variables[VarURL]
	branch := target.Variables[VarBranch]
	accessToken := target.Variables[VarAccessToken]
	scanPath := target.Variables[VarScanPath]

	if branch == "" {
		branch = "main"
	}

	cloneDir := filepath.Join(workDir, sanitizeURL(repoURL), branch)
	args := []string{"clone", "--branch", branch, "--depth", "1", repoURL, cloneDir}

	var err error
	if accessToken != "" {
		username := "x-access-token"
		if strings.Contains(strings.ToLower(repoURL), "gitlab") {
			username = "oauth2"
		}
		env := buildCredentialHelperEnv(username, accessToken)
		_, err = g.Runner.RunWithEnv(env, "git", args...)
	} else {
		_, err = g.Runner.Run("git", args...)
	}

	if err != nil {
		return "", fmt.Errorf("cloning repository %q: %w", repoURL, err)
	}

	if scanPath != "" {
		return filepath.Join(cloneDir, scanPath), nil
	}
	return cloneDir, nil
}

// Router dispatches to GitLoader or LocalPathLoader based on target variables.
type Router struct {
	git   GitLoader
	local LocalPathLoader
}

// NewRouter creates a Router that dispatches to the appropriate loader.
func NewRouter(runner scan.CommandRunner) *Router {
	return &Router{
		git:   GitLoader{Runner: runner},
		local: LocalPathLoader{},
	}
}

// Load delegates to GitLoader if url is set, LocalPathLoader if input_path is
// set, or returns an error if neither is specified.
func (r *Router) Load(target provider.Target, workDir string) (string, error) {
	if target.Variables[VarURL] != "" {
		return r.git.Load(target, workDir)
	}
	if target.Variables[VarInputPath] != "" {
		return r.local.Load(target, workDir)
	}
	return "", fmt.Errorf("target must specify url or input_path")
}

func buildCredentialHelperEnv(username, token string) []string {
	helper := fmt.Sprintf(
		`!f() { test "$1" = get && echo "username=%s" && echo "password=%s"; }; f`,
		username, token,
	)
	return []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=" + helper,
		"GIT_TERMINAL_PROMPT=0",
	}
}

func sanitizeURL(repoURL string) string {
	name := repoURL
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
			break
		}
	}
	var result []rune
	for _, r := range name {
		if r == '/' || r == '.' || r == ':' {
			result = append(result, '-')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
