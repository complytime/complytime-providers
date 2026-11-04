// SPDX-License-Identifier: Apache-2.0

//go:build !unix && !windows

package server

import (
	"fmt"
	"os"
)

func fileIdentityOf(_ string, info os.FileInfo) (fileIdentity, error) {
	return fileIdentity{}, fmt.Errorf("filesystem identity unavailable for %q on this platform", info.Name())
}
