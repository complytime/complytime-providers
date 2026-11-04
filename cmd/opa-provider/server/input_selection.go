// SPDX-License-Identifier: Apache-2.0

package server

import "github.com/complytime/complytime-providers/cmd/opa-provider/results"

// inputSelectionError invalidates all branches of a configured target, unlike
// legacy branch-scoped clone or policy errors.
type inputSelectionError struct {
	err error
}

func (err *inputSelectionError) Error() string { return err.err.Error() }
func (err *inputSelectionError) Unwrap() error { return err.err }

// rejectTargetResults drops assessments without losing earlier operational errors.
func rejectTargetResults(
	previous []*results.PerTargetResult, rejection *results.PerTargetResult,
) []*results.PerTargetResult {
	var rejected []*results.PerTargetResult
	for _, result := range previous {
		if result.Status == "error" {
			rejected = append(rejected, result)
		}
	}
	return append(rejected, rejection)
}
