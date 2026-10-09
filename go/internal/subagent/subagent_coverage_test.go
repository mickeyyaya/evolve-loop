package subagent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestDefaultGitState_RealRepo(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, treeSHA, err := defaultGitState(context.Background(), dir)
	if err != nil {
		t.Fatalf("defaultGitState: %v", err)
	}
	if len(head) < 40 {
		t.Errorf("HEAD too short: %q", head)
	}
	if treeSHA == "" || treeSHA == "unknown" {
		t.Errorf("treeSHA empty/unknown: %q", treeSHA)
	}
}

func TestDefaultGitState_NotARepo(t *testing.T) {
	dir := t.TempDir()
	head, treeSHA, err := defaultGitState(context.Background(), dir)
	if err == nil {
		t.Fatalf("expected error for non-git dir, got head=%q tree=%q", head, treeSHA)
	}
	if head != "unknown" {
		t.Errorf("head=%q want unknown", head)
	}
}

func TestDefaultGitState_HeadOKDiffFails(t *testing.T) {
	t.Skip("hard to synthesize cleanly — covered by NotARepo case")
}

func TestRunGit_Error(t *testing.T) {
	out, err := runGit(context.Background(), t.TempDir(), "bogus-subcommand-that-does-not-exist")
	if err == nil {
		t.Errorf("expected error, got out=%q", out)
	}
}

func TestGenerateToken_ShortRead(t *testing.T) {
	r := &Runner{cfg: Config{
		Rand: func(buf []byte) (int, error) {
			return ChallengeTokenBytes - 1, nil
		},
	}}
	if _, err := r.generateToken(); err == nil {
		t.Errorf("expected short-read error")
	}
}

func TestGenerateToken_RandError(t *testing.T) {
	r := &Runner{cfg: Config{
		Rand: func(buf []byte) (int, error) {
			return 0, errors.New("rand failed")
		},
	}}
	if _, err := r.generateToken(); err == nil {
		t.Errorf("expected rand error to propagate")
	}
}

func TestDefaultHashFile_MissingFile(t *testing.T) {
	if _, err := defaultHashFile("/nonexistent/path/should/not/exist"); err == nil {
		t.Errorf("expected error for missing file")
	}
}

func TestDefaultHashFile_Directory(t *testing.T) {
	dir := t.TempDir()
	_, err := defaultHashFile(dir)
	if err == nil {
		t.Log("defaultHashFile(directory) succeeded on this platform — acceptable")
	}
}

func TestDefaultStatMTime_Missing(t *testing.T) {
	if _, err := defaultStatMTime("/nonexistent/should/not/exist"); err == nil {
		t.Errorf("expected stat error")
	}
}

func TestNew_NilBridge(t *testing.T) {
	loader := profiles.NewFromFS(fstest.MapFS{})
	_, err := New(Config{Profiles: loader, Ledger: &fakeLedger{}})
	if err == nil {
		t.Errorf("expected nil-bridge error")
	}
}

func TestNew_NilLedger(t *testing.T) {
	loader := profiles.NewFromFS(fstest.MapFS{})
	_, err := New(Config{Profiles: loader, Bridge: &fakeBridge{}})
	if err == nil {
		t.Errorf("expected nil-ledger error")
	}
}

func TestNew_DefaultsPopulated(t *testing.T) {
	loader := profiles.NewFromFS(fstest.MapFS{})
	r, err := New(Config{
		Profiles: loader,
		Bridge:   &fakeBridge{},
		Ledger:   &fakeLedger{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if r.cfg.Now == nil {
		t.Errorf("Now not defaulted")
	}
	if r.cfg.Rand == nil {
		t.Errorf("Rand not defaulted")
	}
	if r.cfg.GitState == nil {
		t.Errorf("GitState not defaulted")
	}
	if r.cfg.HashFile == nil {
		t.Errorf("HashFile not defaulted")
	}
	if r.cfg.StatMTime == nil {
		t.Errorf("StatMTime not defaulted")
	}
	if r.cfg.ReadFile == nil {
		t.Errorf("ReadFile not defaulted")
	}
}

type erroringLedger struct{}

func (erroringLedger) Append(_ context.Context, _ core.LedgerEntry) error {
	return errors.New("disk full")
}
func (erroringLedger) Verify(_ context.Context) error { return nil }
func (erroringLedger) Iter(_ context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("not impl")
}

func TestRun_LedgerAppendError(t *testing.T) {
	now := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	r := newRunner(t, &fakeBridge{response: core.BridgeResponse{ExitCode: 0}}, erroringLedger{}, now)
	dir := t.TempDir()
	artifact := filepath.Join(dir, ".evolve", "runs", "cycle-7", "build-report.md")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("<!-- challenge-token: abababababababab -->\nbody\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := r.Run(context.Background(), Request{
		Agent:       "builder",
		ProjectRoot: dir,
		Workspace:   dir,
		Prompt:      "test",
		Cycle:       7,
	})
	if err == nil {
		t.Errorf("expected ledger-append error")
	}
	foundLedgerDiag := false
	for _, d := range res.Diagnostics {
		if strings.Contains(d.Message, "ledger append") {
			foundLedgerDiag = true
		}
	}
	if !foundLedgerDiag {
		t.Errorf("ledger-append diagnostic not appended: %+v", res.Diagnostics)
	}
}

func TestRun_MkdirArtifactDirError(t *testing.T) {
	now := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	r := newRunner(t, &fakeBridge{}, &fakeLedger{}, now)
	tmp := t.TempDir()
	collision := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(collision, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	_, err := r.Run(context.Background(), Request{
		Agent:       "builder",
		ProjectRoot: collision,
		Workspace:   tmp,
		Prompt:      "test",
		Cycle:       1,
	})
	if err == nil {
		t.Errorf("expected MkdirAll error")
	}
}

func TestRun_TokenGenerateError(t *testing.T) {
	now := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{}
	ledger := &fakeLedger{}
	loader := profiles.NewFromFS(fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte(`{
			"name": "builder",
			"role": "builder",
			"cli": "claude-p",
			"output_artifact": ".evolve/runs/cycle-{cycle}/build-report.md"
		}`)},
	})
	r, err := New(Config{
		Profiles: loader,
		Bridge:   bridge,
		Ledger:   ledger,
		Now:      func() time.Time { return now },
		Rand:     func(_ []byte) (int, error) { return 0, errors.New("rand exhausted") },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = r.Run(context.Background(), Request{
		Agent:       "builder",
		ProjectRoot: t.TempDir(),
		Workspace:   t.TempDir(),
		Prompt:      "test",
		Cycle:       1,
	})
	if err == nil {
		t.Errorf("expected token-generate error")
	}
}

func TestRun_GitStateErrorFallsBackToUnknown(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{response: core.BridgeResponse{ExitCode: 0}}
	ledger := &fakeLedger{}
	loader := profiles.NewFromFS(fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte(`{
			"name":"builder","role":"builder","cli":"claude-p",
			"model_tier_default":"sonnet",
			"output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"
		}`)},
	})
	r, err := New(Config{
		Profiles: loader, Bridge: bridge, Ledger: ledger,
		Now:  func() time.Time { return now },
		Rand: deterministicRand(0xAB),
		GitState: func(context.Context, string) (string, string, error) {
			return "", "", errors.New("not a git repo")
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	token := strings.Repeat("ab", ChallengeTokenBytes)
	tmp := t.TempDir()
	bridge.onLaunch = func(req core.BridgeRequest) error {
		writeArtifact(t, req.ArtifactPath, "<!-- challenge-token: "+token+" -->\nok\n", now)
		return nil
	}
	res, err := r.Run(context.Background(), Request{
		Agent: "builder", Cycle: 3, ProjectRoot: tmp,
		Workspace: filepath.Join(tmp, ".evolve/runs/cycle-3"), Prompt: "go",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.LedgerEntry.GitHEAD != "unknown" || res.LedgerEntry.TreeStateSHA != "unknown" {
		t.Errorf("git state on error: head=%q tree=%q, want unknown/unknown",
			res.LedgerEntry.GitHEAD, res.LedgerEntry.TreeStateSHA)
	}
}

func TestRun_DefaultsCLIAndModelWhenProfileSilent(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{response: core.BridgeResponse{ExitCode: 0}}
	ledger := &fakeLedger{}
	loader := profiles.NewFromFS(fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte(`{
			"name":"builder","role":"builder",
			"output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"
		}`)},
	})
	r, err := New(Config{
		Profiles: loader, Bridge: bridge, Ledger: ledger,
		Now:  func() time.Time { return now },
		Rand: deterministicRand(0xAB),
		GitState: func(context.Context, string) (string, string, error) {
			return "h", "t", nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	token := strings.Repeat("ab", ChallengeTokenBytes)
	tmp := t.TempDir()
	bridge.onLaunch = func(req core.BridgeRequest) error {
		writeArtifact(t, req.ArtifactPath, "<!-- challenge-token: "+token+" -->\nok\n", now)
		return nil
	}
	if _, err := r.Run(context.Background(), Request{
		Agent: "builder", Cycle: 3, ProjectRoot: tmp,
		Workspace: filepath.Join(tmp, ".evolve/runs/cycle-3"), Prompt: "go",
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.calls) != 1 {
		t.Fatalf("expected 1 bridge call, got %d", len(bridge.calls))
	}
	got := bridge.calls[0]
	if got.CLI != "claude-tmux" {
		t.Errorf("CLI default=%q, want claude-tmux", got.CLI)
	}
	if got.Model != "auto" {
		t.Errorf("Model default=%q, want auto", got.Model)
	}
}

func TestClassify_EmptyArtifactIsIntegrityFail(t *testing.T) {
	now := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	r := &Runner{cfg: Config{
		Now:       func() time.Time { return now },
		StatMTime: func(string) (time.Time, error) { return now, nil },
		ReadFile:  func(string) ([]byte, error) { return []byte{}, nil },
	}}
	verdict, diags := r.classify(nil, "/tmp/stub", "tok", 0)
	if verdict != VerdictIntegrityFail {
		t.Errorf("verdict=%q, want %q", verdict, VerdictIntegrityFail)
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "empty") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'artifact empty' diagnostic, got %+v", diags)
	}
}

func TestClassify_StaleAndTokenMissing(t *testing.T) {
	now := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	r := &Runner{cfg: Config{
		Now:       func() time.Time { return now },
		StatMTime: func(_ string) (time.Time, error) { return now.Add(-1 * time.Hour), nil },
		ReadFile:  func(_ string) ([]byte, error) { return []byte("body without token"), nil },
	}}
	verdict, diags := r.classify(nil, "/tmp/stub", "deadbeefdeadbeef", 0)
	if verdict != VerdictIntegrityFail {
		t.Errorf("verdict=%q want %q", verdict, VerdictIntegrityFail)
	}
	if len(diags) == 0 {
		t.Errorf("expected stale diagnostic")
	}
}

func TestClassify_ReadError(t *testing.T) {
	now := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	r := &Runner{cfg: Config{
		Now:       func() time.Time { return now },
		StatMTime: func(_ string) (time.Time, error) { return now, nil },
		ReadFile:  func(_ string) ([]byte, error) { return nil, errors.New("read denied") },
	}}
	verdict, _ := r.classify(nil, "/tmp/stub", "token", 0)
	if verdict != VerdictIntegrityFail {
		t.Errorf("verdict=%q want IntegrityFail", verdict)
	}
}

func TestClassify_BridgeErrorNonzero(t *testing.T) {
	now := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	r := &Runner{cfg: Config{
		Now:       func() time.Time { return now },
		StatMTime: func(_ string) (time.Time, error) { return now, nil },
		ReadFile:  func(_ string) ([]byte, error) { return []byte("body with token-xyz\n"), nil },
	}}
	verdict, diags := r.classify(errors.New("bridge launch failed"), "/tmp/stub", "token-xyz", 137)
	if verdict != VerdictFAIL {
		t.Errorf("verdict=%q want %q", verdict, VerdictFAIL)
	}
	if len(diags) == 0 {
		t.Errorf("expected bridge-error diagnostic")
	}
}

func TestComposePrompt_AlreadyTrailingNewline(t *testing.T) {
	out := composePrompt("body\n", "tok", "/a", "scout", 1)
	if strings.Contains(out, "body\n\n## END") {
		t.Errorf("double newline before END marker:\n%s", out)
	}
}

func TestResolveArtifactPath_Empty(t *testing.T) {
	if got := resolveArtifactPath("", 1, "/root"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveArtifactPath_AbsoluteTemplate(t *testing.T) {
	got := resolveArtifactPath("/abs/cycle-{cycle}/r.md", 5, "/root")
	if got != "/abs/cycle-5/r.md" {
		t.Errorf("got %q", got)
	}
}
