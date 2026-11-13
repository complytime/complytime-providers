// SPDX-License-Identifier: Apache-2.0

package evidence

import "syscall"

func makeFIFO(path string) error { return syscall.Mkfifo(path, FileMode) }
