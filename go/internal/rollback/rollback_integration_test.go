//go:build integration

// rollback_integration_test.go — subprocess-spawning tests extracted from rollback_test.go.
//
// These tests call the real defaultDeleteRemoteTag and defaultRevertAndShip
// implementations on a non-git temp directory, which forks real `git` processes.
// Run with:
//
//	go test -tags integration -race -count=1 ./internal/rollback/
package rollback

import (
	"testing"
)

// TestDefaultDeleteRemoteTag_NonGitDir — ls-remote will fail on a non-git dir
// → empty output → treated as "not-present".
func TestDefaultDeleteRemoteTag_NonGitDir(t *testing.T) {
	d := t.TempDir() // not a git repo
	if got := defaultDeleteRemoteTag(d, "v0.0.0-nope"); got != "not-present" {
		t.Errorf("got %q, want 'not-present' on non-git dir", got)
	}
}

// TestDefaultRevertAndShip_NonGitDir — git revert on a non-git dir fails → "failed".
func TestDefaultRevertAndShip_NonGitDir(t *testing.T) {
	d := t.TempDir()
	if got := defaultRevertAndShip(d, "deadbeef", "x", "0.0.0"); got != "failed" {
		t.Errorf("got %q, want 'failed' on non-git dir", got)
	}
}
