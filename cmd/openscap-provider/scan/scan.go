// SPDX-License-Identifier: Apache-2.0

package scan

import (
	"context"
	"fmt"
	"os"

	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/oscap"
	"github.com/complytime/complytime-providers/cmd/openscap-provider/xccdf"
)

// Options carries explicit workspace, input and fresh evaluation-output paths.
// The caller reserves private outputs before execution and validates them afterward.
type Options struct {
	Workspace, PolicyPath, DatastreamPath, Profile, ResultsPath, ARFPath string
}

func validateOpenSCAPFiles(options Options) (map[string]string, error) {
	policyPath := options.PolicyPath
	if _, err := os.Stat(policyPath); err != nil {
		return nil, err
	}

	isXML, err := config.IsXMLFile(policyPath)
	if err != nil {
		return nil, err
	}
	if !isXML {
		return nil, fmt.Errorf("tailoring file is not XML")
	}

	return map[string]string{
		"datastream": options.DatastreamPath,
		"policy":     policyPath,
		"results":    options.ResultsPath,
		"arf":        options.ARFPath,
	}, nil
}

// ScanSystem runs OpenSCAP synchronously using only the explicitly supplied paths.
// Valid noncompliant outcomes succeed; tool/input failures return an error.
func ScanSystem(ctx context.Context, options Options) ([]byte, error) {
	openscapFiles, err := validateOpenSCAPFiles(options)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("absent openscap files: %w\n\nDid you run the generate command?", err)
		}
		return nil, fmt.Errorf("invalid openscap files: %w", err)
	}

	tailoringProfile := fmt.Sprintf("%s_%s", options.Profile, xccdf.XCCDFTailoringSuffix)

	output, err := oscap.OscapScan(ctx, openscapFiles, tailoringProfile, options.Workspace)
	if err != nil {
		return output, fmt.Errorf("failed during scan: %w", err)
	}

	return output, nil
}
