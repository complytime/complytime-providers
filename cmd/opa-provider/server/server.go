// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	"github.com/complytime/complytime-providers/internal/evidence"
	"github.com/complytime/complytime-providers/internal/genindex"
	"github.com/complytime/complytime-providers/internal/version"
)

var safeBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

const conftestNoFilesFound = "Error: running test: parse files: no files found"

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
	opts                 ServerOptions
	detectConftestConfig func(string) (string, error)
	localTraversal       localTraversal
	identifyFile         identityFunc
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
	return &ProviderServer{
		opts: opts, detectConftestConfig: detectConftestConfig,
		localTraversal: localTraversal{readDir: os.ReadDir, lstat: os.Lstat},
		identifyFile:   fileIdentityOf,
	}
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
		Healthy:                      healthy,
		Version:                      version.Version(),
		ErrorMessage:                 errMsg,
		OptionalTargetVariableGroups: []string{loader.VarURL + "|" + loader.VarInputPath},
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

	contentKey := genindex.ContentKey(ids, policyDir)
	generatedDir := cfg.GeneratedDirForKey(contentKey)
	if err := os.MkdirAll(generatedDir, genindex.ContentDirectoryMode); err != nil {
		return nil, fmt.Errorf("creating content-generated directory: %w", err)
	}
	if err := generate.WriteScanConfig(generatedDir, ids, reverseMap, policyDir); err != nil {
		return nil, fmt.Errorf("writing scan config: %w", err)
	}
	targetKeyInput := req.TargetVariables[loader.VarURL]
	if inputPath := req.TargetVariables[loader.VarInputPath]; inputPath != "" {
		targetKeyInput = inputPath
	}
	if err := genindex.UpdateIndex(
		cfg.GeneratedDirPath(), genindex.TargetKey(targetKeyInput), contentKey,
	); err != nil {
		return nil, fmt.Errorf("updating generate index: %w", err)
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
// the PolicyDir from the scan config (set by Generate) if available. Falls
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
	if scanCfg != nil && scanCfg.PolicyDir != "" {
		logger.Info("using policy dir from scan config",
			"policy_dir", scanCfg.PolicyDir)
		return scanCfg.PolicyDir, nil
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
	ctx context.Context, req *provider.ScanRequest,
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
		for _, tool := range missing {
			if tool == "conftest" {
				response := &provider.ScanResponse{}
				for _, target := range req.Targets {
					response.Errors = append(response.Errors, fmt.Sprintf(
						"%s: install conftest and make it available on PATH; %v",
						target.TargetID, toolcheck.FormatMissingToolsError(missing)))
				}
				return response, nil
			}
		}
		return nil, toolcheck.FormatMissingToolsError(missing)
	}

	cfg := config.NewConfig(s.opts.WorkspaceDir)
	if err := cfg.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	index, indexErr := genindex.ReadIndex(cfg.GeneratedDirPath())
	if indexErr != nil && !errors.Is(indexErr, os.ErrNotExist) {
		return nil, fmt.Errorf(
			"reading generate index for target %q: %w", req.Targets[0].TargetID, indexErr,
		)
	}
	hasIndex := indexErr == nil
	var legacyScanConfig *generate.ScanConfig
	if !hasIndex {
		logger.Warn("generate index absent; using legacy scan configuration fallback")
		legacyScanConfig, indexErr = generate.ReadScanConfig(cfg.GeneratedDirPath())
		if indexErr != nil {
			logger.Warn("no legacy scan config found; run Generate before Scan", "error", indexErr)
		}
	}
	scanConfigCache := make(map[string]*generate.ScanConfig)

	bundleCache := map[string]string{}
	var allResults []*results.PerTargetResult
	var writeErrs []error
	hostRemark := evidence.HostRemark()

	for _, target := range req.Targets {
		scanCfg, err := resolveTargetScanConfig(
			target, cfg, index, hasIndex, legacyScanConfig, scanConfigCache,
		)
		if err != nil {
			return nil, err
		}
		targetResults, writeErr := s.processTarget(
			ctx, logger, target, cfg, bundleCache, scanCfg,
		)
		if writeErr != nil {
			writeErrs = append(writeErrs, writeErr)
		}
		for _, tr := range targetResults {
			tr.Remarks = hostRemark
			if scanCfg != nil {
				tr.ReverseMapping = scanCfg.ReverseMapping
			}
		}
		allResults = append(allResults, targetResults...)
	}

	resp := results.ToScanResponse(allResults, nil)

	if aggregatedErr := errors.Join(writeErrs...); aggregatedErr != nil {
		resp.Errors = append(resp.Errors, aggregatedErr.Error())
	}

	return resp, nil
}

func resolveTargetScanConfig(
	target provider.Target,
	cfg *config.Config,
	index genindex.GenerateIndex,
	hasIndex bool,
	legacyScanConfig *generate.ScanConfig,
	cache map[string]*generate.ScanConfig,
) (*generate.ScanConfig, error) {
	if !hasIndex {
		return legacyScanConfig, nil
	}

	targetKeyInput := target.Variables[loader.VarURL]
	if inputPath := target.Variables[loader.VarInputPath]; inputPath != "" {
		targetKeyInput = inputPath
	}
	scanCfg, _, err := genindex.ResolveConfig(
		genindex.TargetKey(targetKeyInput), index, cfg.GeneratedDirPath(), cache, generate.ReadScanConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("resolving generated config for target %q: %w", target.TargetID, err)
	}
	return scanCfg, nil
}

func targetError(target, targetURL, branch string, err error) *results.PerTargetResult {
	return &results.PerTargetResult{
		Target: target, TargetURL: targetURL, Branch: branch, Status: "error", Error: err.Error(),
	}
}

func targetErrors(target string, err error) []*results.PerTargetResult {
	return []*results.PerTargetResult{targetError(target, "", "", err)}
}

func (s *ProviderServer) processTarget(
	ctx context.Context,
	logger hclog.Logger,
	target provider.Target,
	cfg *config.Config,
	bundleCache map[string]string,
	scanCfg *generate.ScanConfig,
) ([]*results.PerTargetResult, error) {
	if err := preflightCancellation(ctx); err != nil {
		return targetErrors(target.TargetID, err), nil
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
		return targetErrors(target.TargetID, err), nil
	}

	if repoURL == "" {
		return s.processLocalInput(ctx, logger, target, cfg, bundleCache, scanCfg)
	}

	// Resolve policy directory: use scan config's PolicyDir (set by
	// Generate from complypack or opa_bundle_ref), then fall back to
	// pulling via opa_bundle_ref if scan config is unavailable.
	policyDir, err := s.resolveScanPolicyDir(
		logger, target, cfg, bundleCache, scanCfg,
	)
	if err != nil {
		return targetErrors(target.TargetID, err), nil
	}

	return s.processRemoteBranches(
		ctx, logger, target, splitCSV(branchesStr), policyDir, cfg, scanCfg,
	)
}

func (s *ProviderServer) processRemoteBranches(
	ctx context.Context,
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
		if err := preflightCancellation(ctx); err != nil {
			return rejectTargetResults(targetResults, targetError(target.TargetID, repoURL, branch, err)),
				errors.Join(writeErrs...)
		}
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
			errResult := targetError(targets.RepoDisplayName(repoURL), repoURL, branch, err)
			if writeErr := results.WritePerTargetResult(
				errResult, cfg.ResultsDirPath(),
			); writeErr != nil {
				writeErrs = append(writeErrs, writeErr)
			}
			targetResults = append(targetResults, errResult)
			continue
		}

		if err := s.inspectRemoteInput(ctx, inputPath, target.Variables, cfg.WorkspaceDir); err != nil {
			errResult := targetError(target.TargetID, repoURL, branch, err)
			// A rejected branch invalidates the configured target as a whole.
			return rejectTargetResults(targetResults, errResult), errors.Join(writeErrs...)
		}
		if err := preflightCancellation(ctx); err != nil {
			return rejectTargetResults(targetResults, targetError(target.TargetID, repoURL, branch, err)),
				errors.Join(writeErrs...)
		}
		result, evalErr := s.evalAndParse(
			logger, inputPath, policyDir,
			targets.RepoDisplayName(repoURL), branch, cfg, scanCfg, "", nil,
		)
		if result == nil && evalErr != nil {
			logger.Warn("eval failed",
				"target", target.TargetID, "branch", branch, "error", evalErr)
			errResult := targetError(target.TargetID, repoURL, branch, evalErr)
			if writeErr := results.WritePerTargetResult(
				errResult, cfg.ResultsDirPath(),
			); writeErr != nil {
				writeErrs = append(writeErrs, writeErr)
			}
			var selectionErr *inputSelectionError
			if errors.As(evalErr, &selectionErr) {
				return rejectTargetResults(targetResults, errResult), errors.Join(writeErrs...)
			}
			targetResults = append(targetResults, errResult)
			continue
		}
		if result != nil {
			result.TargetURL = repoURL
		}
		if evalErr != nil {
			writeErrs = append(writeErrs, evalErr)
		}
		targetResults = append(targetResults, result)
	}

	return targetResults, errors.Join(writeErrs...)
}

func (s *ProviderServer) processLocalInput(
	ctx context.Context,
	logger hclog.Logger,
	target provider.Target,
	cfg *config.Config,
	bundleCache map[string]string,
	scanCfg *generate.ScanConfig,
) ([]*results.PerTargetResult, error) {
	inputPath, err := s.opts.Loader.Load(target, "")
	if err != nil {
		errResult := targetError(target.TargetID, "", "", fmt.Errorf(
			"cannot load local input; check the input path and filesystem access: %w", err))
		if writeErr := results.WritePerTargetResult(
			errResult, cfg.ResultsDirPath(),
		); writeErr != nil {
			return []*results.PerTargetResult{errResult}, writeErr
		}
		return []*results.PerTargetResult{errResult}, nil
	}

	if err := validateLocalWorkspaceTarget(inputPath, target.Variables, cfg.WorkspaceDir); err != nil {
		return targetErrors(target.TargetID, err), nil
	}

	ignore, environment, overlapping, err := s.localWorkspaceIgnore(inputPath, target.Variables, cfg.WorkspaceDir)
	if err != nil {
		return targetErrors(target.TargetID, fmt.Errorf(
			"cannot configure workspace exclusion; check input and workspace paths: %w", err)), nil
	}
	boundaries, err := workspaceBoundaries(target.Variables, cfg.WorkspaceDir)
	if err != nil {
		err = fmt.Errorf("cannot establish workspace boundaries; check workspace paths: %w", err)
	} else {
		var identities map[fileIdentity]string
		identities, err = workspaceIdentities(ctx, boundaries, "", s.localTraversal, s.identifyFile)
		if err == nil {
			err = inspectInput(ctx, inputPath, boundaries, "", ignore, s.localTraversal, identities, s.identifyFile)
		}
	}
	if err != nil {
		return targetErrors(target.TargetID, err), nil
	}
	for _, boundary := range overlapping {
		logger.Info("excluding workspace from local input", "workspace", boundary)
	}

	policyDir, err := s.resolveScanPolicyDir(logger, target, cfg, bundleCache, scanCfg)
	if err != nil {
		return targetErrors(target.TargetID, err), nil
	}
	if err := preflightCancellation(ctx); err != nil {
		return targetErrors(target.TargetID, err), nil
	}

	result, evalErr := s.evalAndParse(
		logger, inputPath, policyDir, inputPath, "", cfg, scanCfg, ignore, environment,
	)
	if result == nil && evalErr != nil {
		return targetErrors(target.TargetID, fmt.Errorf(
			"local evaluation failed; check input paths, filesystem access, and conftest output: %w", evalErr)), nil
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
	ignore string,
	environment []string,
) (*results.PerTargetResult, error) {
	logger.Info("evaluating policies", "path", inputPath)

	if scanCfg == nil || scanCfg.IDs == nil {
		return nil, fmt.Errorf(
			"scan config missing or has no requirement IDs; run Generate first with a bundle containing %s",
			generate.MappingFileName,
		)
	}
	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("checking input %q; verify the input path and access: %w", inputPath, err)
	}

	raw, err := scan.EvalPolicyWithNamespacesAndIgnore(
		inputPath, policyDir, scanCfg.IDs, s.opts.Runner, ignore, environment,
	)
	if err != nil {
		if inputInfo.IsDir() && containsConftestNoFilesFound(raw) {
			return nil, &inputSelectionError{err: fmt.Errorf(
				"no eligible input could be confirmed for directory %q; "+
					"add a supported input or adjust ignore settings: %w", inputPath, err)}
		}
		return nil, fmt.Errorf("evaluating policies; check conftest output and policy configuration: %w", err)
	}

	result, err := results.ParseConftestOutput(raw, displayName, branch)
	if err != nil {
		err = fmt.Errorf("parsing conftest output; check conftest output and policy configuration: %w", err)
		if inputInfo.IsDir() {
			return nil, &inputSelectionError{err: err}
		}
		return nil, err
	}
	if inputInfo.IsDir() && result.EvaluatedFiles == 0 {
		return nil, &inputSelectionError{err: fmt.Errorf(
			"no eligible input was confirmed for directory %q; "+
				"add a supported input or adjust ignore settings", inputPath)}
	}
	result.InputPath = inputPath

	// Compute SHA256 digest for regular-file inputs. Directories
	// are left without a digest per the evidence-source spec.
	// Stat errors are propagated: if the input was accessible
	// enough for conftest to evaluate, a subsequent stat failure
	// is unexpected and must not be silently ignored.
	//
	// Known limitation: the digest is computed after conftest
	// evaluation. If the file is concurrently modified between
	// conftest reading it and this hash, the digest may not match
	// the bytes conftest evaluated. This is an inherent constraint
	// of any external-tool pipeline where the tool reads by path
	// and does not expose the bytes it consumed. In practice,
	// compliance scan inputs are not concurrently modified.
	info, statErr := os.Stat(inputPath)
	if statErr != nil {
		return nil, fmt.Errorf(
			"stat input for digest %s: %w", inputPath, statErr,
		)
	}
	if !info.IsDir() {
		digest, dErr := evidence.FileDigest(inputPath)
		if dErr != nil {
			return nil, fmt.Errorf(
				"computing digest for input %s: %w", inputPath, dErr,
			)
		}
		result.InputDigest = digest
	}

	if writeErr := results.WritePerTargetResult(
		result, cfg.ResultsDirPath(),
	); writeErr != nil {
		return result, writeErr
	}

	return result, nil
}

// Conftest v0.68.2 reports an empty effective input set as this exact line.
// Matching the line, rather than arbitrary error text, avoids misclassifying
// policy or filesystem errors that merely mention the same words.
func containsConftestNoFilesFound(output []byte) bool {
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == conftestNoFilesFound {
			return true
		}
	}
	return false
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
