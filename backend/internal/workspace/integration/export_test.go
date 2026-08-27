package integration

import (
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
)

// resetKeyForTest clears the memoised encryption key.
//
// encryptionKey() resolves once per process, which is right in production and
// wrong in a test that changes the environment between cases. This lives in a
// _test file so it is compiled only for tests and cannot be reached from the
// running server.
func resetKeyForTest() { security.ResetEncryptionKeyForTest() }

func nowPlus(d time.Duration) time.Time  { return time.Now().Add(d) }
func nowMinus(d time.Duration) time.Time { return time.Now().Add(-d) }
