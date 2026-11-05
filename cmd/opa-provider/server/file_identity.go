// SPDX-License-Identifier: Apache-2.0

package server

// fileIdentity uses filesystem-provided volume/device and file identifiers.
type fileIdentity struct {
	device uint64
	inode  uint64
}
