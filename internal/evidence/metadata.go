// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var contextURLPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s]+`)

// TargetContext is the explicit metadata allowlist. Source paths are provenance,
// not archived inputs. No environment, credentials, arbitrary variables or digests
// are accepted. TargetKey is only a readable correlation hint, not identity proof.
type TargetContext struct {
	TargetKey      string `json:"target_key"`
	TargetID       string `json:"target_id,omitempty"`
	URL            string `json:"url,omitempty"`
	InputPath      string `json:"input_path,omitempty"`
	Branch         string `json:"branch,omitempty"`
	ScanPath       string `json:"scan_path,omitempty"`
	ObservedTarget string `json:"observed_target,omitempty"`
}

// ConfigurationContext contains only known provider provenance fields. Internal
// paths are workspace-relative; external paths remain absolute. Paths/content keys
// do not attest to the integrity of historical inputs or policy bytes.
type ConfigurationContext struct {
	SpecReference      string            `json:"spec_reference,omitempty"`
	SpecPath           string            `json:"spec_path,omitempty"`
	PolicyPath         string            `json:"policy_path,omitempty"`
	PolicyDirectory    string            `json:"policy_directory,omitempty"`
	RequirementIDs     []string          `json:"requirement_ids,omitempty"`
	ContentKey         string            `json:"content_key,omitempty"`
	Namespaces         []string          `json:"namespaces,omitempty"`
	RequirementMapping map[string]string `json:"requirement_mapping,omitempty"`
	Profile            string            `json:"profile,omitempty"`
	DatastreamPath     string            `json:"datastream_path,omitempty"`
	TailoringPath      string            `json:"tailoring_path,omitempty"`
}

// WarningLogger is the logging boundary for non-fatal supporting-metadata failures.
type WarningLogger interface{ Warn(string, ...any) }

// RecordMetadata warns with the evaluation token on metadata write/close failures,
// without changing whether complete evidence can be published. Call Finalize after
// this method even on a warning so any partial supporting file receives verification.
func (evaluation *Evaluation) RecordMetadata(logger WarningLogger, provider string,
	target TargetContext, configuration ConfigurationContext) {
	if err := evaluation.WriteMetadata(provider, target, configuration); err != nil {
		logger.Warn("cannot save supporting evaluation metadata", "evaluation", evaluation.Token, "error", err)
	}
}

// PublicContext strips userinfo, query and fragment from URLs before persistence
// or display. Non-URL strings pass through; malformed URL-like values are omitted.
func PublicContext(value string) string {
	if !strings.Contains(value, "://") {
		return value
	}
	return contextURLPattern.ReplaceAllStringFunc(value, func(candidate string) string {
		parsed, err := url.Parse(candidate)
		if err != nil {
			return ""
		}
		parsed.User = nil
		parsed.RawQuery, parsed.Fragment, parsed.RawFragment = "", "", ""
		parsed.ForceQuery = false
		return parsed.String()
	})
}

// RemoteTargetKey uses host and escaped path, omitting scheme, credentials, query
// and fragment. Normalization can merge inputs; this is never an evaluation ID.
func RemoteTargetKey(value string) string {
	parsed, err := url.Parse(PublicContext(value))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return "url:" + parsed.Host + parsed.EscapedPath()
}

// LocalTargetKey uses a cleaned absolute path, which is origin-specific.
func LocalTargetKey(path string) string { return "path:" + filepath.Clean(path) }

// HostTargetKey identifies request targets independently of observed ARF targets.
func HostTargetKey(target string) string {
	if target == "" {
		target = "system"
	}
	return "host:" + PublicContext(target)
}

func (evaluation *Evaluation) provenance(path string) string {
	path = PublicContext(path)
	if !filepath.IsAbs(path) {
		return path
	}
	relative, err := filepath.Rel(evaluation.workspace, path)
	if err == nil && filepath.IsLocal(relative) {
		return relative
	}
	return path
}

// WriteMetadata attempts supporting schema-v1 metadata persistence. The caller must
// warn with Token on any write/close error without suppressing otherwise complete
// evidence, then Finalize even if a partial metadata file remains. The Evaluation
// Log, not metadata or directory contents, defines the evidence set and its digests.
func (evaluation *Evaluation) WriteMetadata(provider string, target TargetContext, configuration ConfigurationContext) error {
	target.TargetID = PublicContext(target.TargetID)
	target.URL = PublicContext(target.URL)
	target.TargetKey = PublicContext(target.TargetKey)
	target.InputPath = evaluation.provenance(target.InputPath)
	target.Branch = PublicContext(target.Branch)
	target.ScanPath = PublicContext(target.ScanPath)
	target.ObservedTarget = PublicContext(target.ObservedTarget)
	configuration.SpecReference = PublicContext(configuration.SpecReference)
	configuration.SpecPath = evaluation.provenance(configuration.SpecPath)
	configuration.PolicyPath = evaluation.provenance(configuration.PolicyPath)
	configuration.PolicyDirectory = evaluation.provenance(configuration.PolicyDirectory)
	configuration.Profile = PublicContext(configuration.Profile)
	configuration.DatastreamPath = evaluation.provenance(configuration.DatastreamPath)
	configuration.TailoringPath = evaluation.provenance(configuration.TailoringPath)
	configuration.ContentKey = PublicContext(configuration.ContentKey)
	configuration.RequirementIDs = publicValues(configuration.RequirementIDs)
	configuration.Namespaces = publicValues(configuration.Namespaces)
	if configuration.RequirementMapping != nil {
		mapping := make(map[string]string, len(configuration.RequirementMapping))
		for key, value := range configuration.RequirementMapping {
			mapping[PublicContext(key)] = PublicContext(value)
		}
		configuration.RequirementMapping = mapping
	}
	envelope := struct {
		SchemaVersion int                  `json:"schema_version"`
		EvaluationID  string               `json:"evaluation_id"`
		Provider      string               `json:"provider"`
		CollectedAt   string               `json:"collected_at"`
		Target        TargetContext        `json:"target"`
		Configuration ConfigurationContext `json:"configuration"`
	}{1, evaluation.Token, provider, evaluation.CollectionTime.Format(time.RFC3339), target, configuration}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding evaluation metadata: %w", err)
	}
	return evaluation.Write(MetadataFile, append(data, '\n'))
}

func publicValues(values []string) []string {
	if values == nil {
		return nil
	}
	cleaned := make([]string, len(values))
	for index, value := range values {
		cleaned[index] = PublicContext(value)
	}
	return cleaned
}
