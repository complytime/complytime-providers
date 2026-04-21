// SPDX-License-Identifier: Apache-2.0

package oscap

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/complytime/complytime-providers/cmd/openscap-provider/config"
	"github.com/hashicorp/go-hclog"
)

// shellJoin formats a command slice as a copy-pasteable shell string.
func shellJoin(command []string) string {
	return strings.Join(command, " ")
}

func executeCommand(ctx context.Context, command []string) ([]byte, error) {
	cmdPath, err := exec.LookPath(command[0])
	if err != nil {
		return nil, fmt.Errorf("command not found: %s: %w", command[0], err)
	}

	cmdStr := shellJoin(command)
	hclog.Default().Info("Executing command", "command", cmdStr)
	cmd := exec.CommandContext(ctx, cmdPath, command[1:]...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return output, fmt.Errorf(
				"command timed out after deadline: %w\n\nTo debug, run manually:\n  %s",
				ctx.Err(), cmdStr,
			)
		}
		if err.Error() == "exit status 1" {
			return output, fmt.Errorf("oscap error during evaluation: %w", err)
		} else if err.Error() == "exit status 2" {
			hclog.Default().Warn("at least one rule resulted in fail or unknown", "err", err)
			return output, nil
		} else {
			return nil, err
		}
	}
	return output, nil
}

func constructScanCommand(openscapFiles map[string]string, profile string) []string {
	datastream := openscapFiles["datastream"]
	tailoringFile := openscapFiles["policy"]
	resultsFile := openscapFiles["results"]
	arfFile := openscapFiles["arf"]

	cmd := []string{
		"oscap",
		"xccdf",
		"eval",
		"--profile", profile,
		"--results", resultsFile,
		"--results-arf", arfFile,
		"--tailoring-file", tailoringFile,
		datastream,
	}

	return cmd
}

func OscapScan(ctx context.Context, openscapFiles map[string]string, profile string) ([]byte, error) {
	command := constructScanCommand(openscapFiles, profile)

	return executeCommand(ctx, command)
}

func constructGenerateFixCommand(fixType, output, profile, tailoringFile, datastream string) []string {

	cmd := []string{
		"oscap",
		"xccdf",
		"generate",
		"fix",
		"--fix-type", fixType,
		"--output", output,
		"--profile", profile,
		"--tailoring-file", tailoringFile,
		datastream,
	}
	return cmd
}

func OscapGenerateFix(ctx context.Context, providerDir, profile, policyFile, datastream string) error {
	fixTypes := map[string]string{
		"bash":      "remediation-script.sh",
		"ansible":   "remediation-playbook.yml",
		"blueprint": "remediation-blueprint.toml",
	}

	for fixType, outputFile := range fixTypes {
		outputPath := filepath.Join(providerDir, config.RemediationDir, outputFile)
		hclog.Default().Debug("Generating remedation file %s", outputPath)
		command := constructGenerateFixCommand(fixType, outputPath, profile, policyFile, datastream)
		_, err := executeCommand(ctx, command)
		if err != nil {
			return err
		}

	}
	return nil
}
