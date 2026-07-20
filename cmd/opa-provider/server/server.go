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
	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
	"github.com/complytime/complytime-providers/cmd/opa-provider/results"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
	"github.com/complytime/complytime-providers/cmd/opa-provider/targets"
	"github.com/complytime/complytime-providers/cmd/opa-provider/toolcheck"
	"github.com/complytime/complytime-providers/internal/archive"
	"github.com/complytime/complytime-providers/internal/version"
)

var safeBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

var _ provider.Provider = (*ProviderServer)(nil)

// ServerOptions configures the ProviderServer dependencies. Zero-value fields
// receive sensible production defaults.
type ServerOptions struct {
	Loader       loader.DataLoader
	Runner       scan.CommandRunner
	ToolChecker  func() ([]string, error)
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

	missing, err := s.opts.ToolChecker()
	if err != nil {
		return nil, fmt.Errorf("checking tools: %w", err)
	}
	if len(missing) > 0 {
		healthy = false
		errMsg = toolcheck.FormatMissingToolsError(missing).Error()
	}

	return &provider.DescribeResponse{
		Healthy:                 healthy,
		Version:                 version.Version(),
		ErrorMessage:            errMsg,
		RequiredTargetVariables: []string{loader.VarURL, loader.VarInputPath},
	}, nil
}

// Generate reads the Gemara requirement IDs from the assessment configurations,
// resolves the policy directory (preferring ComplypackContentPath from a cached
// complypack over opa_bundle_ref + conftest pull), loads the required mapping
// file, matches requirement IDs to Rego namespaces, and writes a
// scan-config.json for Scan to consume. Returns {Success: false} if the
// mapping file is missing or invalid.
func (s *ProviderServer) Generate(
	_ context.Context, req *provider.GenerateRequest,
) (*provider.GenerateResponse, error) {
	logger := hclog.Default()

	if len(req.Configuration) == 0 {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: "no assessment configurations provided",
		}, nil
	}

	missing, err := s.opts.ToolChecker()
	if err != nil {
		return nil, fmt.Errorf("checking tools: %w", err)
	}
	if len(missing) > 0 {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: toolcheck.FormatMissingToolsError(missing).Error(),
		}, nil
	}

	cfg := config.NewConfig(s.opts.WorkspaceDir)
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	// Resolve policy directory: prefer complypack content path (delivered
	// by complyctl via GenerateRequest) over opa_bundle_ref + conftest pull.
	policyDir, err := s.resolvePolicyDir(logger, req, cfg)
	if err != nil {
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	mapping, err := generate.LoadMapping(policyDir)
	if err != nil {
		if errors.Is(err, generate.ErrMappingNotFound) {
			return &provider.GenerateResponse{
				Success: false,
				ErrorMessage: fmt.Sprintf(
					"OCI bundle does not contain %s; "+
						"policy bundles must include a mapping file "+
						"to enable requirement-scoped evaluation",
					generate.MappingFileName,
				),
			}, nil
		}
		return &provider.GenerateResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("invalid mapping file: %s", err),
		}, nil
	}

	ids, reverseMap, warnings := generate.MatchRequirements(
		req.Configuration, mapping,
	)
	for _, w := range warnings {
		logger.Warn(w)
	}

	if err := generate.WriteScanConfig(
		cfg.GeneratedDirPath(), ids, reverseMap, policyDir,
	); err != nil {
		return nil, fmt.Errorf("writing scan config: %w", err)
	}

	logger.Info("generate complete",
		"matched_ids", len(ids),
		"warnings", len(warnings))

	return &provider.GenerateResponse{Success: true}, nil
}

// resolvePolicyDir determines the policy directory for Generate. It checks
// ComplypackContentPath first (delivered by complyctl from a cached complypack),
// then falls back to opa_bundle_ref + conftest pull. Returns an error if
// neither source is available.
//
// When ComplypackContentPath points to a tar.gz archive (the default
// format produced by complyctl's complypack cache), it is extracted
// to a sibling directory so downstream code can read files directly.
func (s *ProviderServer) resolvePolicyDir(
	logger hclog.Logger,
	req *provider.GenerateRequest,
	cfg *config.Config,
) (string, error) {
	if req.ComplypackContentPath != "" {
		contentPath := req.ComplypackContentPath
		logger.Info("using complypack content path for generate",
			"complypack_content_path", contentPath)

		resolved, err := archive.ResolveComplypackPath(contentPath)
		if err != nil {
			return "", fmt.Errorf(
				"resolving complypack content path: %w", err)
		}
		return resolved, nil
	}

	vars := mergeVariables(req.GlobalVariables, req.TargetVariables)
	bundleRef := vars[loader.VarOPABundleRef]
	if bundleRef == "" {
		return "", fmt.Errorf(
			"either a complypack or opa_bundle_ref variable is required")
	}

	policyDir := cfg.PolicyDirForBundle(bundleRef)
	logger.Info("pulling policy bundle for generate", "ref", bundleRef)
	if err := scan.PullBundle(bundleRef, policyDir, s.opts.Runner); err != nil {
		return "", fmt.Errorf("pulling policy bundle: %s", err)
	}
	return policyDir, nil
}

// resolveScanPolicyDir determines the policy directory for Scan. It uses
// the BundleDir from the scan config (set by Generate) if available. Falls
// back to pulling via opa_bundle_ref when no scan config exists.
func (s *ProviderServer) resolveScanPolicyDir(
	logger hclog.Logger,
	target provider.Target,
	cfg *config.Config,
	bundleCache map[string]string,
	scanCfg *generate.ScanConfig,
) (string, error) {
	// Prefer the directory from Generate's scan config (works for both
	// complypack and opa_bundle_ref paths).
	if scanCfg != nil && scanCfg.BundleDir != "" {
		logger.Info("using policy dir from scan config",
			"bundle_dir", scanCfg.BundleDir)
		return scanCfg.BundleDir, nil
	}

	// Fall back to pulling via opa_bundle_ref.
	bundleRef := target.Variables[loader.VarOPABundleRef]
	if bundleRef == "" {
		return "", fmt.Errorf(
			"either run Generate first or set opa_bundle_ref variable")
	}

	policyDir, ok := bundleCache[bundleRef]
	if ok {
		return policyDir, nil
	}

	policyDir = cfg.PolicyDirForBundle(bundleRef)
	logger.Info("pulling policy bundle", "ref", bundleRef)
	if err := scan.PullBundle(bundleRef, policyDir, s.opts.Runner); err != nil {
		return "", fmt.Errorf("pulling policy bundle: %s", err)
	}
	bundleCache[bundleRef] = policyDir
	return policyDir, nil
}

// mergeVariables merges global and target variables. Target variables
// override global variables with the same key.
func mergeVariables(global, target map[string]string) map[string]string {
	merged := make(map[string]string, len(global)+len(target))
	for k, v := range global {
		merged[k] = v
	}
	for k, v := range target {
		merged[k] = v
	}
	return merged
}

// Scan evaluates configuration files against OPA policies using conftest.
func (s *ProviderServer) Scan(
	_ context.Context, req *provider.ScanRequest,
) (*provider.ScanResponse, error) {
	logger := hclog.Default()

	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets provided: at least one target is required")
	}

	missing, err := s.opts.ToolChecker()
	if err != nil {
		return nil, fmt.Errorf("checking tools: %w", err)
	}
	if len(missing) > 0 {
		logger.Warn("required tools missing", "tools", missing)
		return nil, toolcheck.FormatMissingToolsError(missing)
	}

	cfg := config.NewConfig(s.opts.WorkspaceDir)
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	// Read scan config written by Generate.
	scanCfg, scanCfgErr := generate.ReadScanConfig(cfg.GeneratedDirPath())
	if scanCfgErr != nil {
		logger.Warn("no scan config found; run Generate before Scan", "error", scanCfgErr)
	}

	bundleCache := map[string]string{}
	var allResults []*results.PerTargetResult
	var writeErrs []error

	for _, target := range req.Targets {
		targetResults, writeErr := s.processTarget(
			logger, target, cfg, bundleCache, scanCfg,
		)
		if writeErr != nil {
			writeErrs = append(writeErrs, writeErr)
		}
		allResults = append(allResults, targetResults...)
	}

	var reverseMap map[string]string
	if scanCfg != nil {
		reverseMap = scanCfg.ReverseMapping
	}
	resp := results.ToScanResponse(allResults, reverseMap)

	if aggregatedErr := errors.Join(writeErrs...); aggregatedErr != nil {
		resp.Errors = append(resp.Errors, aggregatedErr.Error())
	}

	return resp, nil
}

func (s *ProviderServer) processTarget(
	logger hclog.Logger,
	target provider.Target,
	cfg *config.Config,
	bundleCache map[string]string,
	scanCfg *generate.ScanConfig,
) ([]*results.PerTargetResult, error) {
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

	// Resolve policy directory: use scan config's BundleDir (set by
	// Generate from complypack or opa_bundle_ref), then fall back to
	// pulling via opa_bundle_ref if scan config is unavailable.
	policyDir, err := s.resolveScanPolicyDir(
		logger, target, cfg, bundleCache, scanCfg,
	)
	if err != nil {
		return []*results.PerTargetResult{{
			Target: target.TargetID,
			Status: "error",
			Error:  err.Error(),
		}}, nil
	}

	if repoURL != "" {
		return s.processRemoteBranches(
			logger, target, splitCSV(branchesStr), policyDir, cfg, scanCfg,
		)
	}

	return s.processLocalInput(logger, target, policyDir, cfg, scanCfg)
}

func (s *ProviderServer) processRemoteBranches(
	logger hclog.Logger,
	target provider.Target,
	branches []string,
	policyDir string,
	cfg *config.Config,
	scanCfg *generate.ScanConfig,
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
			targets.RepoDisplayName(repoURL), branch, cfg, scanCfg,
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
	scanCfg *generate.ScanConfig,
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
		logger, inputPath, policyDir, inputPath, "", cfg, scanCfg,
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
	scanCfg *generate.ScanConfig,
) (*results.PerTargetResult, error) {
	logger.Info("evaluating policies", "path", inputPath)

	if scanCfg == nil || scanCfg.IDs == nil {
		return nil, fmt.Errorf(
			"scan config missing or has no requirement IDs; run Generate first with a bundle containing %s",
			generate.MappingFileName,
		)
	}

	raw, err := scan.EvalPolicyWithNamespaces(
		inputPath, policyDir, scanCfg.IDs, s.opts.Runner,
	)
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
