// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func (s *ProviderServer) localWorkspaceIgnore(
	inputPath string, variables map[string]string, configuredDir string,
) (string, []string, []string, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("checking local input %q: %w", inputPath, err)
	}
	if !info.IsDir() {
		return "", nil, nil, nil
	}
	boundaries, err := workspaceBoundaries(variables, configuredDir)
	if err != nil {
		return "", nil, nil, fmt.Errorf("resolving workspace boundaries: %w", err)
	}
	workspacePattern, overlapping, err := workspaceIgnorePattern(inputPath, boundaries)
	if err != nil || workspacePattern == "" {
		return "", nil, nil, err
	}

	// Use the exact same environment for detection and execution so another
	// goroutine cannot change conftest's precedence between the two steps.
	environment := os.Environ()
	userIgnore := environmentValue(environment, conftestIgnoreEnv)
	if userIgnore != "" {
		if _, err := regexp.Compile(userIgnore); err != nil {
			return "", nil, nil, fmt.Errorf("invalid %s expression; correct it: %w", conftestIgnoreEnv, err)
		}
		// A flag takes precedence over conftest config; grouping preserves both
		// regexes while leaving the environment intact for dotenv interpolation.
		return "(?:" + userIgnore + ")|(?:" + workspacePattern + ")", environment, overlapping, nil
	}

	configPath, err := s.detectConftestConfig(environmentValue(environment, conftestConfigFileEnv))
	if err != nil {
		return "", nil, nil, fmt.Errorf("cannot determine conftest configuration: %w; %s", err, configRemedy())
	}
	if configPath != "" {
		return "", nil, nil, fmt.Errorf("conftest configuration %q may define an ignore rule; %s",
			configPath, configRemedy())
	}
	return workspacePattern, environment, overlapping, nil
}

func environmentValue(environment []string, key string) string {
	var value string
	for _, entry := range environment {
		name, candidate, found := strings.Cut(entry, "=")
		if found && (name == key || (runtime.GOOS == "windows" && strings.EqualFold(name, key))) {
			value = candidate
		}
	}
	return value
}

func configRemedy() string {
	return "select a target outside the workspace, or set CONFTEST_IGNORE to the effective " +
		"ignore rule (including the file's ignore value), or an expression matching no paths, such as ^$, " +
		"if the file defines none"
}

func workspaceIgnorePattern(inputPath string, boundaries []string) (string, []string, error) {
	resolvedInput, err := resolveAbsolutePath(inputPath)
	if err != nil {
		return "", nil, fmt.Errorf("resolving local input %q: %w", inputPath, err)
	}
	var patterns []string
	var overlapping []string
	seen := make(map[string]bool)
	for _, boundary := range boundaries {
		inside, err := pathWithin(resolvedInput, boundary)
		if err != nil {
			return "", nil, fmt.Errorf("checking workspace overlap: %w", err)
		}
		if !inside {
			continue
		}
		overlapping = append(overlapping, boundary)
		relative, err := filepath.Rel(resolvedInput, boundary)
		if err != nil {
			return "", nil, fmt.Errorf("deriving workspace path for local input: %w", err)
		}
		// filepath.Walk uses the input spelling; symlink walks can use resolved
		// spellings instead. Both are needed without matching similarly named siblings.
		for _, path := range []string{filepath.Join(inputPath, relative), boundary} {
			if !seen[path] {
				seen[path] = true
				patterns = append(patterns, "^"+regexp.QuoteMeta(path)+"(?:"+
					regexp.QuoteMeta(string(filepath.Separator))+"|$)")
			}
		}
	}
	return strings.Join(patterns, "|"), overlapping, nil
}
