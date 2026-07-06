package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	c := NewConfig()
	require.NotNil(t, c)
}

func TestGranularPolicyDirPath(t *testing.T) {
	expected := filepath.Join(provider.WorkspaceDir, ProviderDir, DefaultGranularPolicyDir)
	require.Equal(t, expected, GranularPolicyDirPath())
}

func TestResultsDirPath(t *testing.T) {
	expected := filepath.Join(provider.WorkspaceDir, ProviderDir, DefaultResultsDir)
	require.Equal(t, expected, ResultsDirPath())
}

func TestGeneratedPolicyDirPath(t *testing.T) {
	expected := filepath.Join(provider.WorkspaceDir, ProviderDir, GeneratedPolicyDir)
	require.Equal(t, expected, GeneratedPolicyDirPath())
}

func TestScanConfigDirPath(t *testing.T) {
	expected := filepath.Join(provider.WorkspaceDir, ProviderDir, GeneratedPolicyDir)
	require.Equal(t, expected, ScanConfigDirPath())
}

func TestSpecDirPath(t *testing.T) {
	expected := filepath.Join(provider.WorkspaceDir, ProviderDir, "specs")
	require.Equal(t, expected, SpecDirPath())
}

func TestEnsureDirectories(t *testing.T) {
	dir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	err = EnsureDirectories()
	require.NoError(t, err)

	require.DirExists(t, filepath.Join(dir, provider.WorkspaceDir, ProviderDir))
	require.DirExists(t, filepath.Join(dir, provider.WorkspaceDir, ProviderDir, DefaultGranularPolicyDir))
	require.DirExists(t, filepath.Join(dir, provider.WorkspaceDir, ProviderDir, GeneratedPolicyDir))
	require.DirExists(t, filepath.Join(dir, provider.WorkspaceDir, ProviderDir, DefaultResultsDir))
	require.DirExists(t, filepath.Join(dir, provider.WorkspaceDir, ProviderDir, "specs"))
}
