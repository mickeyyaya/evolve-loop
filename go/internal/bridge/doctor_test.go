package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func doctorEngine(env map[string]string, present map[string]bool, runnerRC int, runnerErr error) *Engine {
	return NewEngine(Deps{
		LookupEnv: mapLookup(env),
		LookPath: func(b string) (string, error) {
			if present[b] {
				return "/usr/bin/" + b, nil
			}
			return "", errNoBin
		},
		Runner: func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
			if len(args) > 0 && args[0] == "--version" {
				_, _ = stdout.Write([]byte("v1.2.3\n"))
				return 0, nil
			}
			return runnerRC, runnerErr
		},
		KeychainProbe: func(string) bool { return false },
		Now:           time.Now,
	})
}

func TestDriverBinary_DoctorProbesEachDriversBinary(t *testing.T) {
	t.Parallel()
	for cli, want := range map[string]string{
		"claude-p": "claude", "claude-tmux": "claude", "codex": "codex",
		"codex-tmux": "codex", "agy": "agy", "agy-tmux": "agy", "other-tmux": "other",
	} {
		if got := driverBinary(cli); got != want {
			t.Fatalf("driverBinary(%q)=%q want %q", cli, got, want)
		}
	}
}

func TestDoctorHome_Fallback(t *testing.T) {
	t.Parallel()
	if h := doctorEngine(map[string]string{"HOME": "/x"}, nil, 0, nil).doctorHome(); h != "/x" {
		t.Fatalf("HOME from env = %q", h)
	}
	e := NewEngine(Deps{LookupEnv: mapLookup(nil)})
	if e.doctorHome() != os.Getenv("HOME") {
		t.Fatal("doctorHome should fall back to os.Getenv")
	}
}

func TestDoctorAuth(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	eng := doctorEngine(map[string]string{"HOME": home}, nil, 0, nil)

	if a := eng.doctorAuth("claude-p"); a.Configured {
		t.Fatal("claude auth should be unconfigured")
	}
	if a := eng.doctorAuth("codex"); a.Configured {
		t.Fatal("codex auth should be unconfigured")
	}
	if a := eng.doctorAuth("agy"); a.Configured {
		t.Fatal("agy auth should be unconfigured")
	}
	if a := eng.doctorAuth("mystery"); a.Hint == "" {
		t.Fatal("unknown cli should carry a hint")
	}

	mkfile(t, filepath.Join(home, ".claude", ".credentials.json"), "{}")
	if a := eng.doctorAuth("claude-tmux"); !a.Configured || a.Source != "file:credentials.json" {
		t.Fatalf("claude auth = %+v", a)
	}
	mkfile(t, filepath.Join(home, ".codex", "auth.json"), "{}")
	if a := eng.doctorAuth("codex"); !a.Configured {
		t.Fatalf("codex valid auth = %+v", a)
	}
	mkfile(t, filepath.Join(home, ".codex", "auth.json"), "{bad")
	if a := eng.doctorAuth("codex"); a.Configured {
		t.Fatal("codex invalid JSON should be unconfigured")
	}
	if err := os.MkdirAll(filepath.Join(home, ".config", "agy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if a := eng.doctorAuth("agy-tmux"); !a.Configured {
		t.Fatalf("agy auth = %+v", a)
	}
}

func TestDoctorAuth_ClaudeKeychain(t *testing.T) {
	t.Parallel()
	home := t.TempDir()

	eng := NewEngine(Deps{
		LookupEnv:     mapLookup(map[string]string{"HOME": home}),
		KeychainProbe: func(service string) bool { return service == "Claude Code-credentials" },
	})
	if a := eng.doctorAuth("claude-tmux"); !a.Configured || a.Source != "keychain:Claude Code-credentials" {
		t.Fatalf("claude keychain auth = %+v, want Configured via keychain", a)
	}

	eng2 := NewEngine(Deps{
		LookupEnv:     mapLookup(map[string]string{"HOME": home}),
		KeychainProbe: func(string) bool { return false },
	})
	for _, cli := range []string{"claude-p", "claude-tmux"} {
		if a := eng2.doctorAuth(cli); a.Configured {
			t.Fatalf("%s: no file + no keychain should be unconfigured; got %+v", cli, a)
		}
	}
}

func TestDoctorEnvWarnings(t *testing.T) {
	t.Parallel()
	e := doctorEngine(map[string]string{
		"ANTHROPIC_API_KEY": "k", "ANTHROPIC_BASE_URL": "u", "OPENAI_API_KEY": "o",
	}, nil, 0, nil)
	if w := e.doctorEnvWarnings("claude-p"); len(w) != 2 {
		t.Fatalf("claude warnings = %v", w)
	}
	if w := e.doctorEnvWarnings("codex"); len(w) != 1 {
		t.Fatalf("codex warnings = %v", w)
	}
	if w := e.doctorEnvWarnings("agy"); len(w) != 0 {
		t.Fatalf("agy warnings = %v", w)
	}
	e2 := doctorEngine(map[string]string{"OPENAI_API_KEY": "o", "BRIDGE_ALLOW_OPENAI_API_KEY": "1"}, nil, 0, nil)
	if w := e2.doctorEnvWarnings("codex-tmux"); len(w) != 1 {
		t.Fatalf("the environment opt-in is retired; warnings = %v", w)
	}
	e2.deps.AllowOpenAIAPIKey = true
	if w := e2.doctorEnvWarnings("codex-tmux"); len(w) != 0 {
		t.Fatalf("codex allowed warnings = %v", w)
	}
}

func TestDoctorDeep(t *testing.T) {
	t.Parallel()
	e := doctorEngine(nil, nil, 0, nil)
	if dp := e.doctorDeep(context.Background(), "claude-tmux", "claude"); dp.Ran {
		t.Fatal("tmux deep should not run")
	}
	if dp := e.doctorDeep(context.Background(), "claude-p", "claude"); !dp.Ran || !dp.Passed {
		t.Fatalf("claude-p deep = %+v", dp)
	}
	if dp := e.doctorDeep(context.Background(), "codex", "codex"); !dp.Ran || !dp.Passed {
		t.Fatalf("codex deep = %+v", dp)
	}
	if dp := e.doctorDeep(context.Background(), "agy", "agy"); !dp.Ran || !dp.Passed {
		t.Fatalf("agy deep = %+v", dp)
	}
	if dp := e.doctorDeep(context.Background(), "zzz", "zzz"); dp.Ran {
		t.Fatal("unknown cli deep should not run")
	}
	ef := doctorEngine(nil, nil, 1, nil)
	if dp := ef.doctorDeep(context.Background(), "claude-p", "claude"); !dp.Ran || dp.Passed {
		t.Fatalf("claude-p deep fail = %+v", dp)
	}
}

func TestDoctorVersion_Error(t *testing.T) {
	t.Parallel()
	e := NewEngine(Deps{Runner: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return -1, errNoBin
	}})
	if v := doctorVersion(context.Background(), e.deps, "x"); v != "unknown" {
		t.Fatalf("version on runner error = %q, want unknown", v)
	}
}

func TestDoctor_VerdictsAndExitCodes(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	mkfile(t, filepath.Join(home, ".claude", ".credentials.json"), "{}")

	if _, code := doctorEngine(map[string]string{"HOME": home}, nil, 0, nil).Doctor(context.Background(), "claude-p", false); code != 2 {
		t.Fatalf("no-binary exit = %d, want 2 (blocked)", code)
	}
	if _, code := doctorEngine(map[string]string{"HOME": t.TempDir()}, map[string]bool{"claude": true}, 0, nil).Doctor(context.Background(), "claude-p", false); code != 2 {
		t.Fatalf("present-binary no-auth exit = %d, want 2 (blocked)", code)
	}
	rep, code := doctorEngine(map[string]string{"HOME": home}, map[string]bool{"claude": true}, 0, nil).Doctor(context.Background(), "claude-p", false)
	if code != ExitOK || rep.Summary.Ready != 1 {
		t.Fatalf("ready exit=%d summary=%+v", code, rep.Summary)
	}
	_, code = doctorEngine(map[string]string{"HOME": home, "ANTHROPIC_API_KEY": "k"}, map[string]bool{"claude": true}, 0, nil).Doctor(context.Background(), "claude-p", false)
	if code != 1 {
		t.Fatalf("warning exit = %d, want 1", code)
	}
	_, code = doctorEngine(map[string]string{"HOME": home}, map[string]bool{"claude": true}, 1, nil).Doctor(context.Background(), "claude-p", true)
	if code != 2 {
		t.Fatalf("deep-fail exit = %d, want 2", code)
	}
	repAll, _ := doctorEngine(map[string]string{"HOME": home}, nil, 0, nil).Doctor(context.Background(), "", false)
	if len(repAll.Results) < 6 || repAll.Host == "" || repAll.ScannedAt == "" {
		t.Fatalf("full sweep results=%d host=%q", len(repAll.Results), repAll.Host)
	}
}

func mkfile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
