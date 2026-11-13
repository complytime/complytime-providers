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

	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/internal/evidence"
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

// BenchmarkWorkspacePreflightRetainedHistory measures the existing identity walk
// with actual private evaluation leaves. It does not skip historical identities.
func BenchmarkWorkspacePreflightRetainedHistory(b *testing.B) {
	for _, count := range []int{100, 1000} {
		b.Run(fmt.Sprintf("evaluations-%d", count), func(b *testing.B) {
			root := b.TempDir()
			cfg := config.NewWorkspaceConfig(root)
			if err := cfg.EnsureDirectories(); err != nil {
				b.Fatal(err)
			}
			identity, err := evidence.ExecutionIdentity()
			if err != nil {
				b.Fatal(err)
			}
			for range count {
				evaluation, err := evidence.Allocate(cfg.StorageOptions(identity), "target", "main")
				if err != nil {
					b.Fatal(err)
				}
				if err := evaluation.Write(evidence.ConftestFile, []byte(conftestAllSuccessJSON)); err != nil {
					b.Fatal(err)
				}
				if err := evaluation.WriteMetadata(config.ProviderDir, evidence.TargetContext{TargetID: "target"}, evidence.ConfigurationContext{}); err != nil {
					b.Fatal(err)
				}
				if err := evaluation.Finalize(); err != nil {
					b.Fatal(err)
				}
				if err := evaluation.Close(); err != nil {
					b.Fatal(err)
				}
			}
			input := filepath.Join(root, "input.json")
			if err := os.WriteFile(input, []byte(`{}`), 0600); err != nil {
				b.Fatal(err)
			}
			boundaries := []string{cfg.WorkspaceDir}
			traversal := localTraversal{readDir: os.ReadDir, lstat: os.Lstat}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				identities, err := workspaceIdentities(context.Background(), boundaries, "", traversal, fileIdentityOf)
				if err != nil {
					b.Fatal(err)
				}
				if err := inspectInput(context.Background(), input, boundaries, "", "", traversal, identities, fileIdentityOf); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(count), "evaluations")
			b.ReportMetric(float64(2*count), "retained-files")
		})
	}
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
