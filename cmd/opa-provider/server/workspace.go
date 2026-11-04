// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/complytime/complyctl/pkg/provider"

	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

// workspaceBoundaries resolves both the host workspace and the provider's
// configured workspace: either may contain artifacts when their bases differ.
func workspaceBoundaries(variables map[string]string, configuredDir string) ([]string, error) {
	paths := []string{configuredDir}
	if base := variables[loader.VarWorkspace]; base != "" {
		paths = append([]string{filepath.Join(base, provider.WorkspaceDir)}, paths...)
	}

	var boundaries []string
	for _, path := range paths {
		resolved, err := resolveAbsolutePath(path)
		if err != nil {
			return nil, fmt.Errorf("resolving workspace %q: %w", path, err)
		}
		if len(boundaries) == 0 || boundaries[0] != resolved {
			boundaries = append(boundaries, resolved)
		}
	}
	return boundaries, nil
}

func resolveAbsolutePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("making path absolute: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	return resolved, nil
}

// pathWithin compares path components rather than textual prefixes, so a
// sibling such as .complytime-backup is not part of .complytime.
func pathWithin(root, path string) (bool, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false, fmt.Errorf("comparing path to workspace: %w", err)
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func validateLocalWorkspaceTarget(inputPath string, variables map[string]string, workspaceDir string) error {
	boundaries, err := workspaceBoundaries(variables, workspaceDir)
	if err != nil {
		return fmt.Errorf("cannot establish workspace boundaries; check workspace paths: %w", err)
	}
	resolvedInput, err := resolveAbsolutePath(inputPath)
	if err != nil {
		return fmt.Errorf("resolving local input %q; check input path and filesystem access: %w", inputPath, err)
	}
	for _, boundary := range boundaries {
		inside, err := pathWithin(boundary, resolvedInput)
		if err != nil {
			return fmt.Errorf("checking local input %q; check input and workspace paths: %w", inputPath, err)
		}
		if inside {
			return fmt.Errorf(
				"local input %q is inside workspace %q; copy intended input outside the workspace",
				inputPath, boundary,
			)
		}
	}
	return nil
}
