// SPDX-License-Identifier: Apache-2.0

package toolcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckTools_AllPresent(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"go"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	assert.NoError(t, err)
	assert.Empty(t, missing)
}

func TestCheckTools_Missing(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"conftest-nonexistent-tool-abc123"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	assert.NoError(t, err)
	assert.Contains(t, missing, "conftest-nonexistent-tool-abc123")
}

func TestCheckTools_PartiallyMissing(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"go", "conftest-nonexistent-tool-abc123"}
	defer func() { RequiredTools = origTools }()

	missing, err := CheckTools()
	assert.NoError(t, err)
	assert.NotContains(t, missing, "go")
	assert.Contains(t, missing, "conftest-nonexistent-tool-abc123")
}

func TestCheckTools_ReturnSliceOnly(t *testing.T) {
	origTools := RequiredTools
	RequiredTools = []string{"go"}
	defer func() { RequiredTools = origTools }()

	result, err := CheckTools()
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestFormatMissingToolsError(t *testing.T) {
	err := FormatMissingToolsError([]string{"conftest", "git"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conftest")
	assert.Contains(t, err.Error(), "git")
}

func TestFormatMissingToolsError_Empty(t *testing.T) {
	err := FormatMissingToolsError([]string{})
	assert.NoError(t, err)
}
