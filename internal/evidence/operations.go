// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"fmt"
	"io"
	"os"
)

// Operations supplies injectable filesystem boundaries for deterministic failure
// tests. Nil functions use OS operations. Overrides are trusted dependencies, never
// selected through RPC variables or the environment.
type Operations struct {
	Stat          func(*os.File) (os.FileInfo, error)
	Chmod         func(*os.File, os.FileMode) error
	Read          func(*os.File) ([]byte, error)
	Write         func(*os.File, []byte) (int, error)
	Close         func(*os.File) error
	OpenDirectory func(*os.File, string) (*os.File, error)
}

func (operations Operations) defaults() Operations {
	if operations.Stat == nil {
		operations.Stat = (*os.File).Stat
	}
	if operations.Chmod == nil {
		operations.Chmod = (*os.File).Chmod
	}
	if operations.Read == nil {
		operations.Read = func(file *os.File) ([]byte, error) { return io.ReadAll(file) }
	}
	if operations.Write == nil {
		operations.Write = (*os.File).Write
	}
	if operations.Close == nil {
		operations.Close = (*os.File).Close
	}
	if operations.OpenDirectory == nil {
		operations.OpenDirectory = openDirectory
	}
	return operations
}

// VerificationError identifies publication-critical execution-account, mode or
// locator verification failures. Providers return an RPC error, not a per-target
// processing error, when errors.As identifies this type.
type VerificationError struct{ Err error }

// Error describes the failed publication check.
func (failure *VerificationError) Error() string {
	return fmt.Sprintf("evidence verification: %v", failure.Err)
}

// Unwrap preserves the underlying failure for errors.Is and errors.As.
func (failure *VerificationError) Unwrap() error { return failure.Err }
