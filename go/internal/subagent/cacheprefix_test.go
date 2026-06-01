package subagent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractGoalLine(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"empty body", "", ""},
		{"no goal line", "no relevant content here\nanother line\n", ""},
		{"simple goal", "goal: improve dispatch reliability\n", "improve dispatch reliability"},
		{"tab indent after goal:", "goal:\tport remaining bash to Go\n", "port remaining bash to Go"},
		{"goal mid-document, first wins", "header\ngoal: first match\ngoal: second match\n", "first match"},
		{"surrounding whitespace preserved in body, trimmed only at EOL", "goal:   spaces preserved internally   \n", "spaces preserved internally   "},
		{"goal: in nested line ignored", "context:\n  goal: not at line start\n", ""},
		{"CRLF line endings", "header\r\ngoal: windows file\r\nrest\r\n", "windows file"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractGoalLine(tc.body)
			if got != tc.want {
				t.Fatalf("extractGoalLine(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestSummarizeCycleState(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "empty body — all defaults",
			body: "",
			want: "phase=unknown active_agent=none completed_phases=[]",
		},
		{
			name: "full triple",
			body: `{"phase":"scout","active_agent":"scout","completed_phases":["intent"]}`,
			want: "phase=scout active_agent=scout completed_phases=[intent]",
		},
		{
			name: "completed_phases multi-element with whitespace",
			body: `{"phase":"build","active_agent":"builder","completed_phases":["intent", "scout" , "tdd-engineer"]}`,
			want: "phase=build active_agent=builder completed_phases=[intent,scout,tdd-engineer]",
		},
		{
			name: "missing active_agent → default none",
			body: `{"phase":"audit","completed_phases":[]}`,
			want: "phase=audit active_agent=none completed_phases=[]",
		},
		{
			name: "missing phase → default unknown",
			body: `{"active_agent":"auditor","completed_phases":["build"]}`,
			want: "phase=unknown active_agent=auditor completed_phases=[build]",
		},
		{
			name: "completed_phases without quotes ignored (defensive)",
			body: `{"phase":"x","active_agent":"y","completed_phases":[]}`,
			want: "phase=x active_agent=y completed_phases=[]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeCycleState(tc.body)
			if got != tc.want {
				t.Fatalf("summarizeCycleState() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteCachePrefix_ByteDeterministicAcrossInvocations(t *testing.T) {
	// Same inputs → byte-identical output (the whole point of cache-prefix).
	tmp := t.TempDir()
	out1 := filepath.Join(tmp, "p1.md")
	out2 := filepath.Join(tmp, "p2.md")
	req := CachePrefixRequest{
		Cycle:       42,
		Agent:       "scout",
		Workspace:   "/tmp/cycle-42",
		ProjectRoot: tmp,
	}
	req.OutPath = out1
	if err := WriteCachePrefix(req, CachePrefixOptions{
		ReadOrchestratorPrompt: func(string) (string, error) {
			return "header\ngoal: deterministic test\n", nil
		},
		ReadCycleState: func(string) (string, error) {
			return `{"phase":"scout","active_agent":"scout","completed_phases":["intent"]}`, nil
		},
	}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	req.OutPath = out2
	if err := WriteCachePrefix(req, CachePrefixOptions{
		ReadOrchestratorPrompt: func(string) (string, error) {
			return "header\ngoal: deterministic test\n", nil
		},
		ReadCycleState: func(string) (string, error) {
			return `{"phase":"scout","active_agent":"scout","completed_phases":["intent"]}`, nil
		},
	}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	b1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatalf("read out1: %v", err)
	}
	b2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatalf("read out2: %v", err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("non-deterministic output\nfirst:\n%s\nsecond:\n%s", b1, b2)
	}

	body := string(b1)
	// Spot-check key invariants the cache relies on.
	if !strings.Contains(body, "<!-- cache-prefix v8.23.0") {
		t.Errorf("missing header sentinel")
	}
	if !strings.Contains(body, "agent=scout cycle=42 workspace=/tmp/cycle-42") {
		t.Errorf("missing per-invocation metadata: %s", body)
	}
	if !strings.Contains(body, "# Shared Context for Cycle 42 — scout phase") {
		t.Errorf("missing H1 frame")
	}
	if !strings.Contains(body, "deterministic test") {
		t.Errorf("goal not propagated")
	}
	if !strings.Contains(body, "phase=scout active_agent=scout completed_phases=[intent]") {
		t.Errorf("cycle-state summary not embedded")
	}
	if !strings.HasSuffix(body, "<!-- end cache-prefix -->\n") {
		t.Errorf("missing trailer")
	}
}

func TestWriteCachePrefix_DefaultsWhenSeamsAreNil(t *testing.T) {
	// nil seams → uses defaultReadOrchestratorPrompt + defaultReadCycleState.
	// Both files are absent, so we expect the (no goal extracted) +
	// (cycle-state unavailable) fallback markers.
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	out := filepath.Join(tmp, "out", "cache.md")
	err := WriteCachePrefix(CachePrefixRequest{
		Cycle:       7,
		Agent:       "auditor",
		Workspace:   workspace,
		ProjectRoot: tmp,
		OutPath:     out,
	}, CachePrefixOptions{})
	if err != nil {
		t.Fatalf("WriteCachePrefix: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "(no goal extracted)") {
		t.Errorf("expected goal fallback marker: %s", body)
	}
	if !strings.Contains(string(body), "(cycle-state unavailable)") {
		t.Errorf("expected cycle-state fallback marker: %s", body)
	}
}

func TestWriteCachePrefix_DefaultsReadRealFiles(t *testing.T) {
	// Exercises defaultReadOrchestratorPrompt + defaultReadCycleState on the
	// real filesystem.
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workspace, "orchestrator-prompt.md"),
		[]byte("goal: real-fs path\n"), 0o644,
	); err != nil {
		t.Fatalf("write orchestrator-prompt: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(tmp, ".evolve", "cycle-state.json"),
		[]byte(`{"phase":"build","active_agent":"builder","completed_phases":["scout"]}`),
		0o644,
	); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}

	out := filepath.Join(tmp, "out.md")
	if err := WriteCachePrefix(CachePrefixRequest{
		Cycle:       99,
		Agent:       "builder",
		Workspace:   workspace,
		ProjectRoot: tmp,
		OutPath:     out,
	}, CachePrefixOptions{}); err != nil {
		t.Fatalf("WriteCachePrefix: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "real-fs path") {
		t.Errorf("goal from disk not propagated: %s", body)
	}
	if !strings.Contains(string(body), "phase=build active_agent=builder completed_phases=[scout]") {
		t.Errorf("cycle-state from disk not propagated: %s", body)
	}
}

func TestWriteCachePrefix_SeamErrorsFallThroughGracefully(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "out.md")
	if err := WriteCachePrefix(CachePrefixRequest{
		Cycle:       1,
		Agent:       "tdd-engineer",
		Workspace:   "/nonexistent",
		ProjectRoot: tmp,
		OutPath:     out,
	}, CachePrefixOptions{
		ReadOrchestratorPrompt: func(string) (string, error) { return "", errors.New("no prompt") },
		ReadCycleState:         func(string) (string, error) { return "", errors.New("no state") },
	}); err != nil {
		t.Fatalf("WriteCachePrefix: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "(no goal extracted)") {
		t.Errorf("expected goal fallback on seam error")
	}
	if !strings.Contains(string(body), "(cycle-state unavailable)") {
		t.Errorf("expected state fallback on seam error")
	}
}

func TestWriteCachePrefix_MkdirError(t *testing.T) {
	// Force os.MkdirAll to fail by pointing OutPath into an unwritable parent.
	tmp := t.TempDir()
	bad := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(bad, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	out := filepath.Join(bad, "sub", "cache.md") // bad is a file, so MkdirAll fails
	err := WriteCachePrefix(CachePrefixRequest{
		Cycle: 0, Agent: "x", Workspace: tmp, ProjectRoot: tmp, OutPath: out,
	}, CachePrefixOptions{
		ReadOrchestratorPrompt: func(string) (string, error) { return "", os.ErrNotExist },
		ReadCycleState:         func(string) (string, error) { return "", os.ErrNotExist },
	})
	if err == nil {
		t.Fatalf("expected mkdir error, got nil")
	}
	if !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("expected mkdir error, got: %v", err)
	}
}

// failingWriter returns an error after writeCount successful writes, used to
// drive the io.WriteString error branch in renderCachePrefix.
type failingWriter struct {
	failAfter int
	writes    int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errors.New("disk full")
	}
	w.writes++
	return len(p), nil
}

// TestRenderCachePrefix_WriteError covers the io.WriteString failure branch
// (cacheprefix.go:102) by injecting a writer that fails on the first part.
func TestRenderCachePrefix_WriteError(t *testing.T) {
	err := renderCachePrefix(&failingWriter{failAfter: 0}, CachePrefixRequest{
		Cycle: 1, Agent: "scout", Workspace: "/ws",
	}, "goal", "summary")
	if err == nil {
		t.Fatalf("expected write error")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Errorf("expected write error, got %v", err)
	}
}

// TestRenderCachePrefix_WriteErrorMidStream covers the failure occurring on a
// later part rather than the first, confirming the error short-circuits the
// loop wherever it fires.
func TestRenderCachePrefix_WriteErrorMidStream(t *testing.T) {
	err := renderCachePrefix(&failingWriter{failAfter: 3}, CachePrefixRequest{
		Cycle: 2, Agent: "auditor", Workspace: "/ws",
	}, "g", "s")
	if err == nil {
		t.Fatalf("expected mid-stream write error")
	}
}

// TestWriteCachePrefix_CreateError covers the os.Create failure branch
// (cacheprefix.go:68) — OutPath points at an existing directory, so Create
// fails even though MkdirAll of its parent succeeds.
func TestWriteCachePrefix_CreateError(t *testing.T) {
	tmp := t.TempDir()
	// OutPath is the tmp dir itself; filepath.Dir(tmp) exists so MkdirAll
	// succeeds, but os.Create on a directory path fails.
	err := WriteCachePrefix(CachePrefixRequest{
		Cycle: 0, Agent: "x", Workspace: tmp, ProjectRoot: tmp, OutPath: tmp,
	}, CachePrefixOptions{
		ReadOrchestratorPrompt: func(string) (string, error) { return "", os.ErrNotExist },
		ReadCycleState:         func(string) (string, error) { return "", os.ErrNotExist },
	})
	if err == nil {
		t.Fatalf("expected create error for directory OutPath")
	}
	if !strings.Contains(err.Error(), "create") {
		t.Errorf("expected create error, got %v", err)
	}
}

func TestDefaultReadersReturnErrorForMissingFiles(t *testing.T) {
	tmp := t.TempDir()
	if _, err := defaultReadOrchestratorPrompt(tmp); err == nil {
		t.Errorf("expected error for missing orchestrator-prompt.md")
	}
	if _, err := defaultReadCycleState(tmp); err == nil {
		t.Errorf("expected error for missing cycle-state.json")
	}
}
