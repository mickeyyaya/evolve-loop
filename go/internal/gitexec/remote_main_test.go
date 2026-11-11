package gitexec

import "testing"

func TestRemoteMainIsTheRemoteAndTheMainBranch(t *testing.T) {
	if Remote != "origin" || MainBranch != "main" || RemoteMain != "origin/main" {
		t.Fatalf("Remote=%q MainBranch=%q RemoteMain=%q", Remote, MainBranch, RemoteMain)
	}
}
