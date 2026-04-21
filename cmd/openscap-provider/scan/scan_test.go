// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"fmt"
	"os"
	"testing"
)

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

func TestMain(m *testing.M) {
	if err := setupTestFiles(); err != nil {
		fmt.Printf("Failed to setup test files: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	teardownTestFiles()
	os.Exit(code)
}

func TestValidateOpenSCAPFiles(t *testing.T) {
	tests := []struct {
		name       string
		policyPath string
		wantErr    bool
	}{
		{
			name:       "present and valid policy file",
			policyPath: "testdata/valid.xml",
			wantErr:    false,
		},
		{
			name:       "absent policy file",
			policyPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateOpenSCAPFiles(tt.policyPath, "testdata/valid.xml")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOpenSCAPFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
