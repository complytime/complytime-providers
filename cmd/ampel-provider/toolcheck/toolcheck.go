// SPDX-License-Identifier: Apache-2.0

package toolcheck

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// MinAmpelVersion is the minimum ampel CLI version required for
// --allow-skip support.
const MinAmpelVersion = "1.4.2"

// RequiredTools lists the external tools the provider depends on.
var RequiredTools = []string{"snappy", "ampel"}

// CheckTools verifies that all required tools are available on the system PATH.
// It returns a list of missing tool names.
func CheckTools() ([]string, error) {
	var missing []string
	for _, tool := range RequiredTools {
		_, err := exec.LookPath(tool)
		if err != nil {
			missing = append(missing, tool)
		}
	}
	return missing, nil
}

// versionPattern matches a major.minor.patch version string with an
// optional prerelease suffix (e.g., 1.4.2, 1.4.2-rc.1).
var versionPattern = regexp.MustCompile(
	`(\d+\.\d+\.\d+)(-[a-zA-Z0-9.]+)?`,
)

// CheckVersion runs "ampel version" and returns an error if the detected
// version is below minVersion. This ensures the provider does not pass
// flags (e.g. --allow-skip) that older ampel versions do not support.
func CheckVersion(minVersion string) error {
	cmdPath, err := exec.LookPath("ampel")
	if err != nil {
		return fmt.Errorf("ampel command not found: %w", err)
	}
	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second,
	)
	defer cancel()
	out, err := exec.CommandContext(
		ctx, cmdPath, "version",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"running 'ampel version': %w (output: %s)",
			err, strings.TrimSpace(string(out)),
		)
	}
	detected, err := parseVersionOutput(string(out))
	if err != nil {
		return err
	}
	return compareVersions(detected, minVersion)
}

// parseVersionOutput extracts a major.minor.patch version string from
// the output of "ampel version". It prefers the GitVersion field in
// structured output and rejects development or prerelease builds.
func parseVersionOutput(output string) (string, error) {
	// Prefer structured output: look for a GitVersion field.
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "GitVersion:") {
			continue
		}
		value := strings.TrimSpace(
			strings.TrimPrefix(trimmed, "GitVersion:"),
		)
		if value == "" || value == "devel" {
			return "", fmt.Errorf(
				"ampel reports development version %q;"+
					" a released version >= %s is required",
				value, MinAmpelVersion,
			)
		}
		return extractCleanVersion(value)
	}

	// Unstructured output: accept only if exactly one version
	// substring is present to avoid misidentifying GoVersion
	// or other metadata as the ampel version.
	matches := versionPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf(
			"could not parse version from ampel output: %q",
			output,
		)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf(
			"ambiguous ampel output contains multiple version"+
				" strings; cannot determine ampel version: %q",
			output,
		)
	}
	return extractCleanVersion(matches[0][0])
}

// extractCleanVersion validates that a version string is a clean
// X.Y.Z release without prerelease suffixes.
func extractCleanVersion(s string) (string, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf(
			"could not parse version from %q", s,
		)
	}
	if m[2] != "" {
		return "", fmt.Errorf(
			"ampel prerelease version %q is not supported;"+
				" a stable release >= %s is required",
			m[0], MinAmpelVersion,
		)
	}
	return m[1], nil
}

// compareVersions returns an error if detected is below minimum.
// Both must be in major.minor.patch format.
func compareVersions(detected, minimum string) error {
	d, err := parseSemver(detected)
	if err != nil {
		return fmt.Errorf("parsing detected version: %w", err)
	}
	m, err := parseSemver(minimum)
	if err != nil {
		return fmt.Errorf("parsing minimum version: %w", err)
	}
	if d[0] < m[0] ||
		(d[0] == m[0] && d[1] < m[1]) ||
		(d[0] == m[0] && d[1] == m[1] && d[2] < m[2]) {
		return fmt.Errorf(
			"ampel version %s is below minimum required %s",
			detected, minimum,
		)
	}
	return nil
}

// parseSemver splits a "major.minor.patch" string into three integers.
func parseSemver(v string) ([3]int, error) {
	var parts [3]int
	n, err := fmt.Sscanf(v, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
	if err != nil || n != 3 {
		return parts, fmt.Errorf(
			"invalid version format %q: expected major.minor.patch", v,
		)
	}
	return parts, nil
}

// FormatMissingToolsError constructs an error message listing each missing tool.
func FormatMissingToolsError(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"required tools not found: %s. Ensure the following tools are installed and available on your PATH: %s. See AMPEL documentation for installation instructions",
		strings.Join(missing, ", "),
		strings.Join(missing, ", "),
	)
}
