// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"fmt"
	"io"

	"github.com/antchfx/xmlquery"
)

const (
	xccdfNamespace11 = "http://checklists.nist.gov/xccdf/1.1"
	xccdfNamespace12 = "http://checklists.nist.gov/xccdf/1.2"
)

// ParseResults parses a retained XCCDF snapshot and recognizes its result
// structure before publication. It accepts Benchmark-wrapped or standalone
// TestResult output in XCCDF 1.1/1.2; it is not full schema validation.
func ParseResults(reader io.Reader) (*xmlquery.Node, error) {
	node, err := xmlquery.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parsing XCCDF results: %w", err)
	}
	root := node.SelectElement("*")
	if root == nil || (root.NamespaceURI != xccdfNamespace11 && root.NamespaceURI != xccdfNamespace12) {
		return nil, fmt.Errorf("missing XCCDF result document root")
	}
	var results []*xmlquery.Node
	switch root.Data {
	case "TestResult":
		results = []*xmlquery.Node{root}
	case "Benchmark":
		for child := root.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == xmlquery.ElementNode && child.Data == "TestResult" && child.NamespaceURI == root.NamespaceURI {
				results = append(results, child)
			}
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("missing XCCDF TestResult")
	}
	return node, nil
}
