// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ownershipInfo struct {
	os.FileInfo
	owner syscall.Stat_t
}

func (info ownershipInfo) Sys() any { return &info.owner }

func TestEvaluation_ExecutionIdentityAndPrivateModes(t *testing.T) {
	t.Setenv("SUDO_UID", "12345")
	t.Setenv("SUDO_GID", "23456")
	t.Setenv("USER", "not-the-execution-account")
	actual, err := ExecutionIdentity()
	require.NoError(t, err)
	assert.Equal(t, os.Geteuid(), actual.UID)
	assert.Equal(t, os.Getegid(), actual.GID)
	for _, identity := range []Identity{{UID: 0, GID: 0}, {UID: 1000, GID: 1000}} {
		options := storageOptions(t)
		options.Identity = identity
		options.Operations.Stat = func(file *os.File) (os.FileInfo, error) {
			info, err := file.Stat()
			if err != nil {
				return nil, err
			}
			owner := *info.Sys().(*syscall.Stat_t)
			if identity.UID == 0 {
				owner.Uid, owner.Gid = 0, 0
			} else {
				owner.Uid, owner.Gid = 1000, 1000
			}
			return ownershipInfo{FileInfo: info, owner: owner}, nil
		}
		evaluation := allocateTest(t, options)
		require.NoError(t, evaluation.Write(ConftestFile, []byte("[]")))
		_, err := evaluation.Read(ConftestFile)
		require.NoError(t, err)
		require.NoError(t, evaluation.Finalize())
		info, err := os.Stat(evaluation.Path(ConftestFile))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(FileMode), info.Mode().Perm())
	}
}

func TestEvaluation_VerificationFailuresArePublicationCritical(t *testing.T) {
	for _, failure := range []string{"ownership", "chmod", "stat", "parent-access"} {
		t.Run(failure, func(t *testing.T) {
			evaluation := allocateTest(t, storageOptions(t), "context")
			require.NoError(t, evaluation.Write(ConftestFile, []byte("[]")))
			_, err := evaluation.Read(ConftestFile)
			require.NoError(t, err)
			switch failure {
			case "ownership":
				evaluation.owner.UID++
			case "chmod":
				evaluation.operations.Chmod = func(*os.File, os.FileMode) error { return syscall.EPERM }
			case "stat":
				evaluation.operations.Stat = func(*os.File) (os.FileInfo, error) { return nil, syscall.EIO }
			case "parent-access":
				evaluation.operations.OpenDirectory = func(*os.File, string) (*os.File, error) { return nil, syscall.EACCES }
			}
			err = evaluation.Finalize()
			var critical *VerificationError
			require.ErrorAs(t, err, &critical)
			assert.NotEmpty(t, critical.Error())
			assert.NotNil(t, errors.Unwrap(critical))
		})
	}
}
