// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/complytime/complyctl/pkg/provider"

	"github.com/complytime/complytime-providers/cmd/opa-provider/generate"
	"github.com/complytime/complytime-providers/cmd/opa-provider/loader"
	"github.com/complytime/complytime-providers/cmd/opa-provider/scan"
	"github.com/complytime/complytime-providers/internal/evidence"
)

// inWorkspace returns request-local dependencies. It never changes process CWD or
// remembers a preceding RPC's workspace. Custom artifact boundaries remain protected.
func (s *ProviderServer) inWorkspace(root string) *ProviderServer {
	scoped := *s
	if runner, ok := scoped.opts.Runner.(scan.ExecRunner); ok {
		runner.WorkingDirectory = root
		scoped.opts.Runner = runner
	}
	if scoped.defaultLoader {
		scoped.opts.Loader = loader.NewRouter(scoped.opts.Runner)
	}
	if scoped.defaultWorkspace {
		// Protect existing invocation-relative artifacts as well as host outputs.
		// An absent old default has no identities to protect; do not create it in
		// the invoking tree merely to establish the selected workspace boundary.
		if _, err := os.Lstat(scoped.opts.WorkspaceDir); errors.Is(err, os.ErrNotExist) {
			scoped.opts.WorkspaceDir = filepath.Join(root, provider.WorkspaceDir)
		}
	}
	if scoped.detectConftestConfig == nil {
		scoped.detectConftestConfig = func(explicit string) (string, error) {
			return findConftestConfig(root, explicit, os.Stat)
		}
	}
	return &scoped
}

func evaluationLabels(target provider.Target, inputPath, branch string) []string {
	if repoURL := target.Variables[loader.VarURL]; repoURL != "" {
		return []string{evidence.StripURLScheme(evidence.PublicContext(repoURL)), branch}
	}
	label := target.TargetID
	if label == "" {
		label = filepath.Base(inputPath)
	}
	return []string{label}
}

func evaluationTarget(target provider.Target, inputPath, branch string) evidence.TargetContext {
	context := evidence.TargetContext{TargetID: target.TargetID, InputPath: inputPath,
		Branch: branch, ScanPath: target.Variables[loader.VarScanPath]}
	if repoURL := target.Variables[loader.VarURL]; repoURL != "" {
		context.URL = repoURL
		context.TargetKey = evidence.RemoteTargetKey(repoURL)
	} else {
		context.TargetKey = evidence.LocalTargetKey(inputPath)
	}
	return context
}

func evaluationConfiguration(scanCfg *generate.ScanConfig,
	policyDir string,
) evidence.ConfigurationContext {
	context := evidence.ConfigurationContext{PolicyDirectory: policyDir, Namespaces: scanCfg.IDs,
		RequirementMapping: scanCfg.ReverseMapping, ContentKey: scanCfg.ContentKey}
	return context
}
