// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/complytime/complyctl/pkg/provider"
	"github.com/complytime/complytime-providers/cmd/opa-provider/config"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
	"github.com/complytime/complytime-providers/cmd/opa-provider/results"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
	"github.com/complytime/complytime-providers/cmd/opa-provider/targets"
	"github.com/complytime/complytime-providers/cmd/opa-provider/toolcheck"
)

var safeBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

var _ provider.Provider = (*ProviderServer)(nil)

// ServerOptions configures the ProviderServer dependencies. Zero-value fields
// receive sensible production defaults.
type ServerOptions struct {
	Loader       loader.DataLoader
	Runner       scan.CommandRunner
	ToolChecker  func() []string
	WorkspaceDir string
}

// ProviderServer implements the provider.Provider interface for the OPA provider.
type ProviderServer struct {
	opts ServerOptions
}

// New returns a new ProviderServer with the given options. Zero-value fields
// in opts are replaced with production defaults.
func New(opts ServerOptions) *ProviderServer {
	if opts.Runner == nil {
		opts.Runner = scan.ExecRunner{}
	}
	if opts.Loader == nil {
		opts.Loader = loader.NewRouter(opts.Runner)
	}
	if opts.ToolChecker == nil {
		opts.ToolChecker = toolcheck.CheckTools
	}
	if opts.WorkspaceDir == "" {
		opts.WorkspaceDir = provider.WorkspaceDir
	}
	return &ProviderServer{opts: opts}
}

// Describe returns the provider metadata and health status.
func (s *ProviderServer) Describe(
	_ context.Context, _ *provider.DescribeRequest,
) (*provider.DescribeResponse, error) {
	healthy := true
	var errMsg string

	missing := s.opts.ToolChecker()
	if len(missing) > 0 {
		healthy = false
		errMsg = toolcheck.FormatMissingToolsError(missing).Error()
	}

	return &provider.DescribeResponse{
		Healthy:                 healthy,
		Version:                 "0.1.0",
		ErrorMessage:            errMsg,
		RequiredTargetVariables: []string{loader.VarURL, loader.VarInputPath},
	}, nil
}

// Generate is a stub that returns success. The Generate phase is deferred.
func (s *ProviderServer) Generate(
	_ context.Context, _ *provider.GenerateRequest,
) (*provider.GenerateResponse, error) {
	return &provider.GenerateResponse{Success: true}, nil
}

// Scan evaluates configuration files against OPA policies using conftest.
func (s *ProviderServer) Scan(
	_ context.Context, req *provider.ScanRequest,
) (*provider.ScanResponse, error) {
	logger := hclog.Default()

	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets provided: at least one target is required")
	}

	missing := s.opts.ToolChecker()
	if len(missing) > 0 {
		logger.Warn("required tools missing", "tools", missing)
		return nil, toolcheck.FormatMissingToolsError(missing)
	}

	cfg := config.NewConfig(s.opts.WorkspaceDir)
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	bundleCache := map[string]string{}
	var allResults []*results.PerTargetResult
	var writeErrs []error

	for _, target := range req.Targets {
		targetResults, writeErr := s.processTarget(logger, target, cfg, bundleCache)
		if writeErr != nil {
			writeErrs = append(writeErrs, writeErr)
		}
		allResults = append(allResults, targetResults...)
	}

	resp := results.ToScanResponse(allResults)
	scanStatus := results.ScanStatusAssessment(allResults, errors.Join(writeErrs...))
	resp.Assessments = append(
		[]provider.AssessmentLog{scanStatus}, resp.Assessments...,
	)

	return resp, nil
}

func (s *ProviderServer) processTarget(
	logger hclog.Logger,
	target provider.Target,
	cfg *config.Config,
	bundleCache map[string]string,
) ([]*results.PerTargetResult, error) {
	bundleRef := target.Variables[loader.VarOPABundleRef]
	if bundleRef == "" {
		logger.Warn("missing opa_bundle_ref", "target", target.TargetID)
		return []*results.PerTargetResult{{
			Target: target.TargetID,
			Status: "error",
			Error:  "opa_bundle_ref variable is required but not set",
		}}, nil
	}

	repoURL := target.Variables[loader.VarURL]
	inputPath := target.Variables[loader.VarInputPath]
	branchesStr := target.Variables[loader.VarBranches]
	accessToken := target.Variables[loader.VarAccessToken]
	scanPath := target.Variables[loader.VarScanPath]

	if branchesStr == "" {
		branchesStr = "main"
	}

	if err := validateTargetVariables(
		repoURL, inputPath, branchesStr, scanPath, accessToken,
	); err != nil {
		return []*results.PerTargetResult{{
			Target: target.TargetID,
			Status: "error",
			Error:  err.Error(),
		}}, nil
	}

	policyDir, ok := bundleCache[bundleRef]
	if !ok {
		policyDir = cfg.PolicyDirForBundle(bundleRef)
		logger.Info("pulling policy bundle", "ref", bundleRef)
		if err := scan.PullBundle(bundleRef, policyDir, s.opts.Runner); err != nil {
			logger.Warn("bundle pull failed", "ref", bundleRef, "error", err)
			return []*results.PerTargetResult{{
				Target: target.TargetID,
				Status: "error",
				Error:  fmt.Sprintf("pulling policy bundle: %s", err),
			}}, nil
		}
		bundleCache[bundleRef] = policyDir
	}

	if repoURL != "" {
		return s.processRemoteBranches(
			logger, target, splitCSV(branchesStr), policyDir, cfg,
		)
	}

	return s.processLocalInput(logger, target, policyDir, cfg)
}

func (s *ProviderServer) processRemoteBranches(
	logger hclog.Logger,
	target provider.Target,
	branches []string,
	policyDir string,
	cfg *config.Config,
) ([]*results.PerTargetResult, error) {
	var targetResults []*results.PerTargetResult
	var writeErrs []error
	repoURL := target.Variables[loader.VarURL]

	for _, branch := range branches {
		branchTarget := provider.Target{
			TargetID:  target.TargetID,
			Variables: copyVars(target.Variables),
		}
		branchTarget.Variables[loader.VarBranch] = branch

		workDir := cfg.ReposDirPath()
		inputPath, err := s.opts.Loader.Load(branchTarget, workDir)
		if err != nil {
			logger.Warn("data load failed",
				"target", target.TargetID, "branch", branch, "error", err)
			errResult := &results.PerTargetResult{
				Target: targets.RepoDisplayName(repoURL),
				Branch: branch,
				Status: "error",
				Error:  err.Error(),
			}
			if writeErr := results.WritePerTargetResult(
				errResult, cfg.ResultsDirPath(),
			); writeErr != nil {
				writeErrs = append(writeErrs, writeErr)
			}
			targetResults = append(targetResults, errResult)
			continue
		}

		result, evalErr := s.evalAndParse(
			logger, inputPath, policyDir,
			targets.RepoDisplayName(repoURL), branch, cfg,
		)
		if result == nil && evalErr != nil {
			logger.Warn("eval failed",
				"target", target.TargetID, "branch", branch, "error", evalErr)
			errResult := &results.PerTargetResult{
				Target: targets.RepoDisplayName(repoURL),
				Branch: branch,
				Status: "error",
				Error:  evalErr.Error(),
			}
			if writeErr := results.WritePerTargetResult(
				errResult, cfg.ResultsDirPath(),
			); writeErr != nil {
				writeErrs = append(writeErrs, writeErr)
			}
			targetResults = append(targetResults, errResult)
			continue
		}
		if evalErr != nil {
			writeErrs = append(writeErrs, evalErr)
		}
		targetResults = append(targetResults, result)
	}

	return targetResults, errors.Join(writeErrs...)
}

func (s *ProviderServer) processLocalInput(
	logger hclog.Logger,
	target provider.Target,
	policyDir string,
	cfg *config.Config,
) ([]*results.PerTargetResult, error) {
	inputPath, err := s.opts.Loader.Load(target, "")
	if err != nil {
		errResult := &results.PerTargetResult{
			Target: target.TargetID,
			Status: "error",
			Error:  err.Error(),
		}
		if writeErr := results.WritePerTargetResult(
			errResult, cfg.ResultsDirPath(),
		); writeErr != nil {
			return []*results.PerTargetResult{errResult}, writeErr
		}
		return []*results.PerTargetResult{errResult}, nil
	}

	result, evalErr := s.evalAndParse(
		logger, inputPath, policyDir, inputPath, "", cfg,
	)
	if result == nil && evalErr != nil {
		return []*results.PerTargetResult{{
			Target: target.TargetID,
			Status: "error",
			Error:  evalErr.Error(),
		}}, nil
	}
	if evalErr != nil {
		return []*results.PerTargetResult{result}, evalErr
	}

	return []*results.PerTargetResult{result}, nil
}

func (s *ProviderServer) evalAndParse(
	logger hclog.Logger,
	inputPath, policyDir, displayName, branch string,
	cfg *config.Config,
) (*results.PerTargetResult, error) {
	logger.Info("evaluating policies", "path", inputPath)
	raw, err := scan.EvalPolicy(inputPath, policyDir, s.opts.Runner)
	if err != nil {
		return nil, fmt.Errorf("evaluating policies: %w", err)
	}

	result, err := results.ParseConftestOutput(raw, displayName, branch)
	if err != nil {
		return nil, fmt.Errorf("parsing conftest output: %w", err)
	}

	if writeErr := results.WritePerTargetResult(
		result, cfg.ResultsDirPath(),
	); writeErr != nil {
		return result, writeErr
	}

	return result, nil
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

// validateTargetVariables performs defense-in-depth validation of target
// variables.
func validateTargetVariables(
	repoURL, inputPath, branches, scanPath, accessToken string,
) error {
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
				return fmt.Errorf(
					"branch name contains path traversal: %q", b,
				)
			}
			if !safeBranchPattern.MatchString(b) {
				return fmt.Errorf(
					"branch name contains invalid characters: %q", b,
				)
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

func copyVars(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
