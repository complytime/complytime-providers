// SPDX-License-Identifier: Apache-2.0

package toolcheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckTools_KnownGoodTool(t *testing.T) {
	// Temporarily override RequiredTools with a tool that exists on all systems
	origTools := RequiredTools
	RequiredTools = []string{"ls"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	require.NoError(t, err)
	require.Empty(t, missing)
}

func TestCheckTools_OneMissing(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"ls", "nonexistent-tool-xyz-12345"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	require.NoError(t, err)
	require.Len(t, missing, 1)
	require.Equal(t, "nonexistent-tool-xyz-12345", missing[0])
}

func TestCheckTools_AllMissing(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"nonexistent-tool-abc", "nonexistent-tool-xyz"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	require.NoError(t, err)
	require.Len(t, missing, 2)
	require.Contains(t, missing, "nonexistent-tool-abc")
	require.Contains(t, missing, "nonexistent-tool-xyz")
}

func TestFormatMissingToolsError_OneTool(t *testing.T) {
	err := FormatMissingToolsError([]string{"ampel"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ampel")
	require.Contains(t, err.Error(), "PATH")
}

func TestFormatMissingToolsError_MultipleTools(t *testing.T) {
	err := FormatMissingToolsError([]string{"snappy", "ampel"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "snappy")
	require.Contains(t, err.Error(), "ampel")
}

func TestFormatMissingToolsError_MentionsPATH(t *testing.T) {
	err := FormatMissingToolsError([]string{"snappy"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PATH")
}

func TestFormatMissingToolsError_Empty(t *testing.T) {
	err := FormatMissingToolsError(nil)
	require.NoError(t, err)
}

func TestCheckVersion_AmpelNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := CheckVersion("1.4.2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ampel command not found")
}

func TestCheckVersion_OutputParsing(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name:   "standard ampel output",
			output: "ampel version 1.4.2\n",
			want:   "1.4.2",
		},
		{
			name:   "version only",
			output: "1.4.2",
			want:   "1.4.2",
		},
		{
			name:   "verbose output with build info",
			output: "ampel CLI v1.5.0 (built 2026-01-15)",
			want:   "1.5.0",
		},
		{
			name:    "no version in output",
			output:  "ampel is running",
			wantErr: true,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseVersionOutput(tc.output)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(),
					"could not parse version")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCheckVersion_Comparison(t *testing.T) {
	tests := []struct {
		name     string
		detected string
		minimum  string
		wantErr  bool
	}{
		{
			name:     "exact match",
			detected: "1.4.2",
			minimum:  "1.4.2",
		},
		{
			name:     "above minimum patch",
			detected: "1.4.3",
			minimum:  "1.4.2",
		},
		{
			name:     "above minimum minor",
			detected: "1.5.0",
			minimum:  "1.4.2",
		},
		{
			name:     "above minimum major",
			detected: "2.0.0",
			minimum:  "1.4.2",
		},
		{
			name:     "below minimum patch",
			detected: "1.4.1",
			minimum:  "1.4.2",
			wantErr:  true,
		},
		{
			name:     "below minimum minor",
			detected: "1.3.9",
			minimum:  "1.4.2",
			wantErr:  true,
		},
		{
			name:     "below minimum major",
			detected: "0.9.9",
			minimum:  "1.4.2",
			wantErr:  true,
		},
		{
			name:     "invalid detected version",
			detected: "notaversion",
			minimum:  "1.4.2",
			wantErr:  true,
		},
		{
			name:     "invalid minimum version",
			detected: "1.4.2",
			minimum:  "bad",
			wantErr:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := compareVersions(tc.detected, tc.minimum)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
