// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type identityFunc func(string, os.FileInfo) (fileIdentity, error)

// workspaceIdentities omits clone paths during collection, never by subtracting
// identities afterwards: a second workspace path may name the same inode.
func workspaceIdentities(
	ctx context.Context, boundaries []string, cloneRoot string, traversal localTraversal,
	identify identityFunc,
) (map[fileIdentity]string, error) {
	collector := workspaceIdentityCollector{
		ctx:       ctx,
		cloneRoot: cloneRoot, boundaries: boundaries, traversal: traversal, identify: identify,
		identities: make(map[fileIdentity]string), activeDirs: make(map[string]bool),
		visitedDirs: make(map[string]bool), partialDirs: make(map[string]bool),
	}
	for _, boundary := range boundaries {
		if err := collector.collect(boundary); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			return nil, fmt.Errorf("cannot inspect workspace %q; fix inaccessible or unsafe paths: %w",
				boundary, err)
		}
	}
	if err := preflightCancellation(ctx); err != nil {
		return nil, err
	}
	return collector.identities, nil
}

type workspaceIdentityCollector struct {
	ctx         context.Context
	cloneRoot   string
	boundaries  []string
	traversal   localTraversal
	identify    identityFunc
	identities  map[fileIdentity]string
	activeDirs  map[string]bool
	visitedDirs map[string]bool
	partialDirs map[string]bool
}

func (collector *workspaceIdentityCollector) collect(path string) error {
	if err := preflightCancellation(collector.ctx); err != nil {
		return err
	}
	if collector.cloneRoot != "" {
		inside, err := pathWithin(collector.cloneRoot, path)
		if err != nil {
			return fmt.Errorf("checking clone path %q: %w", path, err)
		}
		if inside {
			return nil
		}
	}
	info, err := collector.traversal.lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !collector.isBoundary(path) {
			return nil
		}
		return fmt.Errorf("accessing %q: %w", path, err)
	}
	if err := preflightCancellation(collector.ctx); err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		resolved, err := resolveAbsolutePath(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("resolving workspace link %q: %w", path, err)
		}
		insideBoundary := false
		for _, boundary := range collector.boundaries {
			inside, err := pathWithin(boundary, resolved)
			if err != nil {
				return fmt.Errorf("checking workspace link %q: %w", path, err)
			}
			if inside {
				insideBoundary = true
				break
			}
		}
		if !insideBoundary {
			return nil
		}
		info, err = collector.traversal.lstat(resolved)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("accessing workspace link %q: %w", path, err)
		}
	}
	if info.IsDir() {
		return collector.collectDirectory(path)
	}
	if info.Mode().IsRegular() {
		identity, err := collector.identify(path, info)
		if err != nil {
			return fmt.Errorf("identifying workspace file %q: %w", path, err)
		}
		if err := preflightCancellation(collector.ctx); err != nil {
			return err
		}
		collector.identities[identity] = path
	}
	return nil
}

func (collector *workspaceIdentityCollector) isBoundary(path string) bool {
	for _, boundary := range collector.boundaries {
		if path == boundary {
			return true
		}
	}
	return false
}

func (collector *workspaceIdentityCollector) collectDirectory(path string) error {
	resolved, err := resolveAbsolutePath(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !collector.isBoundary(path) {
			return nil
		}
		return fmt.Errorf("resolving workspace directory %q: %w", path, err)
	}
	if collector.activeDirs[resolved] {
		return fmt.Errorf("workspace directory cycle at %q; remove the cycle", path)
	}
	if collector.visitedDirs[resolved] {
		return nil
	}
	partial := false
	if collector.cloneRoot != "" {
		partial, err = pathWithin(path, collector.cloneRoot)
		if err != nil {
			return fmt.Errorf("checking clone path in %q: %w", path, err)
		}
	}
	if partial && collector.partialDirs[resolved] {
		return nil
	}
	// An alias outside the clone may expose files omitted by an earlier walk.
	// Track partial and complete walks separately to retain those identities.
	if partial {
		collector.partialDirs[resolved] = true
	} else {
		collector.visitedDirs[resolved] = true
	}
	collector.activeDirs[resolved] = true
	defer delete(collector.activeDirs, resolved)
	entries, err := collector.traversal.readDir(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !collector.isBoundary(path) {
			return nil
		}
		return fmt.Errorf("reading workspace directory %q: %w", path, err)
	}
	if err := preflightCancellation(collector.ctx); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := collector.collect(filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return preflightCancellation(collector.ctx)
}
