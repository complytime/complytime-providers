// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	benchmarkDirectories = 100
	benchmarkFilesPerDir = 100
	benchmarkContent     = `{"blocked":true}`
)

// BenchmarkWorkspacePreflightLargeTree reports preflight and real-conftest
// times on the same fixture. Setup is excluded from both measurements.
func BenchmarkWorkspacePreflightLargeTree(b *testing.B) {
	benchmarkWorkspacePreflight(b, benchmarkDirectories)
}

func BenchmarkWorkspacePreflightHalfTree(b *testing.B) {
	benchmarkWorkspacePreflight(b, benchmarkDirectories/2)
}

func benchmarkWorkspacePreflight(b *testing.B, directoryCount int) {
	base := b.TempDir()
	workspace := filepath.Join(base, "workspace")
	input := filepath.Join(base, "input")
	for _, root := range []string{workspace, input} {
		for directory := 0; directory < directoryCount; directory++ {
			path := filepath.Join(root, fmt.Sprintf("directory-%03d", directory))
			if err := os.MkdirAll(path, 0750); err != nil {
				b.Fatal(err)
			}
			for file := 0; file < benchmarkFilesPerDir; file++ {
				name := filepath.Join(path, fmt.Sprintf("input-%03d.json", file))
				if err := os.WriteFile(name, []byte(benchmarkContent), 0600); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	policy := filepath.Join(base, "policy")
	if err := os.Mkdir(policy, 0750); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(policy, "policy.rego"), []byte(
		"package main\ndeny contains \"blocked\" if input.blocked == true\n",
	), 0600); err != nil {
		b.Fatal(err)
	}
	fileCount := directoryCount * benchmarkFilesPerDir
	reportFixture := func(b *testing.B) {
		b.ReportMetric(float64(fileCount), "workspace-files")
		b.ReportMetric(float64(fileCount), "target-files")
		b.ReportMetric(float64(fileCount*len(benchmarkContent)), "workspace-content-bytes")
		b.ReportMetric(float64(fileCount*len(benchmarkContent)), "target-content-bytes")
	}
	b.Run("preflight", func(b *testing.B) {
		traversal := localTraversal{readDir: os.ReadDir, lstat: os.Lstat}
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			boundaries, err := workspaceBoundaries(nil, workspace)
			if err != nil {
				b.Fatal(err)
			}
			identities, err := workspaceIdentities(ctx, boundaries, "", traversal, fileIdentityOf)
			if err != nil {
				b.Fatal(err)
			}
			if err := inspectInput(ctx, input, boundaries, "", "", traversal, identities, fileIdentityOf); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		reportFixture(b)
	})
	b.Run("conftest", func(b *testing.B) {
		binary := requireRealConftest(b)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cmd := exec.Command(binary, "test", input, "--policy", policy, "--output", "json", "--no-fail")
			cmd.Dir = base
			cmd.Env = append(os.Environ(), "CONFTEST_IGNORE=", "CONFTEST_CONFIG_FILE=", "HOME="+base)
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			if err := cmd.Run(); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		reportFixture(b)
	})
}
