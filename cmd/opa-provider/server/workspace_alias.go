// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// localTraversal keeps filesystem failures injectable without mutable globals.
type localTraversal struct {
	readDir func(string) ([]os.DirEntry, error)
	lstat   func(string) (os.FileInfo, error)
}

func inspectInput(
	ctx context.Context, inputPath string, boundaries []string, cloneRoot, ignore string, traversal localTraversal,
	identities map[fileIdentity]string, identify identityFunc,
) error {
	var ignored *regexp.Regexp
	if ignore != "" {
		var err error
		ignored, err = regexp.Compile(ignore)
		if err != nil {
			return fmt.Errorf("checking local input ignore expression; correct CONFTEST_IGNORE: %w", err)
		}
	}
	inspector := localInputInspector{
		ctx:        ctx,
		boundaries: boundaries, ignored: ignored, traversal: traversal,
		cloneRoot: cloneRoot, identities: identities, identify: identify,
		activeDirs: make(map[string]bool), visitedDirs: make(map[string]bool),
	}
	if err := inspector.inspect(inputPath); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("cannot safely inspect input %q; fix inaccessible or unsafe paths, "+
			"or select input outside the workspace: %w", inputPath, err)
	}
	return preflightCancellation(ctx)
}

type localInputInspector struct {
	ctx         context.Context
	boundaries  []string
	cloneRoot   string
	identities  map[fileIdentity]string
	identify    identityFunc
	ignored     *regexp.Regexp
	traversal   localTraversal
	activeDirs  map[string]bool
	visitedDirs map[string]bool
}

func (inspector *localInputInspector) inspect(path string) error {
	if err := preflightCancellation(inspector.ctx); err != nil {
		return err
	}
	info, err := inspector.traversal.lstat(path)
	if err != nil {
		return fmt.Errorf("accessing %q: %w", path, err)
	}
	if err := preflightCancellation(inspector.ctx); err != nil {
		return err
	}
	if info.IsDir() {
		return inspector.inspectDirectory(path)
	}
	// Conftest checks ignore before resolving non-directory entries, but does
	// not prune directories. Reject broken links even when ignored so an
	// uncertain filesystem state cannot produce a passing assessment.
	if inspector.ignored != nil && inspector.ignored.MatchString(path) {
		if info.Mode()&fs.ModeSymlink != 0 {
			if _, err := filepath.EvalSymlinks(path); err != nil {
				return fmt.Errorf("resolving ignored symlink %q: %w", path, err)
			}
		}
		return preflightCancellation(inspector.ctx)
	}
	resolved, err := resolveAbsolutePath(path)
	if err != nil {
		return fmt.Errorf("resolving input path %q: %w", path, err)
	}
	if err := inspector.checkWorkspacePath(path, resolved); err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		// Conftest recursively walks the symlink target using its resolved spelling.
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolving symlink %q: %w", path, err)
		}
		targetInfo, err := inspector.traversal.lstat(target)
		if err != nil {
			return fmt.Errorf("accessing symlink target %q: %w", target, err)
		}
		if targetInfo.IsDir() {
			return inspector.inspectDirectory(target)
		}
		info = targetInfo
	}
	if info.Mode().IsRegular() && inspector.identities != nil {
		identity, err := inspector.identify(path, info)
		if err != nil {
			return fmt.Errorf("identifying input file %q: %w", path, err)
		}
		if err := preflightCancellation(inspector.ctx); err != nil {
			return err
		}
		if workspacePath, found := inspector.identities[identity]; found {
			return fmt.Errorf("input file %q shares filesystem identity with workspace file %q; "+
				"copy intended input outside the workspace", path, workspacePath)
		}
	}
	return preflightCancellation(inspector.ctx)
}

func (inspector *localInputInspector) checkWorkspacePath(path, resolved string) error {
	if inspector.cloneRoot != "" {
		insideClone, err := pathWithin(inspector.cloneRoot, resolved)
		if err != nil {
			return fmt.Errorf("checking clone path %q: %w", path, err)
		}
		if insideClone {
			return nil
		}
	}
	for _, boundary := range inspector.boundaries {
		inside, err := pathWithin(boundary, resolved)
		if err != nil {
			return fmt.Errorf("checking input path %q: %w", path, err)
		}
		if inside {
			return fmt.Errorf("input path %q resolves to workspace %q; copy intended input outside the workspace",
				path, boundary)
		}
	}
	return nil
}

func (inspector *localInputInspector) inspectDirectory(path string) error {
	resolved, err := resolveAbsolutePath(path)
	if err != nil {
		return fmt.Errorf("resolving directory %q: %w", path, err)
	}
	if inspector.cloneRoot != "" {
		if err := inspector.checkWorkspacePath(path, resolved); err != nil {
			return err
		}
	}
	if inspector.activeDirs[resolved] {
		return fmt.Errorf("directory symlink cycle at %q; remove the cycle", path)
	}
	if inspector.visitedDirs[path] {
		return nil
	}
	inspector.visitedDirs[path] = true
	inspector.activeDirs[resolved] = true
	defer delete(inspector.activeDirs, resolved)

	entries, err := inspector.traversal.readDir(path)
	if err != nil {
		return fmt.Errorf("reading directory %q: %w", path, err)
	}
	if err := preflightCancellation(inspector.ctx); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := inspector.inspect(filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
