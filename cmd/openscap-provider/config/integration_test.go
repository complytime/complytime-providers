// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCPEMatching_RealSystem tests CPE-based datastream matching against the
// actual system's os-release and installed SSG content.
// Run with: go test -tags=integration -v ./cmd/openscap-provider/config
func TestCPEMatching_RealSystem(t *testing.T) {
	// Get system CPE
	cpe, err := getSystemCPE()
	require.NoError(t, err, "Failed to read CPE_NAME from %s", getSystemInfoFile())
	t.Logf("System CPE: %s", cpe)

	// Find matching datastream
	ds, err := findMatchingDatastream()
	require.NoError(t, err, "Failed to find matching datastream")
	t.Logf("Matched datastream: %s", ds)

	// Verify it's a CentOS Stream 10 datastream
	require.Contains(t, ds, "ssg-cs10-ds.xml",
		"Expected to match CentOS Stream 10 datastream")
}

// TestDatastreamDirEnvVar verifies that SSG_CONTENT_DIR environment variable
// overrides the default datastream directory.
func TestDatastreamDirEnvVar(t *testing.T) {
	// Save original env
	original := os.Getenv(DatastreamsDirEnvVar)
	defer func() {
		if original != "" {
			os.Setenv(DatastreamsDirEnvVar, original)
		} else {
			os.Unsetenv(DatastreamsDirEnvVar)
		}
	}()

	t.Run("nonexistent directory produces clear error", func(t *testing.T) {
		customDir := "/nonexistent/ssg/path"
		require.NoError(t, os.Setenv(DatastreamsDirEnvVar, customDir))

		_, err := findMatchingDatastream()
		require.Error(t, err)
		require.Contains(t, err.Error(), customDir)
		require.Contains(t, err.Error(), "Install scap-security-guide")
		require.Contains(t, err.Error(), DatastreamsDirEnvVar)
	})

	t.Run("valid directory works", func(t *testing.T) {
		require.NoError(t, os.Setenv(DatastreamsDirEnvVar, "/usr/share/xml/scap/ssg/content"))

		ds, err := findMatchingDatastream()
		require.NoError(t, err)
		require.Contains(t, ds, "ssg-cs10-ds.xml")
	})
}

// TestSystemInfoFileEnvVar verifies that OS_RELEASE_FILE environment variable
// overrides the default os-release file location.
func TestSystemInfoFileEnvVar(t *testing.T) {
	// Save original env
	original := os.Getenv(SystemInfoFileEnvVar)
	defer func() {
		if original != "" {
			os.Setenv(SystemInfoFileEnvVar, original)
		} else {
			os.Unsetenv(SystemInfoFileEnvVar)
		}
	}()

	t.Run("custom os-release file is read", func(t *testing.T) {
		// Create custom os-release file
		tmpFile := filepath.Join(t.TempDir(), "custom-os-release")
		err := os.WriteFile(tmpFile, []byte(`NAME="Test Linux"
VERSION="1.0"
ID=test
VERSION_ID="1.0"
CPE_NAME="cpe:/o:test:test:1"
`), 0644)
		require.NoError(t, err)

		require.NoError(t, os.Setenv(SystemInfoFileEnvVar, tmpFile))

		cpe, err := getSystemCPE()
		require.NoError(t, err)
		require.Equal(t, "cpe:/o:test:test:1", cpe)
	})

	t.Run("missing file produces error", func(t *testing.T) {
		require.NoError(t, os.Setenv(SystemInfoFileEnvVar, "/nonexistent/os-release"))

		_, err := getSystemCPE()
		require.Error(t, err)
		require.Contains(t, err.Error(), "/nonexistent/os-release")
	})
}
