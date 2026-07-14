// SPDX-License-Identifier: Apache-2.0

package xccdf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSkippableResult(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"notselected is skippable", "notselected", true},
		{"notapplicable is not skippable", "notapplicable", false},
		{"pass is not skippable", "pass", false},
		{"fail is not skippable", "fail", false},
		{"error is not skippable", "error", false},
		{"empty is not skippable", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSkippableResult(tt.input))
		})
	}
}
