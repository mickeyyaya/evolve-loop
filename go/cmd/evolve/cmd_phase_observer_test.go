package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestObserverEnvConfig_Defaults(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	c := observerEnvConfig()
	if c.PollS != 5 || c.StallS != 600 || c.EOFGraceS != 0 {
		t.Errorf("PollS/StallS/EOFGraceS = %d/%d/%d, want 5/600/0", c.PollS, c.StallS, c.EOFGraceS)
	}
	if c.NudgeS != 300 {
		t.Errorf("NudgeS default = %d, want 300 (a flip to 0 disables the nudge)", c.NudgeS)
	}
	if c.NudgeBody != "" {
		t.Errorf("NudgeBody default = %q, want empty", c.NudgeBody)
	}
}

func TestObserverEnvConfig_Parsing(t *testing.T) {
	root := writeObserverPolicy(t, `{"observer":{"poll_s":7,"stall_s":20,"nudge_s":0,"nudge_body":"wake up","eof_grace_s":3}}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	c := observerEnvConfig()
	if c.PollS != 7 || c.StallS != 20 || c.NudgeS != 0 || c.EOFGraceS != 3 || c.NudgeBody != "wake up" {
		t.Errorf("got PollS=%d StallS=%d NudgeS=%d EOFGraceS=%d NudgeBody=%q", c.PollS, c.StallS, c.NudgeS, c.EOFGraceS, c.NudgeBody)
	}
}

func writeObserverPolicy(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
