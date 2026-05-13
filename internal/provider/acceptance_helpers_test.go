package provider

import (
	"crypto/rand"
	"fmt"
	"os"
	"testing"
)

// randomTestName returns a unique resource name suitable for live tests. The
// returned string is always prefixed with testResourcePrefix so the sweeper
// can identify it as a test resource later.
func randomTestName(purpose string) string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure on Linux is virtually impossible; if it ever
		// does fail the test should fail loudly rather than picking a
		// predictable fallback.
		panic("randomTestName: " + err.Error())
	}
	return fmt.Sprintf("%s%s-%x", testResourcePrefix, purpose, b[:])
}

// liveMode reports whether the live test mode is active. Tests that need to
// branch on this (e.g. randomize names) should check it instead of reading
// the env var directly.
func liveMode() bool {
	return os.Getenv("TF_ACC_LIVE") == "1"
}

// requireLive skips the calling test if live mode is not active. Use for
// tests that only make sense against the real API.
func requireLive(t *testing.T) {
	t.Helper()
	if !liveMode() {
		t.Skip("set TF_ACC_LIVE=1 to run live-only tests")
	}
}

// testAgentName returns a name appropriate for the current test mode. In
// live mode it's randomized so concurrent CI runs don't collide; in fake
// mode it's stable for assertion convenience.
func testAgentName(purpose string) string {
	if liveMode() {
		return randomTestName(purpose)
	}
	return testResourcePrefix + purpose
}
