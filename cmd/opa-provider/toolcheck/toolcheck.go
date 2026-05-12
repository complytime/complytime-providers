// SPDX-License-Identifier: Apache-2.0

package toolcheck

import (
	"fmt"
	"os/exec"
	"strings"
)

// RequiredTools lists the external tools the provider depends on.
var RequiredTools = []string{"conftest", "git"}

// CheckTools verifies that all required tools are available on the system PATH.
// It returns a list of missing tool names.
func CheckTools() []string {
	var missing []string
	for _, tool := range RequiredTools {
		_, err := exec.LookPath(tool)
		if err != nil {
			missing = append(missing, tool)
		}
	}
	return missing
}

// FormatMissingToolsError constructs an error message listing each missing tool.
func FormatMissingToolsError(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"required tools not found: %s. Ensure the following tools are installed and available on your PATH: %s",
		strings.Join(missing, ", "),
		strings.Join(missing, ", "),
	)
}
