// SPDX-License-Identifier: Apache-2.0

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
)

func TestWriteTailoringFile_ReplacesLinksAndSupportsReruns(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			cfg := config.WorkspaceConfig{Root: t.TempDir()}
			require.NoError(t, cfg.EnsureDirectories())
			external := filepath.Join(t.TempDir(), "unrelated.xml")
			require.NoError(t, os.WriteFile(external, []byte("original"), 0o600))
			require.NoError(t, os.Chmod(external, 0o640))
			if kind == "symlink" {
				require.NoError(t, os.Symlink(external, cfg.PolicyPath()))
			} else {
				require.NoError(t, os.Link(external, cfg.PolicyPath()))
			}
			for range 2 {
				require.NoError(t, writeTailoringFile([]provider.AssessmentConfiguration{
					{RequirementID: "account_unique_id"},
				}, filepath.Join("..", "..", "..", "internal", "complytime", "testdata", "openscap", "ssg-rhel-ds.xml"), "test_profile", cfg))
				data, err := os.ReadFile(cfg.PolicyPath())
				require.NoError(t, err)
				assert.Contains(t, string(data), "Tailoring")
			}
			data, err := os.ReadFile(external)
			require.NoError(t, err)
			assert.Equal(t, "original", string(data))
			info, err := os.Stat(external)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
		})
	}
}
