// SPDX-License-Identifier: Apache-2.0

//go:build unix

package server

import (
	"fmt"
	"os"
	"syscall"
)

// fileIdentityOf uses the same device/inode identity as os.SameFile on Unix.
func fileIdentityOf(_ string, info os.FileInfo) (fileIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return fileIdentity{}, fmt.Errorf("filesystem identity unavailable for %q", info.Name())
	}
	return fileIdentity{device: uint64(stat.Dev), inode: uint64(stat.Ino)}, nil
}
