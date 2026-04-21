// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/go-hclog"
)

const (
	// workspaceDir is the workspace-local directory used by complyctl for all artifacts.
	// Matches complyctl's internal/complytime.WorkspaceDir constant.
	workspaceDir string = ".complytime"

	ProviderDir    string = "openscap"
	PolicyDir      string = "policy"
	ResultsDir     string = "results"
	RemediationDir string = "remediations"
	DatastreamsDir string = "/usr/share/xml/scap/ssg/content"
	SystemInfoFile string = "/etc/os-release"
)

// Resolved convention-based file paths. These are constants — the provider
// always reads/writes to these locations under the workspace directory.
var (
	PolicyPath  = filepath.Join(workspaceDir, ProviderDir, PolicyDir, "tailoring.xml")
	ResultsPath = filepath.Join(workspaceDir, ProviderDir, ResultsDir, "results.xml")
	ARFPath     = filepath.Join(workspaceDir, ProviderDir, ResultsDir, "arf.xml")
)

// WorkspaceDir returns the workspace-local directory used by complyctl for all artifacts.
func WorkspaceDir() string {
	return workspaceDir
}

// EnsureDirectories creates the provider workspace directory structure.
// Called during Generate to guarantee paths exist before writing artifacts.
func EnsureDirectories() error {
	directories := []string{
		filepath.Join(workspaceDir, ProviderDir),
		filepath.Join(workspaceDir, ProviderDir, PolicyDir),
		filepath.Join(workspaceDir, ProviderDir, ResultsDir),
		filepath.Join(workspaceDir, ProviderDir, RemediationDir),
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

// getDistroIdsAndVersions returns distribution IDs and versions from SystemInfoFile.
func getDistroIdsAndVersions() ([]string, []string, error) {
	file, err := os.Open(SystemInfoFile)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var ids []string
	var versionID string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			id := strings.Trim(strings.Split(line, "=")[1], `"`)
			ids = append(ids, id)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.Split(line, "=")[1], `"`)
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			altIdString := strings.Trim(strings.Split(line, "=")[1], `"`)
			altIds := strings.Split(altIdString, " ")
			ids = append(ids, altIds...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	if ids != nil && versionID != "" {
		majorVersion := strings.Split(versionID, ".")[0]
		fullVersion := strings.ReplaceAll(versionID, ".", "")
		return ids, []string{majorVersion, fullVersion}, nil
	}

	return nil, nil, fmt.Errorf("could not determine distribution and version based on %s", SystemInfoFile)
}

func findMatchingDatastream() (string, error) {
	distroIds, distroVersions, err := getDistroIdsAndVersions()
	if err != nil {
		return "", err
	}

	var foundFile string

	err = filepath.Walk(DatastreamsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			for id := range distroIds {
				for version := range distroVersions {
					pattern := fmt.Sprintf("ssg-%s%s-ds.xml", distroIds[id], distroVersions[version])
					if info.Name() == pattern {
						foundFile = path
						return filepath.SkipDir
					}
				}
				pattern := fmt.Sprintf("ssg-%s-ds.xml", distroIds[id])
				if info.Name() == pattern {
					foundFile = path
					return filepath.SkipDir
				}
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	if foundFile != "" {
		return foundFile, nil
	}

	return "", fmt.Errorf("could not determine a datastream file for a system with ids: %v and versions: %v", distroIds, distroVersions)
}
