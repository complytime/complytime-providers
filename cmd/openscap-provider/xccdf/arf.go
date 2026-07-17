// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/antchfx/xmlquery"
)

const (
	// OVALCheckType is the OVAL system URI used in XCCDF check elements.
	OVALCheckType = "http://oval.mitre.org/XMLSchema/oval-definitions-5"
)

// ovalRegex extracts the short name from an OVAL check definition ID.
var ovalRegex = regexp.MustCompile(`^[^:]*?:[^-]*?-(.*?):.*?$`)

// ParseARFFile opens and parses an ARF XML file, returning the root node.
func ParseARFFile(arfPath string) (*xmlquery.Node, error) {
	file, err := os.Open(filepath.Clean(arfPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open ARF: %w", err)
	}
	defer file.Close()

	xmlnode, err := xmlquery.Parse(bufio.NewReader(file))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ARF: %w", err)
	}
	return xmlnode, nil
}

// IsSkippableResult returns true for XCCDF result statuses that should
// be excluded from scan assessments. "notselected" means the rule was
// not selected for evaluation in the tailoring profile and carries no
// compliance signal. "notapplicable" is NOT skippable — it indicates
// the rule was evaluated but does not apply to the target, which is
// meaningful compliance information reported as RESULT_SKIPPED.
func IsSkippableResult(resultText string) bool {
	return resultText == "notselected"
}

// FindOVALCheckContentRef locates the check-content-ref element for the
// OVAL check system within an XCCDF Rule node.
func FindOVALCheckContentRef(rule *xmlquery.Node) *xmlquery.Node {
	for _, check := range rule.SelectElements("//xccdf-1.2:check") {
		if check.SelectAttr("system") == OVALCheckType {
			return check.SelectElement("xccdf-1.2:check-content-ref")
		}
	}
	return nil
}

// ParseCheck extracts the requirement ID (short name) from an OVAL
// check-content-ref node's name attribute.
func ParseCheck(check *xmlquery.Node) (string, error) {
	ovalCheckName := strings.TrimSpace(check.SelectAttr("name"))
	if ovalCheckName == "" {
		return "", errors.New("check-content-ref node has no 'name' attribute")
	}
	matches := ovalRegex.FindStringSubmatch(ovalCheckName)

	minimumPart, shortNameLoc := 2, 1
	if len(matches) < minimumPart {
		return "", fmt.Errorf("check id %q is in unexpected format", ovalCheckName)
	}
	return matches[shortNameLoc], nil
}

// RuleResultMessage builds a human-readable message from the XCCDF
// Rule definition and rule-result node.
func RuleResultMessage(rule *xmlquery.Node, result *xmlquery.Node, resultText string) string {
	title := ""
	if el := rule.SelectElement("xccdf-1.2:title"); el != nil {
		title = strings.TrimSpace(el.InnerText())
	}

	var parts []string
	for _, msg := range result.SelectElements("message") {
		if t := strings.TrimSpace(msg.InnerText()); t != "" {
			parts = append(parts, t)
		}
	}
	diagnostic := strings.Join(parts, "; ")

	if title != "" && diagnostic != "" {
		return fmt.Sprintf("%s — %s (%s)", title, diagnostic, resultText)
	}
	if title != "" {
		return fmt.Sprintf("%s (%s)", title, resultText)
	}
	if diagnostic != "" {
		return fmt.Sprintf("%s (%s)", diagnostic, resultText)
	}
	return fmt.Sprintf("openscap rule-result is %s", resultText)
}
