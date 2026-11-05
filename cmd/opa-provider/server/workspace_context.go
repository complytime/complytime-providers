// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
)

func preflightCancellation(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("input preflight canceled; retry the scan: %w", err)
	}
	return nil
}
