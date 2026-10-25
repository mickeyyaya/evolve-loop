//go:build integration

package bridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealTmux_APollutedServerGivesThePaneOnlyItsEnvPairs(t *testing.T) {
	requireTmux(t)
	socket := DeriveTestSocket(os.Getpid()) + "-hermetic"
	t.Setenv(TmuxSocketEnv, socket)
	t.Setenv("FOO_LEAK_CLIENT", "1")
	seed := exec.Command("tmux", TmuxSocketArgs("-f", os.DevNull, "new-session", "-d", "-s", "seed", ";", "set-environment", "-g", "FOO_LEAK_GLOBAL", "1")...)
	seed.Env = append(os.Environ(), "FOO_LEAK=1", "CLAUDECODE=1")
	if out, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("start the polluted server: %v: %s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("tmux", TmuxSocketArgs("kill-server")...).Run() })

	ctx := context.Background()
	dump := filepath.Join(t.TempDir(), "env.txt")
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "SHELL=/bin/zsh", "KEEP_ME=1"}
	if err := (execTmux{}).NewSessionEnv(ctx, "cell", 120, 40, t.TempDir(), env); err != nil {
		t.Fatalf("NewSessionEnv: %v", err)
	}
	if err := (execTmux{}).SendKeys(ctx, "cell", "env > "+dump+".tmp && mv "+dump+".tmp "+dump, true); err != nil {
		t.Fatalf("send-keys: %v", err)
	}
	var got []byte
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if b, err := os.ReadFile(dump); err == nil {
			got = b
			break
		}
	}
	if got == nil {
		t.Fatal("the pane never wrote its environment")
	}
	lines := strings.Split(string(got), "\n")
	has := func(prefix string) bool {
		for _, l := range lines {
			if strings.HasPrefix(l, prefix) {
				return true
			}
		}
		return false
	}
	if !has("KEEP_ME=1") || !has("PATH=/usr/bin:/bin") {
		t.Errorf("the pane lost its -e pairs; env:\n%s", got)
	}
	for _, leak := range []string{"FOO_LEAK=", "FOO_LEAK_GLOBAL=", "FOO_LEAK_CLIENT=", "CLAUDECODE=", "TMUX="} {
		if has(leak) {
			t.Errorf("%s reached the pane from the server or the client; env:\n%s", leak, got)
		}
	}
}
