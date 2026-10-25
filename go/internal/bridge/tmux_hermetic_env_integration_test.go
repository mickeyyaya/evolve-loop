//go:build integration

package bridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	got := dumpPaneEnv(t, "cell", []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "SHELL=/bin/zsh", "KEEP_ME=1"})
	if !hasLine(got, "KEEP_ME=1") || !hasLine(got, "PATH=/usr/bin:/bin") {
		t.Errorf("the pane lost its cell env; env:\n%s", got)
	}
	for _, leak := range []string{"FOO_LEAK=", "FOO_LEAK_GLOBAL=", "FOO_LEAK_CLIENT=", "CLAUDECODE=", "TMUX="} {
		if hasLine(got, leak) {
			t.Errorf("%s reached the pane from the server or the client; env:\n%s", leak, got)
		}
	}
}

func TestRealTmux_ASecretValueIsInNoArgvAndNoLeftoverFile(t *testing.T) {
	requireTmux(t)
	socket := DeriveTestSocket(os.Getpid()) + "-secret"
	t.Setenv(TmuxSocketEnv, socket)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Cleanup(func() { _ = exec.Command("tmux", TmuxSocketArgs("kill-server")...).Run() })
	const sentinel = "sk-ant-oat-SENTINEL-7f3a"

	got := dumpPaneEnv(t, "secret-cell", []string{"PATH=/usr/bin:/bin", "HOME=" + tmp, "CLAUDE_CODE_OAUTH_TOKEN=" + sentinel})
	if !hasLine(got, "CLAUDE_CODE_OAUTH_TOKEN="+sentinel) {
		t.Fatalf("the pane must still get the secret; env:\n%s", got)
	}
	ps, err := exec.Command("ps", "-axww", "-o", "args").Output()
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	if strings.Contains(string(ps), sentinel) {
		for _, l := range strings.Split(string(ps), "\n") {
			if strings.Contains(l, sentinel) {
				t.Errorf("argv holds the secret: %.60s", strings.ReplaceAll(l, sentinel, "<SENTINEL>"))
			}
		}
	}
	shown, _ := exec.Command("tmux", TmuxSocketArgs("show-environment", "-t", "secret-cell")...).Output()
	if strings.Contains(string(shown), sentinel) {
		t.Fatal("a secret value is in the tmux session environment")
	}
	leftovers, _ := filepath.Glob(filepath.Join(tmp, "evolve-cellenv-*"))
	if len(leftovers) != 0 {
		t.Fatalf("the cell env file must be gone after the start: %v", leftovers)
	}
	if fp, err := (execTmux{}).SessionEnvFingerprint(context.Background(), "secret-cell"); err != nil || fp == "" {
		t.Fatalf("the session must carry its env fingerprint; got %q, %v", fp, err)
	}
}

func dumpPaneEnv(t *testing.T, session string, env []string) string {
	t.Helper()
	ctx := context.Background()
	dump := filepath.Join(t.TempDir(), "env.txt")
	if err := (execTmux{}).NewSessionEnv(ctx, session, 120, 40, t.TempDir(), env); err != nil {
		t.Fatalf("NewSessionEnv: %v", err)
	}
	if err := (execTmux{}).SendKeys(ctx, session, "env > "+dump+".tmp && mv "+dump+".tmp "+dump, true); err != nil {
		t.Fatalf("send-keys: %v", err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if b, err := os.ReadFile(dump); err == nil {
			return string(b)
		}
	}
	t.Fatal("the pane never wrote its environment")
	return ""
}

func hasLine(env, prefix string) bool {
	for _, l := range strings.Split(env, "\n") {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

func TestRealTmux_AFailedStartLeavesNoEnvFile(t *testing.T) {
	requireTmux(t)
	socket := DeriveTestSocket(os.Getpid()) + "-failstart"
	t.Setenv(TmuxSocketEnv, socket)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Cleanup(func() { _ = exec.Command("tmux", TmuxSocketArgs("kill-server")...).Run() })
	ctx := context.Background()
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + tmp, "SECRET_ENV=1"}
	if err := (execTmux{}).NewSessionEnv(ctx, "dup", 80, 24, "", env); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := (execTmux{}).NewSessionEnv(ctx, "dup", 80, 24, "", env); err == nil {
		t.Fatal("a duplicate session name must fail the start")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(tmp, "evolve-cellenv-*")); len(leftovers) != 0 {
		t.Fatalf("a failed start must remove its env file: %v", leftovers)
	}
	t.Setenv("TMPDIR", filepath.Join(tmp, "missing"))
	if err := (execTmux{}).NewSessionEnv(ctx, "nodir", 80, 24, "", env); err == nil {
		t.Fatal("a start that cannot write its env file must fail, never fall back to argv")
	}
}

func TestRealTmux_EachSessionOfASharedServerCarriesItsOwnFingerprint(t *testing.T) {
	requireTmux(t)
	socket := DeriveTestSocket(os.Getpid()) + "-twofp"
	t.Setenv(TmuxSocketEnv, socket)
	t.Cleanup(func() { _ = exec.Command("tmux", TmuxSocketArgs("kill-server")...).Run() })
	ctx := context.Background()
	a := []string{"PATH=/usr/bin:/bin", "CELL=a"}
	b := []string{"PATH=/usr/bin:/bin", "CELL=b"}
	for name, env := range map[string][]string{"cell-a": a, "cell-b": b} {
		if err := (execTmux{}).NewSessionEnv(ctx, name, 80, 24, "", env); err != nil {
			t.Fatalf("start %s: %v", name, err)
		}
	}
	for name, env := range map[string][]string{"cell-a": a, "cell-b": b} {
		if got, err := (execTmux{}).SessionEnvFingerprint(ctx, name); err != nil || got != envFingerprint(env) {
			t.Errorf("%s fingerprint = %q, %v; want its own %q", name, got, err, envFingerprint(env))
		}
	}
}

func TestRealTmux_AKilledSessionLeavesNoEnvFile(t *testing.T) {
	requireTmux(t)
	socket := DeriveTestSocket(os.Getpid()) + "-earlykill"
	t.Setenv(TmuxSocketEnv, socket)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Cleanup(func() { _ = exec.Command("tmux", TmuxSocketArgs("kill-server")...).Run() })
	ctx := context.Background()
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + tmp, "CLAUDE_CODE_OAUTH_TOKEN=fake-secret-for-test"}
	for i := range 20 {
		name := "early-" + strconv.Itoa(i)
		if err := (execTmux{}).NewSessionEnv(ctx, name, 80, 24, "", env); err != nil {
			t.Fatalf("start %s: %v", name, err)
		}
		if err := (execTmux{}).KillSession(ctx, name); err != nil {
			t.Fatalf("kill %s: %v", name, err)
		}
	}
	if leftovers, _ := filepath.Glob(filepath.Join(tmp, "evolve-cellenv-*")); len(leftovers) != 0 {
		t.Fatalf("a session killed right after its start left %d env files: %v", len(leftovers), leftovers)
	}
}

func TestRealTmux_AFailedStartNeverKillsASessionItDidNotCreate(t *testing.T) {
	requireTmux(t)
	socket := DeriveTestSocket(os.Getpid()) + "-notmine"
	t.Setenv(TmuxSocketEnv, socket)
	t.Setenv("TMPDIR", t.TempDir())
	t.Cleanup(func() { _ = exec.Command("tmux", TmuxSocketArgs("kill-server")...).Run() })
	if out, err := exec.Command("tmux", TmuxSocketArgs("-f", os.DevNull, "new-session", "-d", "-s", "theirs")...).CombinedOutput(); err != nil {
		t.Fatalf("start the session of another owner: %v: %s", err, out)
	}
	ctx := context.Background()
	if err := (execTmux{}).NewSessionEnv(ctx, "theirs", 80, 24, "", []string{"PATH=/usr/bin:/bin"}); err == nil {
		t.Fatal("a duplicate session name must fail the start")
	}
	if !(execTmux{}).HasSession(ctx, "theirs") {
		t.Fatal("a failed start killed a session that it did not create")
	}
}
