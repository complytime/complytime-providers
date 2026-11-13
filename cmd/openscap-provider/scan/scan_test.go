// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOpenSCAPFiles_ExplicitPaths(t *testing.T) {
	for _, scenario := range []string{"valid", "missing", "invalid"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			options := Options{Workspace: root, PolicyPath: filepath.Join(root, "tailoring.xml"),
				DatastreamPath: filepath.Join(root, "datastream.xml"), Profile: "stig",
				ResultsPath: filepath.Join(root, "evaluation", "results.xml"),
				ARFPath:     filepath.Join(root, "evaluation", "arf.xml")}
			if scenario != "missing" {
				data := "<Tailoring/>"
				if scenario == "invalid" {
					data = "<broken"
				}
				require.NoError(t, os.WriteFile(options.PolicyPath, []byte(data), 0o600))
			}
			files, err := validateOpenSCAPFiles(options)
			if scenario != "valid" {
				require.Error(t, err)
				assert.Nil(t, files)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, map[string]string{"datastream": options.DatastreamPath,
				"policy": options.PolicyPath, "results": options.ResultsPath, "arf": options.ARFPath}, files)
		})
	}
}
