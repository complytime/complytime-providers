// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func requireRealConftest(t testing.TB) string {
	t.Helper()
	binary, err := exec.LookPath("conftest")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			t.Skip("conftest is not installed on PATH")
		}
		t.Fatalf("locating conftest: %v", err)
	}
	version, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		t.Logf("conftest version unavailable: %v (%s); running behavior tests", err, version)
	} else {
		t.Logf("real conftest version: %s", strings.TrimSpace(string(version)))
	}
	return binary
}
