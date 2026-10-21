// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// ScanConfigFileName is the name of the generation artifact consumed by Scan.
	ScanConfigFileName = "scan-config.json"
)

// ScanConfig is the generation artifact written by Generate and read by Scan.
type ScanConfig struct {
	IDs            []string          `json:"ids"`
	ReverseMapping map[string]string `json:"reverse_mapping"`
	PolicyDir      string            `json:"bundle_dir"`
	GeneratedAt    string            `json:"generated_at"`
}

// WriteScanConfig writes a scan-config.json to the given directory.
func WriteScanConfig(
	dir string,
	ids []string,
	reverseMap map[string]string,
	policyDir string,
) error {
	cfg := ScanConfig{
		IDs:            ids,
		ReverseMapping: reverseMap,
		PolicyDir:      policyDir,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling scan config: %w", err)
	}

	path := filepath.Join(dir, ScanConfigFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing scan config: %w", err)
	}

	return nil
}

// ReadScanConfig reads a scan-config.json from the given directory. Returns
// an error if the file is missing or malformed.
func ReadScanConfig(dir string) (*ScanConfig, error) {
	path := filepath.Join(dir, ScanConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading scan config: %w", err)
	}

	var cfg ScanConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing scan config: %w", err)
	}

	return &cfg, nil
}
