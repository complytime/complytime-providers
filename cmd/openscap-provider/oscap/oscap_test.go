// SPDX-License-Identifier: Apache-2.0

package oscap

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestConstructScanCommand(t *testing.T) {
	tests := []struct {
		name          string
		openscapFiles map[string]string
		profile       string
		expectedCmd   []string
	}{
		{
			name: "Scan command contruction",
			openscapFiles: map[string]string{
				"datastream": "test-datastream.xml",
				"policy":     "test-policy.xml",
				"results":    "test-results.xml",
				"arf":        "test-arf.xml",
			},
			profile: "test-profile",
			expectedCmd: []string{
				"oscap",
				"xccdf",
				"eval",
				"--profile",
				"test-profile",
				"--results",
				"test-results.xml",
				"--results-arf",
				"test-arf.xml",
				"--tailoring-file",
				"test-policy.xml",
				"test-datastream.xml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := constructScanCommand(tt.openscapFiles, tt.profile)
			if !reflect.DeepEqual(cmd, tt.expectedCmd) {
				t.Errorf("constructScanCommand() = %v, expected %v", cmd, tt.expectedCmd)
			}
		})
	}
}

func TestShellJoin(t *testing.T) {
	cmd := []string{"oscap", "xccdf", "eval", "--profile", "cis_l1", "datastream.xml"}
	got := shellJoin(cmd)
	expected := "oscap xccdf eval --profile cis_l1 datastream.xml"
	if got != expected {
		t.Errorf("shellJoin() = %q, expected %q", got, expected)
	}
}

func TestExecuteCommand_TimeoutIncludesCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := executeCommand(ctx, []string{"sleep", "10"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "timed out") && !strings.Contains(errMsg, "cancelled") {
		t.Errorf("expected timeout/cancel message, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "sleep 10") {
		t.Errorf("expected error to contain the command string 'sleep 10', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "To debug, run manually") {
		t.Errorf("expected error to contain debug hint, got: %s", errMsg)
	}
}

func TestExecuteCommand_NonTimeoutErrorOmitsCommand(t *testing.T) {
	_, err := executeCommand(context.Background(), []string{"nonexistent-binary-12345"})
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if strings.Contains(err.Error(), "To debug, run manually") {
		t.Error("non-timeout errors should not include the debug hint")
	}
}

func TestConstructGenerateFixCommand(t *testing.T) {
	tests := []struct {
		name          string
		fixType       string
		output        string
		profile       string
		tailoringFile string
		datastream    string
		expectedCmd   []string
	}{
		{
			name:          "Genereate fix command construction",
			fixType:       "bash",
			output:        "test-remediation-script.sh",
			profile:       "test-profile",
			tailoringFile: "test-policy.xml",
			datastream:    "test-datastream.xml",
			expectedCmd: []string{
				"oscap",
				"xccdf",
				"generate",
				"fix",
				"--fix-type", "bash",
				"--output", "test-remediation-script.sh",
				"--profile", "test-profile",
				"--tailoring-file", "test-policy.xml",
				"test-datastream.xml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := constructGenerateFixCommand(tt.fixType, tt.output, tt.profile, tt.tailoringFile, tt.datastream)
			if !reflect.DeepEqual(cmd, tt.expectedCmd) {
				t.Errorf("constructGenerateFixCommand() = %v, expected %v", cmd, tt.expectedCmd)
			}
		})
	}
}
