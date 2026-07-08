// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/complytime/complyctl/pkg/provider"
	"github.com/hashicorp/go-hclog"
)

const (
	ProviderDir           string = "openscap"
	PolicyDir             string = "policy"
	ResultsDir            string = "results"
	RemediationDir        string = "remediations"
	DefaultDatastreamsDir string = "/usr/share/xml/scap/ssg/content"
	DefaultSystemInfoFile string = "/etc/os-release"
	DatastreamsDirEnvVar  string = "SSG_CONTENT_DIR"
	SystemInfoFileEnvVar  string = "OS_RELEASE_FILE"
)

// getDatastreamsDir returns the directory containing SSG datastream files.
// Checks SSG_CONTENT_DIR environment variable first, falls back to default.
func getDatastreamsDir() string {
	if dir := os.Getenv(DatastreamsDirEnvVar); dir != "" {
		return filepath.Clean(dir)
	}
	return DefaultDatastreamsDir
}

// getSystemInfoFile returns the path to the os-release file.
// Checks OS_RELEASE_FILE environment variable first, falls back to default.
func getSystemInfoFile() string {
	if file := os.Getenv(SystemInfoFileEnvVar); file != "" {
		return filepath.Clean(file)
	}
	return DefaultSystemInfoFile
}

// normalizeCPE converts a CPE string to CPE 2.2 URI format for comparison.
// CPE 2.3 formatted strings (cpe:2.3:part:vendor:...) are converted to
// CPE 2.2 URI binding (cpe:/part:vendor:...) with trailing wildcards removed.
// CPE 2.2 URIs are returned unchanged.
func normalizeCPE(cpe string) string {
	const cpe23Prefix = "cpe:2.3:"
	if !strings.HasPrefix(cpe, cpe23Prefix) {
		return cpe
	}
	parts := strings.Split(strings.TrimPrefix(cpe, cpe23Prefix), ":")
	for len(parts) > 0 && parts[len(parts)-1] == "*" {
		parts = parts[:len(parts)-1]
	}
	return "cpe:/" + strings.Join(parts, ":")
}

// cpeMatches reports whether systemCPE and datastreamCPE identify the same
// platform. Both values are normalized to CPE 2.2 URI format, then compared
// using case-insensitive component-prefix matching: the shorter CPE must be
// a component-wise prefix of the longer one (components are colon-separated).
func cpeMatches(systemCPE, datastreamCPE string) bool {
	sys := strings.ToLower(normalizeCPE(systemCPE))
	ds := strings.ToLower(normalizeCPE(datastreamCPE))
	if sys == ds {
		return true
	}
	sysParts := strings.Split(sys, ":")
	dsParts := strings.Split(ds, ":")
	shorter, longer := sysParts, dsParts
	if len(sysParts) > len(dsParts) {
		shorter, longer = dsParts, sysParts
	}
	for i, part := range shorter {
		if part != longer[i] {
			return false
		}
	}
	return true
}

// Resolved convention-based file paths. These are constants — the provider
// always reads/writes to these locations under the workspace directory.
var (
	PolicyPath  = filepath.Join(provider.WorkspaceDir, ProviderDir, PolicyDir, "tailoring.xml")
	ResultsPath = filepath.Join(provider.WorkspaceDir, ProviderDir, ResultsDir, "results.xml")
	ARFPath     = filepath.Join(provider.WorkspaceDir, ProviderDir, ResultsDir, "arf.xml")
)

// EnsureDirectories creates the provider workspace directory structure.
// Called during Generate to guarantee paths exist before writing artifacts.
func EnsureDirectories() error {
	directories := []string{
		filepath.Join(provider.WorkspaceDir, ProviderDir),
		filepath.Join(provider.WorkspaceDir, ProviderDir, PolicyDir),
		filepath.Join(provider.WorkspaceDir, ProviderDir, ResultsDir),
		filepath.Join(provider.WorkspaceDir, ProviderDir, RemediationDir),
	}
	for _, dir := range directories {
		if err := ensureDirectory(dir); err != nil {
			return fmt.Errorf("failed to ensure directory %s: %w", dir, err)
		}
	}
	return nil
}

// ResolveDatastream validates a provided datastream path or auto-detects
// the system's datastream from /usr/share/xml/scap/ssg/content when the
// path is empty.
func ResolveDatastream(path string) (string, error) {
	if path == "" {
		return findMatchingDatastream()
	}

	cleanPath, err := SanitizePath(path)
	if err != nil {
		return "", err
	}

	if _, err := validatePath(cleanPath, false); err != nil {
		return "", fmt.Errorf("invalid datastream path: %s: %w", cleanPath, err)
	}

	isXML, err := IsXMLFile(cleanPath)
	if err != nil || !isXML {
		return "", fmt.Errorf("invalid datastream file: %s: %w", cleanPath, err)
	}

	return cleanPath, nil
}

func SanitizeInput(input string) (string, error) {
	safePattern := regexp.MustCompile(`^[a-zA-Z0-9-_.]+$`)
	if !safePattern.MatchString(input) {
		return "", fmt.Errorf("input contains unexpected characters: %s", input)
	}
	return input, nil
}

func SanitizePath(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	expandedPath, err := expandPath(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to expand path: %w", err)
	}
	return expandedPath, nil
}

func IsXMLFile(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	for {
		_, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				return true, nil
			}
			return false, fmt.Errorf("invalid XML file %s: %w", filePath, err)
		}
	}
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("failed to identify current user: %w", err)
		}
		return filepath.Join(usr.HomeDir, path[1:]), nil
	}
	return path, nil
}

func validatePath(path string, shouldBeDir bool) (string, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to confirm path existence: %w", err)
	}

	if shouldBeDir && !stat.IsDir() {
		return "", fmt.Errorf("expected a directory, but found a file at path: %s", path)
	}
	if !shouldBeDir && stat.IsDir() {
		return "", fmt.Errorf("expected a file, but found a directory at path: %s", path)
	}

	return path, nil
}

func ensureDirectory(path string) error {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(path, 0750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		hclog.Default().Info("Directory created", "path", path)
	} else if err != nil {
		return fmt.Errorf("error checking directory: %w", err)
	}
	return nil
}

// extractCPEFromOsRelease parses CPE_NAME from an os-release formatted reader.
// Returns an error if CPE_NAME is missing.
func extractCPEFromOsRelease(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "CPE_NAME=") {
			cpe := strings.Trim(strings.SplitN(line, "=", 2)[1], `"'`)
			if cpe == "" {
				return "", fmt.Errorf("CPE_NAME field is empty")
			}
			return cpe, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading os-release data: %w", err)
	}

	return "", fmt.Errorf("CPE_NAME not found in os-release")
}

// getSystemCPE returns the CPE_NAME from the system's os-release file.
func getSystemCPE() (string, error) {
	systemInfoFile := getSystemInfoFile()
	file, err := os.Open(systemInfoFile)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %w", systemInfoFile, err)
	}
	defer file.Close()
	return extractCPEFromOsRelease(file)
}

// extractCPEFromDatastream parses a datastream XML file and extracts all CPE
// names from cpe-dict:cpe-item elements.
func extractCPEFromDatastream(fsys fs.FS, filename string) ([]string, error) {
	file, err := fsys.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open datastream %s: %w", filename, err)
	}
	defer file.Close()

	doc, err := xmlquery.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML from %s: %w", filename, err)
	}

	// Query for all cpe-item elements with name attributes.
	// Use local-name() to ignore namespace prefixes (cpe-dict:cpe-item, etc.)
	nodes := xmlquery.Find(doc, "//*[local-name()='cpe-item'][@name]")
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no CPE entries found in %s", filename)
	}

	var cpes []string
	for _, node := range nodes {
		if cpeName := node.SelectAttr("name"); cpeName != "" {
			cpes = append(cpes, cpeName)
		}
	}

	if len(cpes) == 0 {
		return nil, fmt.Errorf(
			"no valid CPE entries found in %s (all name attributes empty)",
			filename,
		)
	}

	return cpes, nil
}

// matchDatastreamByCPE searches fsys for an SSG datastream file whose CPE
// dictionary matches the system's CPE. Returns the first matching filename.
// The datastreamDir parameter is used only in error messages to help users
// identify the directory that was searched.
func matchDatastreamByCPE(fsys fs.FS, systemCPE, datastreamDir string) (string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", fmt.Errorf("reading datastream directory: %w", err)
	}

	var checked []string
	var skipped []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "ssg-") || !strings.HasSuffix(name, "-ds.xml") {
			continue
		}

		cpes, err := extractCPEFromDatastream(fsys, name)
		if err != nil {
			hclog.Default().Debug("skipping datastream", "file", name, "error", err.Error())
			skipped = append(skipped, name)
			continue
		}

		checked = append(checked, name)
		for _, cpe := range cpes {
			if cpeMatches(systemCPE, cpe) {
				return name, nil
			}
		}
	}

	if len(checked) == 0 && len(skipped) > 0 {
		hclog.Default().Warn("all SSG datastream files failed CPE extraction", "files", skipped)
		return "", fmt.Errorf("no valid SSG datastreams found in %s (%d files failed to parse). Verify file permissions and integrity, or set %s environment variable to the SSG content directory",
			datastreamDir, len(skipped), DatastreamsDirEnvVar)
	}

	if len(checked) == 0 {
		return "", fmt.Errorf("no valid SSG datastreams found in %s. Install scap-security-guide or set %s environment variable to the SSG content directory",
			datastreamDir, DatastreamsDirEnvVar)
	}

	return "", fmt.Errorf("no datastream matches system CPE %s (checked: %v). Set variables.datastream explicitly to override auto-detection",
		systemCPE, checked)
}

// findMatchingDatastream auto-detects the appropriate SSG datastream by
// matching the system's CPE_NAME against CPE entries in datastream files.
func findMatchingDatastream() (string, error) {
	systemCPE, err := getSystemCPE()
	if err != nil {
		return "", fmt.Errorf("failed to determine system CPE: %w. Set variables.datastream explicitly to override auto-detection", err)
	}

	datastreamDir := getDatastreamsDir()

	// Check if directory exists
	if _, err := os.Stat(datastreamDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("datastream directory %s does not exist. Install scap-security-guide or set %s environment variable to the SSG content directory. Alternatively, set variables.datastream explicitly",
				datastreamDir, DatastreamsDirEnvVar)
		}
		return "", fmt.Errorf("cannot access datastream directory %s: %w", datastreamDir, err)
	}

	fsys := os.DirFS(datastreamDir)
	name, err := matchDatastreamByCPE(fsys, systemCPE, datastreamDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(datastreamDir, name), nil
}
