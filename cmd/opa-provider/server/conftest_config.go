// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	conftestIgnoreEnv     = "CONFTEST_IGNORE"
	conftestConfigFileEnv = "CONFTEST_CONFIG_FILE"
)

func detectConftestConfig(explicitPath string) (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determining conftest working directory: %w", err)
	}
	return findConftestConfig(workingDir, explicitPath, os.Stat)
}

func findConftestConfig(
	workingDir, explicitPath string, stat func(string) (os.FileInfo, error),
) (string, error) {
	if explicitPath != "" {
		path := explicitPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(workingDir, path)
		}
		info, err := stat(path)
		if err != nil {
			return "", fmt.Errorf("checking conftest configuration %q: %w", path, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("conftest configuration %q is a directory", path)
		}
		return path, nil
	}

	// Match v0.68.2 discovery order, including formats which discovery finds
	// but its default decoder cannot parse: conftest fails on those files too.
	configNames := [...]string{
		"conftest.json", "conftest.toml", "conftest.yaml", "conftest.yml",
		"conftest.properties", "conftest.props", "conftest.prop", "conftest.hcl",
		"conftest.tfvars", "conftest.dotenv", "conftest.env", "conftest.ini",
	}
	for _, name := range configNames {
		path := filepath.Join(workingDir, name)
		info, err := stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("searching for conftest configuration at %q: %w", path, err)
		}
		if !info.IsDir() {
			return path, nil
		}
	}
	return "", nil
}
