package subagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// --- Test doubles ----------------------------------------------------

type fakeBridge struct {
	mu        sync.Mutex
	calls     []core.BridgeRequest
	response  core.BridgeResponse
	err       error
	onLaunch  func(core.BridgeRequest) error // optional pre-launch hook
}

func (f *fakeBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	hook := f.onLaunch
	f.mu.Unlock()
	if hook != nil {
		if err := hook(req); err != nil {
			return f.response, err
		}
	}
	return f.response, f.err
}

func (f *fakeBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

type fakeLedger struct {
	mu      sync.Mutex
	entries []core.LedgerEntry
	err     error
}

func (f *fakeLedger) Append(_ context.Context, e core.LedgerEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.entries = append(f.entries, e)
	return nil
}

func (f *fakeLedger) Verify(_ context.Context) error                   { return nil }
func (f *fakeLedger) Iter(_ context.Context) (core.LedgerIterator, error) { return nil, errors.New("not impl") }

// --- Helpers ---------------------------------------------------------

// writeArtifact materializes a file at path with the given token-bearing
// body and pins its mtime to mtime. The bridge hook uses this to simulate
// a working subagent under a fake clock — without the chtimes() call the
// file's real-wall mtime is compared against the fake Now() and may
// register as "stale" or "from the future".
func writeArtifact(t *testing.T, path, body string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// newRunner wires a Runner with the given doubles + a profile loader
// backed by an in-memory FS containing a "builder" profile.
func newRunner(t *testing.T, bridge core.Bridge, ledger core.Ledger, now time.Time) *Runner {
	t.Helper()
	fsys := fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte(`{
			"name": "builder",
			"role": "builder",
			"cli": "claude-p",
			"model_tier_default": "sonnet",
			"output_artifact": ".evolve/runs/cycle-{cycle}/build-report.md"
		}`)},
		"noartifact.json": &fstest.MapFile{Data: []byte(`{
			"name": "noartifact",
			"role": "builder",
			"cli": "claude-p",
			"model_tier_default": "sonnet"
		}`)},
	}
	loader := profiles.NewFromFS(fsys)
	r, err := New(Config{
		Profiles: loader,
		Bridge:   bridge,
		Ledger:   ledger,
		Now:      func() time.Time { return now },
		Rand:     deterministicRand(0xAB),
		GitState: func(_ context.Context, _ string) (string, string, error) {
			return "abc123head", "def456tree", nil
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func deterministicRand(b byte) func([]byte) (int, error) {
	return func(buf []byte) (int, error) {
		for i := range buf {
			buf[i] = b
		}
		return len(buf), nil
	}
}

// --- Tests -----------------------------------------------------------

func TestRun_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{response: core.BridgeResponse{ExitCode: 0, CostUSD: 0.05}}
	ledger := &fakeLedger{}

	r := newRunner(t, bridge, ledger, now)
	expectedToken := strings.Repeat("ab", ChallengeTokenBytes) // 16-hex from deterministicRand(0xAB)
	expectedArtifact := filepath.Join(tmp, ".evolve/runs/cycle-3/build-report.md")

	// Simulate the agent writing the artifact with the token embedded.
	bridge.onLaunch = func(req core.BridgeRequest) error {
		writeArtifact(t, req.ArtifactPath, "<!-- challenge-token: "+expectedToken+" -->\nbuild ok\n", now)
		return nil
	}

	res, err := r.Run(context.Background(), Request{
		Agent:       "builder",
		Cycle:       3,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, ".evolve/runs/cycle-3"),
		Prompt:      "do the build",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Verdict != VerdictPASS {
		t.Errorf("verdict = %q, want PASS; diags=%v", res.Verdict, res.Diagnostics)
	}
	if res.ChallengeToken != expectedToken {
		t.Errorf("token = %q, want %q", res.ChallengeToken, expectedToken)
	}
	if res.ArtifactPath != expectedArtifact {
		t.Errorf("artifact = %q, want %q", res.ArtifactPath, expectedArtifact)
	}
	if res.ArtifactSHA256 == "" {
		t.Errorf("artifact sha empty")
	}
	if len(bridge.calls) != 1 {
		t.Fatalf("bridge calls = %d, want 1", len(bridge.calls))
	}
	br := bridge.calls[0]
	if !strings.Contains(br.Prompt, expectedToken) {
		t.Errorf("prompt missing token: %s", br.Prompt)
	}
	if !strings.Contains(br.Prompt, "do the build") {
		t.Errorf("prompt missing user body")
	}
	if br.CLI != "claude-p" || br.Model != "sonnet" {
		t.Errorf("bridge CLI/model wrong: cli=%s model=%s", br.CLI, br.Model)
	}

	if len(ledger.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(ledger.entries))
	}
	e := ledger.entries[0]
	if e.Kind != "agent_subprocess" {
		t.Errorf("kind = %q, want agent_subprocess", e.Kind)
	}
	if e.Role != "builder" {
		t.Errorf("role = %q, want builder", e.Role)
	}
	if e.Cycle != 3 {
		t.Errorf("cycle = %d, want 3", e.Cycle)
	}
	if e.GitHEAD != "abc123head" || e.TreeStateSHA != "def456tree" {
		t.Errorf("git state not propagated: %+v", e)
	}
	if e.ChallengeToken != expectedToken {
		t.Errorf("ledger token mismatch")
	}
}

func TestRun_ArtifactMissing(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{response: core.BridgeResponse{ExitCode: 0}}
	ledger := &fakeLedger{}
	r := newRunner(t, bridge, ledger, now)

	// Bridge "succeeds" but writes no artifact.
	res, err := r.Run(context.Background(), Request{
		Agent:       "builder",
		Cycle:       1,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, ".evolve/runs/cycle-1"),
		Prompt:      "missing-artifact case",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Verdict != VerdictIntegrityFail {
		t.Errorf("verdict = %q, want INTEGRITY_FAIL", res.Verdict)
	}
	if len(res.Diagnostics) == 0 || !strings.Contains(res.Diagnostics[0].Message, "missing") {
		t.Errorf("expected missing-artifact diagnostic, got %+v", res.Diagnostics)
	}
	if len(ledger.entries) != 1 {
		t.Errorf("ledger entry should still be appended for forensics")
	}
}

func TestRun_ArtifactStale(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{response: core.BridgeResponse{ExitCode: 0}}
	ledger := &fakeLedger{}
	r := newRunner(t, bridge, ledger, now)

	expectedToken := strings.Repeat("ab", ChallengeTokenBytes)
	bridge.onLaunch = func(req core.BridgeRequest) error {
		// Backdate by 10 minutes — past ArtifactMaxAge (5 minutes).
		writeArtifact(t, req.ArtifactPath, "<!-- challenge-token: "+expectedToken+" -->\nstale\n", now.Add(-10*time.Minute))
		return nil
	}

	res, err := r.Run(context.Background(), Request{
		Agent:       "builder",
		Cycle:       1,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, ".evolve/runs/cycle-1"),
		Prompt:      "stale case",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Verdict != VerdictIntegrityFail {
		t.Errorf("verdict = %q, want INTEGRITY_FAIL", res.Verdict)
	}
	if len(res.Diagnostics) == 0 || !strings.Contains(res.Diagnostics[0].Message, "stale") {
		t.Errorf("expected stale diagnostic, got %+v", res.Diagnostics)
	}
}

func TestRun_TokenMissing(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{response: core.BridgeResponse{ExitCode: 0}}
	ledger := &fakeLedger{}
	r := newRunner(t, bridge, ledger, now)

	bridge.onLaunch = func(req core.BridgeRequest) error {
		// Write an artifact WITHOUT the challenge token; fresh mtime.
		writeArtifact(t, req.ArtifactPath, "build report without provenance proof\n", now)
		return nil
	}

	res, err := r.Run(context.Background(), Request{
		Agent:       "builder",
		Cycle:       1,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, ".evolve/runs/cycle-1"),
		Prompt:      "forged case",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Verdict != VerdictIntegrityFail {
		t.Errorf("verdict = %q, want INTEGRITY_FAIL", res.Verdict)
	}
	if len(res.Diagnostics) == 0 || !strings.Contains(res.Diagnostics[0].Message, "challenge token") {
		t.Errorf("expected token-missing diagnostic, got %+v", res.Diagnostics)
	}
}

func TestRun_BridgeError(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{
		response: core.BridgeResponse{ExitCode: 1},
		err:      errors.New("bridge: launch exit=1"),
	}
	ledger := &fakeLedger{}
	r := newRunner(t, bridge, ledger, now)

	res, err := r.Run(context.Background(), Request{
		Agent:       "builder",
		Cycle:       1,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, ".evolve/runs/cycle-1"),
		Prompt:      "bridge-error case",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if res.Verdict != VerdictIntegrityFail {
		// Bridge failed AND no artifact written → integrity fail dominates.
		t.Errorf("verdict = %q, want INTEGRITY_FAIL", res.Verdict)
	}
	// Ledger entry must still be appended so we have a trail.
	if len(ledger.entries) != 1 {
		t.Errorf("ledger entries = %d, want 1", len(ledger.entries))
	}
	if ledger.entries[0].ExitCode != 1 {
		t.Errorf("ledger exit_code = %d, want 1", ledger.entries[0].ExitCode)
	}
}

func TestRun_ProfileMissing(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	r := newRunner(t, &fakeBridge{}, &fakeLedger{}, now)
	_, err := r.Run(context.Background(), Request{
		Agent:       "nonexistent",
		Cycle:       1,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, "ws"),
		Prompt:      "x",
	})
	if err == nil || !strings.Contains(err.Error(), "load profile") {
		t.Errorf("expected load-profile error, got %v", err)
	}
}

func TestRun_ProfileNoArtifact(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	r := newRunner(t, &fakeBridge{}, &fakeLedger{}, now)
	_, err := r.Run(context.Background(), Request{
		Agent:       "noartifact",
		Cycle:       1,
		ProjectRoot: tmp,
		Workspace:   filepath.Join(tmp, "ws"),
		Prompt:      "x",
	})
	if err == nil || !strings.Contains(err.Error(), "output_artifact") {
		t.Errorf("expected output_artifact error, got %v", err)
	}
}

func TestRun_RequestValidation(t *testing.T) {
	r := newRunner(t, &fakeBridge{}, &fakeLedger{}, time.Now())
	cases := []struct {
		name string
		req  Request
		want string
	}{
		{"missing agent", Request{Cycle: 1, ProjectRoot: "/p", Workspace: "/w", Prompt: "x"}, "Agent required"},
		{"missing project root", Request{Agent: "builder", Workspace: "/w", Prompt: "x"}, "ProjectRoot required"},
		{"missing workspace", Request{Agent: "builder", ProjectRoot: "/p", Prompt: "x"}, "Workspace required"},
		{"missing prompt", Request{Agent: "builder", ProjectRoot: "/p", Workspace: "/w"}, "Prompt required"},
		{"negative cycle", Request{Agent: "builder", ProjectRoot: "/p", Workspace: "/w", Prompt: "x", Cycle: -1}, "Cycle must be >= 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Run(context.Background(), tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestNew_RequiredDeps(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"no profiles", Config{Bridge: &fakeBridge{}, Ledger: &fakeLedger{}}, "Profiles required"},
		{"no bridge", Config{Profiles: profiles.NewFromFS(fstest.MapFS{}), Ledger: &fakeLedger{}}, "Bridge required"},
		{"no ledger", Config{Profiles: profiles.NewFromFS(fstest.MapFS{}), Bridge: &fakeBridge{}}, "Ledger required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestResolveArtifactPath(t *testing.T) {
	cases := []struct {
		name, template string
		cycle          int
		projectRoot    string
		want           string
	}{
		{"relative with {cycle}", ".evolve/runs/cycle-{cycle}/build-report.md", 7, "/p", "/p/.evolve/runs/cycle-7/build-report.md"},
		{"absolute template", "/tmp/x-{cycle}.md", 5, "/p", "/tmp/x-5.md"},
		{"empty template", "", 1, "/p", ""},
		{"no placeholder", ".evolve/runs/static/out.md", 9, "/p", "/p/.evolve/runs/static/out.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveArtifactPath(tc.template, tc.cycle, tc.projectRoot)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestComposePrompt(t *testing.T) {
	out := composePrompt("the task", "deadbeefcafef00d", "/tmp/art.md", "builder", 5)
	for _, want := range []string{
		"Agent: builder",
		"Cycle: 5",
		"Challenge token: deadbeefcafef00d",
		"Artifact path: /tmp/art.md",
		"--- BEGIN TASK PROMPT ---",
		"the task",
		"--- END TASK PROMPT ---",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt missing %q\n---\n%s\n---", want, out)
		}
	}
}

func TestGenerateToken_LengthAndAlphabet(t *testing.T) {
	r := newRunner(t, &fakeBridge{}, &fakeLedger{}, time.Now())
	tok, err := r.generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if len(tok) != ChallengeTokenBytes*2 {
		t.Errorf("token len = %d, want %d", len(tok), ChallengeTokenBytes*2)
	}
	for i, c := range tok {
		ok := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !ok {
			t.Errorf("non-hex char %q at index %d", c, i)
		}
	}
}
