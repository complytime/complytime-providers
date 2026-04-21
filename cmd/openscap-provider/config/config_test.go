// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"valid-input", "valid-input", false},
		{"another_valid.input", "another_valid.input", false},
		{"CAPS_and_numbers123", "CAPS_and_numbers123", false},
		{"mixed-123.UP_case", "mixed-123.UP_case", false},
		{"invalid/input", "", true},
		{"input with spaces", "", true},
		{"invalid@input", "", true},
		{"<invalid>", "", true},
		{";ls", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := SanitizeInput(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
			if result != tt.expected {
				t.Errorf("Expected result: %s, got: %s", tt.expected, result)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	usr, _ := user.Current()
	homeDir := usr.HomeDir

	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"/foo/bar/../baz", "/foo/baz", false},
		{"./foo/bar", "foo/bar", false},
		{"foo/./bar", "foo/bar", false},
		{"foo/bar/..", "foo", false},
		{"/foo//bar", "/foo/bar", false},
		{"foo//bar//baz", "foo/bar/baz", false},
		{"foo/bar/../../baz", "baz", false},
		{"./../foo", "../foo", false},
		{"~/foo/bar", filepath.Join(homeDir, "foo", "bar"), false},
		{"~", homeDir, false},
		{"~weird", "~weird", false},
		{"", ".", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := SanitizePath(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}
			if result != tt.expected {
				t.Errorf("Expected result: %s, got: %s", tt.expected, result)
			}
		})
	}
}

func setupTestFiles() error {
	if err := os.MkdirAll("testdata", os.ModePerm); err != nil {
		return err
	}

	if err := os.WriteFile("testdata/valid.xml", []byte(`<root></root>`), 0600); err != nil {
		return err
	}
	if err := os.WriteFile("testdata/invalid.xml", []byte(`<root>`), 0600); err != nil {
		return err
	}
	return nil
}

func teardownTestFiles() {
	os.RemoveAll("testdata")
}

func TestIsXMLFile(t *testing.T) {
	if err := setupTestFiles(); err != nil {
		t.Fatalf("Failed to setup test files: %v", err)
	}
	defer teardownTestFiles()

	tests := []struct {
		name      string
		filePath  string
		want      bool
		expectErr bool
	}{
		{"Valid XML file", "testdata/valid.xml", true, false},
		{"Invalid XML file", "testdata/invalid.xml", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isXML, err := IsXMLFile(tt.filePath)
			if (err != nil) != tt.expectErr {
				t.Errorf("IsXMLFile(%s) error = %v, expectErr %v", tt.filePath, err, tt.expectErr)
				return
			}
			if isXML != tt.want {
				t.Errorf("IsXMLFile() = %v, want %v", isXML, tt.want)
			}
		})
	}
}

func TestEnsureDirectory(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		path        string
		expectError bool
	}{
		{filepath.Join(tempDir, "absent_dir"), false},
		{filepath.Join(tempDir, "existing_dir"), false},
		{tempDir + "/invalid\000dir", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if tt.path == filepath.Join(tempDir, "existing_dir") {
				if err := os.MkdirAll(tt.path, 0750); err != nil {
					t.Fatalf("Failed to create directory: %v", err)
				}
			}

			err := ensureDirectory(tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, got: %v", tt.expectError, err)
			}

			if !tt.expectError {
				if _, err := os.Stat(tt.path); os.IsNotExist(err) {
					t.Errorf("Expected directory to be created: %s", tt.path)
				}
			}
		})
	}
}

func TestEnsureDirectories(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() { os.Chdir(originalDir) })

	require.NoError(t, EnsureDirectories())

	expectedDirs := []string{
		filepath.Join(".complytime", ProviderDir),
		filepath.Join(".complytime", ProviderDir, PolicyDir),
		filepath.Join(".complytime", ProviderDir, ResultsDir),
		filepath.Join(".complytime", ProviderDir, RemediationDir),
	}
	for _, dir := range expectedDirs {
		_, statErr := os.Stat(dir)
		require.NoError(t, statErr, "Expected directory to be created: %s", dir)
	}
}

func TestResolveDatastream(t *testing.T) {
	tempDir := t.TempDir()
	validDS := filepath.Join(tempDir, "ds.xml")
	require.NoError(t, os.WriteFile(validDS, []byte(`<root></root>`), 0600))

	t.Run("explicit valid path", func(t *testing.T) {
		result, err := ResolveDatastream(validDS)
		require.NoError(t, err)
		require.Equal(t, validDS, result)
	})

	t.Run("nonexistent path", func(t *testing.T) {
		_, err := ResolveDatastream(filepath.Join(tempDir, "missing.xml"))
		require.Error(t, err)
	})
}
