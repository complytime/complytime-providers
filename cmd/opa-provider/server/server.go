// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/results"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
	"github.com/complytime/complytime-providers/cmd/opa-provider/targets"
	"github.com/complytime/complytime-providers/cmd/opa-provider/toolcheck"
)

// ScanRunner is used by Scan to execute scan commands.
// It defaults to scan.ExecRunner{} and can be overridden for testing.
var ScanRunner scan.CommandRunner = scan.ExecRunner{}

// SkipToolCheck disables tool presence validation. Used in tests.
var SkipToolCheck bool

// TestWorkspaceDir overrides provider.WorkspaceDir for testing.
var TestWorkspaceDir string

var safeBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

var _ provider.Provider = (*ProviderServer)(nil)

// ProviderServer implements the provider.Provider interface for the OPA provider.
type ProviderServer struct{}

// New returns a new ProviderServer.
func New() *ProviderServer {
	return &ProviderServer{}
}

// Describe returns the provider metadata and health status.
func (s *ProviderServer) Describe(_ context.Context, _ *provider.DescribeRequest) (*provider.DescribeResponse, error) {
	healthy := true
	var errMsg string

	if !SkipToolCheck {
		missing, _ := toolcheck.CheckTools()
		if len(missing) > 0 {
			healthy = false
			errMsg = toolcheck.FormatMissingToolsError(missing).Error()
		}
	}

	return &provider.DescribeResponse{
		Healthy:                 healthy,
		Version:                 "0.1.0",
		ErrorMessage:            errMsg,
		RequiredTargetVariables: []string{"url", "input_path"},
	}, nil
}

// Generate is a stub that returns success. The Generate phase is deferred.
func (s *ProviderServer) Generate(_ context.Context, _ *provider.GenerateRequest) (*provider.GenerateResponse, error) {
	return &provider.GenerateResponse{Success: true}, nil
}

// Scan evaluates configuration files against OPA policies using conftest.
func (s *ProviderServer) Scan(_ context.Context, req *provider.ScanRequest) (*provider.ScanResponse, error) {
	logger := hclog.Default()

	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets provided: at least one target is required")
	}

	if err := checkRequiredTools(logger); err != nil {
		return nil, err
	}

	// Extract opa_bundle_ref from first target's variables
	bundleRef := extractBundleRef(req.Targets)
	if bundleRef == "" {
		return nil, fmt.Errorf("opa_bundle_ref variable is required but not set in any target")
	}

	workspaceDir := provider.WorkspaceDir
	if TestWorkspaceDir != "" {
		workspaceDir = TestWorkspaceDir
	}
	cfg := config.NewConfig(workspaceDir)
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	// Pull policy bundle
	logger.Info("pulling policy bundle", "ref", bundleRef)
	if err := scan.PullBundle(bundleRef, cfg.PolicyDirPath(), ScanRunner); err != nil {
		return nil, fmt.Errorf("pulling policy bundle: %w", err)
	}

	var allResults []*results.PerTargetResult

	for _, target := range req.Targets {
		targetResults, err := s.processTarget(logger, target, cfg)
		if err != nil {
			logger.Warn("target processing failed", "target", target.TargetID, "error", err)
			errResult := &results.PerTargetResult{
				Target: target.TargetID,
				Status: "error",
				Error:  err.Error(),
			}
			allResults = append(allResults, errResult)
			continue
		}
		allResults = append(allResults, targetResults...)
	}

	return results.ToScanResponse(allResults), nil
}

func (s *ProviderServer) processTarget(
	logger hclog.Logger,
	target provider.Target,
	cfg *config.Config,
) ([]*results.PerTargetResult, error) {
	repoURL := target.Variables["url"]
	inputPath := target.Variables["input_path"]
	branchesStr := target.Variables["branches"]
	accessToken := target.Variables["access_token"]
	scanPath := target.Variables["scan_path"]

	if branchesStr == "" {
		branchesStr = "main"
	}

	if err := validateTargetVariables(repoURL, inputPath, branchesStr, scanPath, accessToken); err != nil {
		return nil, err
	}

	if repoURL != "" {
		return s.processRemoteTarget(logger, repoURL, branchesStr, accessToken, scanPath, cfg)
	}

	return s.processLocalTarget(logger, inputPath, cfg)
}

func (s *ProviderServer) processRemoteTarget(
	logger hclog.Logger,
	repoURL, branchesStr, accessToken, scanPath string,
	cfg *config.Config,
) ([]*results.PerTargetResult, error) {
	branches := splitCSV(branchesStr)
	var targetResults []*results.PerTargetResult

	for _, branch := range branches {
		result, err := s.scanRemoteBranch(logger, repoURL, branch, accessToken, scanPath, cfg)
		if err != nil {
			logger.Warn("branch scan failed", "url", repoURL, "branch", branch, "error", err)
			errResult := &results.PerTargetResult{
				Target: targets.RepoDisplayName(repoURL),
				Branch: branch,
				Status: "error",
				Error:  err.Error(),
			}
			targetResults = append(targetResults, errResult)
			if writeErr := results.WritePerTargetResult(errResult, cfg.ResultsDirPath()); writeErr != nil {
				logger.Error("failed to write error result", "error", writeErr)
			}
			continue
		}
		targetResults = append(targetResults, result)
	}

	return targetResults, nil
}

func (s *ProviderServer) scanRemoteBranch(
	logger hclog.Logger,
	repoURL, branch, accessToken, scanPath string,
	cfg *config.Config,
) (*results.PerTargetResult, error) {
	cloneDir := filepath.Join(cfg.ReposDirPath(), targets.SanitizeRepoURL(repoURL)+"-"+branch)

	logger.Info("cloning repository", "url", repoURL, "branch", branch)
	if err := scan.CloneRepository(repoURL, branch, cloneDir, accessToken, ScanRunner); err != nil {
		return nil, fmt.Errorf("cloning repository: %w", err)
	}

	scanDir := cloneDir
	if scanPath != "" {
		scanDir = filepath.Join(cloneDir, scanPath)
	}

	logger.Info("evaluating policies", "path", scanDir)
	raw, err := scan.EvalPolicy(scanDir, cfg.PolicyDirPath(), ScanRunner)
	if err != nil {
		return nil, fmt.Errorf("evaluating policies: %w", err)
	}

	displayName := targets.RepoDisplayName(repoURL)
	result, err := results.ParseConftestOutput(raw, displayName, branch)
	if err != nil {
		return nil, fmt.Errorf("parsing conftest output: %w", err)
	}

	if writeErr := results.WritePerTargetResult(result, cfg.ResultsDirPath()); writeErr != nil {
		logger.Error("failed to write result", "error", writeErr)
	}

	return result, nil
}

func (s *ProviderServer) processLocalTarget(
	logger hclog.Logger,
	inputPath string,
	cfg *config.Config,
) ([]*results.PerTargetResult, error) {
	if err := targets.ValidateInputPath(inputPath); err != nil {
		return nil, fmt.Errorf("validating input path: %w", err)
	}

	logger.Info("evaluating policies", "path", inputPath)
	raw, err := scan.EvalPolicy(inputPath, cfg.PolicyDirPath(), ScanRunner)
	if err != nil {
		return nil, fmt.Errorf("evaluating policies: %w", err)
	}

	result, err := results.ParseConftestOutput(raw, inputPath, "")
	if err != nil {
		return nil, fmt.Errorf("parsing conftest output: %w", err)
	}

	if writeErr := results.WritePerTargetResult(result, cfg.ResultsDirPath()); writeErr != nil {
		logger.Error("failed to write result", "error", writeErr)
	}

	return []*results.PerTargetResult{result}, nil
}

// splitCSV splits a comma-separated string into trimmed, non-empty parts.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// validateTargetVariables performs defense-in-depth validation of target variables.
func validateTargetVariables(repoURL, inputPath, branches, scanPath, accessToken string) error {
	if repoURL != "" && inputPath != "" {
		return fmt.Errorf("specify either url or input_path, not both")
	}
	if repoURL == "" && inputPath == "" {
		return fmt.Errorf("url or input_path is required")
	}

	if repoURL != "" {
		if !strings.HasPrefix(repoURL, "https://") {
			return fmt.Errorf("url %q must use HTTPS scheme", repoURL)
		}
	}

	if branches != "" {
		for _, b := range strings.Split(branches, ",") {
			b = strings.TrimSpace(b)
			if b == "" {
				continue
			}
			if strings.Contains(b, "..") {
				return fmt.Errorf("branch name contains path traversal: %q", b)
			}
			if !safeBranchPattern.MatchString(b) {
				return fmt.Errorf("branch name contains invalid characters: %q", b)
			}
		}
	}

	if scanPath != "" && strings.Contains(scanPath, "..") {
		return fmt.Errorf("scan_path contains path traversal: %q", scanPath)
	}

	if accessToken != "" && strings.ContainsAny(accessToken, "\n\r\x00") {
		return fmt.Errorf("access_token contains invalid characters")
	}

	return nil
}

func extractBundleRef(targetList []provider.Target) string {
	for _, t := range targetList {
		if ref, ok := t.Variables["opa_bundle_ref"]; ok && ref != "" {
			return ref
		}
	}
	return ""
}

func checkRequiredTools(logger hclog.Logger) error {
	if SkipToolCheck {
		return nil
	}
	missing, err := toolcheck.CheckTools()
	if err != nil {
		return fmt.Errorf("checking required tools: %w", err)
	}
	if len(missing) > 0 {
		logger.Warn("required tools missing", "tools", missing)
		return toolcheck.FormatMissingToolsError(missing)
	}
	return nil
}
