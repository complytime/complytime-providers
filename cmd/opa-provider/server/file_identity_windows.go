// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"os"
	"syscall"
)

// Windows file identity requires handle information; os.FileInfo.Sys omits it.
func fileIdentityOf(path string, _ os.FileInfo) (fileIdentity, error) {
	file, err := os.Open(path)
	if err != nil {
		return fileIdentity{}, fmt.Errorf("opening file for identity: %w", err)
	}
	var info syscall.ByHandleFileInformation
	statErr := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &info)
	closeErr := file.Close()
	if statErr != nil {
		return fileIdentity{}, fmt.Errorf("reading file identity: %w", statErr)
	}
	if closeErr != nil {
		return fileIdentity{}, fmt.Errorf("closing identity handle: %w", closeErr)
	}
	return fileIdentity{
		device: uint64(info.VolumeSerialNumber),
		inode:  uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
	}, nil
}
