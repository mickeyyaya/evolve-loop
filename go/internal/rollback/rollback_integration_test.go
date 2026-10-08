//go:build integration

package rollback

import (
	"testing"
)

func TestDefaultDeleteRemoteTag_NonGitDir(t *testing.T) {
	d := t.TempDir()
	if got := defaultDeleteRemoteTag(d, "v0.0.0-nope"); got != "failed" {
		t.Errorf("got %q, want 'failed' on non-git dir (lookup failure is not absence)", got)
	}
}

func TestDefaultRevertAndShip_NonGitDir(t *testing.T) {
	d := t.TempDir()
	if got := defaultRevertAndShip(d, "deadbeef", "x", "0.0.0"); got != "failed" {
		t.Errorf("got %q, want 'failed' on non-git dir", got)
	}
}
