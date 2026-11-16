// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResults_RecognizesXCCDFOutput(t *testing.T) {
	for _, text := range []string{
		`<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2"><TestResult/></Benchmark>`,
		`<xccdf:TestResult xmlns:xccdf="http://checklists.nist.gov/xccdf/1.2"/>`,
		`<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.1"><TestResult/></Benchmark>`,
	} {
		node, err := ParseResults(strings.NewReader(text))
		require.NoError(t, err)
		assert.NotEmpty(t, node.SelectElement("*").NamespaceURI)
	}
	for _, text := range []string{"", "<broken", "<root/>",
		`<Benchmark><TestResult/></Benchmark>`,
		`<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2"/>`,
		`<root xmlns="http://checklists.nist.gov/xccdf/1.2"><TestResult/></root>`,
		`<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2"><TestResult xmlns="urn:unrelated"/></Benchmark>`,
	} {
		node, err := ParseResults(strings.NewReader(text))
		require.Error(t, err, text)
		assert.Nil(t, node)
	}
}
