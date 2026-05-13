// SPDX-License-Identifier: Apache-2.0

package loader

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVarConstants_Values(t *testing.T) {
	assert.Equal(t, "url", VarURL)
	assert.Equal(t, "input_path", VarInputPath)
	assert.Equal(t, "branch", VarBranch)
	assert.Equal(t, "branches", VarBranches)
	assert.Equal(t, "access_token", VarAccessToken)
	assert.Equal(t, "scan_path", VarScanPath)
	assert.Equal(t, "opa_bundle_ref", VarOPABundleRef)
}
