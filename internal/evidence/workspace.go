// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/complytime/complyctl/pkg/provider"
)

// WorkspaceVariable is the existing host-supplied workspace variable. Providers
// deliberately do not read COMPLYTIME_WORKSPACE or reinterpret CLI precedence.
const WorkspaceVariable = "workspace"

// ResolveWorkspace resolves a supplied workspace once, including root symlinks.
// Empty values use CWD for legacy callers. Invalid non-empty paths never fall back.
// This function performs no writes and does not change process CWD.
func ResolveWorkspace(value string) (string, error) {
	if value == "" {
		value = "."
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolving workspace: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolving workspace: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("checking workspace: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace must be a directory")
	}
	return resolved, nil
}

// GenerateWorkspace resolves the authoritative global workspace, rejecting a
// conflicting non-empty target value before any generated artifact is written.
func GenerateWorkspace(request *provider.GenerateRequest) (string, error) {
	root, err := ResolveWorkspace(request.GlobalVariables[WorkspaceVariable])
	if err != nil {
		return "", err
	}
	if value := request.TargetVariables[WorkspaceVariable]; value != "" {
		targetRoot, err := ResolveWorkspace(value)
		if err != nil {
			return "", err
		}
		if targetRoot != root {
			return "", fmt.Errorf("target workspace conflicts with global workspace")
		}
	}
	return root, nil
}

// ScanWorkspace validates one root across every target before storage access.
// Missing workspace values use CWD independently, not a remembered Generate root.
func ScanWorkspace(request *provider.ScanRequest) (string, error) {
	var root string
	for _, target := range request.Targets {
		resolved, err := ResolveWorkspace(target.Variables[WorkspaceVariable])
		if err != nil {
			return "", fmt.Errorf("target %q: %w", target.TargetID, err)
		}
		if root != "" && root != resolved {
			return "", fmt.Errorf("scan targets have conflicting workspaces")
		}
		root = resolved
	}
	if root == "" {
		return ResolveWorkspace("")
	}
	return root, nil
}

// WorkspacePath resolves relative inputs/configuration against the selected root;
// absolute external paths are preserved. Empty optional paths remain empty.
func WorkspacePath(root, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
