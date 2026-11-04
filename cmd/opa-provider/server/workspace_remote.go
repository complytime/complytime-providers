// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
)

const remoteCloneRootRemedy = "set scan_path to a subdirectory of the cloned repository " +
	"that matches the cloned input path"

func (s *ProviderServer) inspectRemoteInput(
	ctx context.Context, inputPath string, variables map[string]string, configuredDir string,
) error {
	if err := preflightCancellation(ctx); err != nil {
		return err
	}
	clonePath, err := remoteCloneRoot(inputPath, variables[loader.VarScanPath])
	if err != nil {
		return err
	}
	cloneRoot, err := resolveAbsolutePath(clonePath)
	if err != nil {
		return fmt.Errorf("cannot resolve remote clone %q; check clone and scan_path: %w", clonePath, err)
	}
	boundaries, err := workspaceBoundaries(variables, configuredDir)
	if err != nil {
		return fmt.Errorf("cannot establish workspace boundaries; check workspace paths: %w", err)
	}
	identities, err := workspaceIdentities(ctx, boundaries, cloneRoot, s.localTraversal, s.identifyFile)
	if err != nil {
		return err
	}
	return inspectInput(ctx, inputPath, boundaries, cloneRoot, "", s.localTraversal, identities, s.identifyFile)
}

// The loader returns cloneRoot/scanPath; recovering the lexical root before
// resolving links prevents a selected scan-path alias from redefining the clone.
func remoteCloneRoot(inputPath, scanPath string) (string, error) {
	root := filepath.Clean(inputPath)
	if scanPath == "" {
		return root, nil
	}
	// Match the loader's filepath.Join semantics, including existing absolute
	// spellings, rather than interpreting scan_path independently.
	const anchor = "clone-root"
	relative, err := filepath.Rel(anchor, filepath.Join(anchor, scanPath))
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("cannot establish remote clone root for scan_path %q; %s",
			scanPath, remoteCloneRootRemedy)
	}
	if relative == "." {
		return root, nil
	}
	components := strings.Split(relative, string(filepath.Separator))
	for index := len(components) - 1; index >= 0; index-- {
		if filepath.Base(root) != components[index] {
			return "", fmt.Errorf("cannot establish remote clone root for input %q and scan_path %q; %s",
				inputPath, scanPath, remoteCloneRootRemedy)
		}
		root = filepath.Dir(root)
	}
	if filepath.Join(root, scanPath) != filepath.Clean(inputPath) {
		return "", fmt.Errorf("cannot establish remote clone root for input %q and scan_path %q; %s",
			inputPath, scanPath, remoteCloneRootRemedy)
	}
	return root, nil
}
