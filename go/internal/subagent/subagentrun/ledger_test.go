package subagentrun

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ledger_test.go — the host's ten direct ledger-writer tests moved (ADR-0103
// unit 16 D5) with their callee adapted from writeSubprocessLedger(path, e,
// now) error to the dispatcher-owned appendLedger(path, e) (op, err): the
// clock is the dispatcher's, the op is asserted.

func fixedNowFn() func() time.Time {
	return func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }
}

// writer is a dispatcher whose only live collaborators are the clock and the
// ledger opener.
func writer(t *testing.T, opts ...Option) *Dispatcher {
	t.Helper()
	return New(Deps{}, append([]Option{WithClock(fixedNowFn())}, opts...)...)
}

func writeAndRead(t *testing.T, e ledgerEntry) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	if op, err := writer(t).appendLedger(p, e); err != nil {
		t.Fatalf("appendLedger %s: %v", op, err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// Test 45 — the line, the tip and the chain are byte-identical to the goldens
// captured on the pre-extraction writer; the run id rides as an argument
// (format verbs stay literal), is omitted when empty and decodes for the
// binding readers.
func TestLedger_LineGoldenChainAndRunID(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.jsonl")
	d := New(Deps{}, WithClock(clock))
	e := ledgerEntry{Cycle: 9, Role: "scout", Model: "sonnet", ExitCode: 0, DurationS: "12", ArtifactPath: "/a.md", ArtifactSHA256: "deadbeef", ChallengeToken: "tok", GitHEAD: "hhhh", TreeStateSHA: "tttt", QualityTier: "full"}
	if op, err := d.appendLedger(ledger, e); err != nil {
		t.Fatalf("%s: %v", op, err)
	}
	wantLines := strings.SplitAfter(golden(t, "ledger-line.golden.jsonl"), "\n")
	wantTips := strings.SplitAfter(golden(t, "ledger-tip.golden.txt"), "\n")
	got, _ := os.ReadFile(ledger)
	tip, _ := os.ReadFile(filepath.Join(dir, "ledger.tip"))
	if string(got) != wantLines[0] || string(tip) != wantTips[0] || string(tip) != "0:"+SHA256Hex(strings.TrimSuffix(wantLines[0], "\n"))+"\n" {
		t.Fatalf("first line/tip drifted:\n%s%s", got, tip)
	}
	e.RunID = "01M09657TDN6Q1VMJK1XKYR376"
	e.Cycle, e.ExitCode, e.DurationS = 10, 1, "0"
	if op, err := d.appendLedger(ledger, e); err != nil {
		t.Fatalf("%s: %v", op, err)
	}
	got, _ = os.ReadFile(ledger)
	tip, _ = os.ReadFile(filepath.Join(dir, "ledger.tip"))
	if string(got) != wantLines[0]+wantLines[1] || string(tip) != wantTips[1] {
		t.Fatalf("second line/tip (the chain) drifted:\n%s%s", got, tip)
	}
	if renderLedgerLine(e, "2026-05-23T17:00:00Z", 1, SHA256Hex(strings.TrimSuffix(wantLines[0], "\n"))) != strings.TrimSuffix(wantLines[1], "\n") {
		t.Fatal("renderLedgerLine is the pure line")
	}

	// The host's runid_stamp_test.go intents.
	line := writeAndRead(t, ledgerEntry{Cycle: 1519, Role: "auditor", ExitCode: 0, RunID: "01M09657TDN6Q1VMJK1XKYR376"})
	if !strings.Contains(line, `"run_id":"01M09657TDN6Q1VMJK1XKYR376"`) {
		t.Errorf("ledger line carries no run_id — a run-scoped binding lookup can never match it.\nline: %s", line)
	}
	var decoded struct {
		Kind  string `json:"kind"`
		Role  string `json:"role"`
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(line), &decoded); err != nil || decoded.Kind != "agent_subprocess" || decoded.Role != "auditor" || decoded.RunID != "01M09657TDN6Q1VMJK1XKYR376" {
		t.Errorf("the binding readers' wire contract: %v %+v", err, decoded)
	}
	line = writeAndRead(t, ledgerEntry{Cycle: 1519, Role: "auditor"})
	var probe map[string]any
	if strings.Contains(line, "run_id") || json.Unmarshal([]byte(line), &probe) != nil {
		t.Errorf("an unresolved run id must OMIT the key (omitempty parity): %s", line)
	}
	for _, runID := range []string{`run%s-weird`, `run%d`, `a"b`, `100%`, `%!x(MISSING)`} {
		line := writeAndRead(t, ledgerEntry{Cycle: 1519, Role: "auditor", Model: "opus", ExitCode: 0, RunID: runID})
		var got struct {
			Role  string `json:"role"`
			Model string `json:"model"`
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil || got.RunID != runID || got.Role != "auditor" || got.Model != "opus" {
			t.Errorf("run_id %q corrupted or shifted the line: %v %+v\nline: %s", runID, err, got, line)
		}
	}
}

// failingWriter is an io.WriteCloser double whose write or close fails.
type failingWriter struct{ write, close error }

func (w failingWriter) Write(p []byte) (int, error) {
	if w.write != nil {
		return 0, w.write
	}
	return len(p), nil
}
func (w failingWriter) Close() error { return w.close }

// Test 46 — every failing op returns its BARE error with the op named
// (the host's five writer tests as subtests, plus write and close through an
// injected opener); the tip is written after the line.
func TestLedger_EveryOpFailureIsBareTextWithTheOpInFields(t *testing.T) {
	entry := ledgerEntry{Cycle: 1, Role: "scout", Model: "sonnet", ChallengeToken: "tok"}
	cases := []struct {
		name, op string
		setup    func(t *testing.T, dir string) (path string, opts []Option)
	}{
		{"mkdir", "mkdir", func(t *testing.T, dir string) (string, []Option) {
			blocker := filepath.Join(dir, "blocker")
			_ = os.WriteFile(blocker, []byte("x"), 0o644)
			return filepath.Join(blocker, "sub", "ledger.jsonl"), nil
		}},
		{"chain link (a directory at the ledger)", "chain_link", func(t *testing.T, dir string) (string, []Option) {
			p := filepath.Join(dir, "ledger.jsonl")
			_ = os.Mkdir(p, 0o755)
			return p, nil
		}},
		{"open (a read-only ledger)", "open", func(t *testing.T, dir string) (string, []Option) {
			p := filepath.Join(dir, "ledger.jsonl")
			_ = os.WriteFile(p, []byte("seed\n"), 0o444)
			t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
			return p, nil
		}},
		{"write", "write", func(t *testing.T, dir string) (string, []Option) {
			return filepath.Join(dir, "ledger.jsonl"), []Option{WithLedgerOpener(func(string) (io.WriteCloser, error) { return failingWriter{write: errors.New("disk full")}, nil })}
		}},
		{"close", "close", func(t *testing.T, dir string) (string, []Option) {
			return filepath.Join(dir, "ledger.jsonl"), []Option{WithLedgerOpener(func(string) (io.WriteCloser, error) { return failingWriter{close: errors.New("close failed")}, nil })}
		}},
		{"tip tmp (a directory at ledger.tip.tmp)", "tip_tmp", func(t *testing.T, dir string) (string, []Option) {
			_ = os.Mkdir(filepath.Join(dir, "ledger.tip.tmp"), 0o755)
			return filepath.Join(dir, "ledger.jsonl"), nil
		}},
		{"tip rename (a directory at ledger.tip)", "tip_rename", func(t *testing.T, dir string) (string, []Option) {
			_ = os.Mkdir(filepath.Join(dir, "ledger.tip"), 0o755)
			return filepath.Join(dir, "ledger.jsonl"), nil
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path, opts := c.setup(t, dir)
			op, err := writer(t, opts...).appendLedger(path, entry)
			if err == nil || op != c.op || strings.Contains(err.Error(), "subagent") || strings.Contains(err.Error(), c.op+":") {
				t.Fatalf("op %q err %v (bare, the op in the return, never in the text)", op, err)
			}
			if c.op == "tip_tmp" || c.op == "tip_rename" {
				if b, _ := os.ReadFile(path); !strings.Contains(string(b), `"kind":"agent_subprocess"`) {
					t.Fatalf("the line is written before the tip: %s", b)
				}
			}
		})
	}
	// The wall-clock intent of the host's nil-clock test: no WithClock ⇒ a
	// parseable timestamp of today.
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	if op, err := New(Deps{}).appendLedger(p, entry); err != nil {
		t.Fatalf("%s: %v", op, err)
	}
	b, _ := os.ReadFile(p)
	var got struct {
		TS string `json:"ts"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(b))), &got); err != nil {
		t.Fatal(err)
	}
	if ts, err := time.Parse("2006-01-02T15:04:05Z", got.TS); err != nil || time.Since(ts) > time.Hour {
		t.Fatalf("the default clock is the wall clock: %q %v", got.TS, err)
	}
}

// Test 47 — the rendered key set never carries worktree_tree_sha (the
// release preflight's negative contract), whatever the field permutation.
func TestLedger_NeverCarriesWorktreeTreeSHA(t *testing.T) {
	for _, e := range []ledgerEntry{{}, {RunID: "r"}, {Cycle: 1, Role: "scout", Model: "m", ExitCode: 1, DurationS: "1", ArtifactPath: "/a", ArtifactSHA256: "s", ChallengeToken: "t", GitHEAD: "h", TreeStateSHA: "t", QualityTier: "full", RunID: "r"}} {
		line := renderLedgerLine(e, "2026-05-23T17:00:00Z", 0, LedgerZeroSeed)
		var keys map[string]any
		if err := json.Unmarshal([]byte(line), &keys); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		if _, has := keys["worktree_tree_sha"]; has || keys["cli_resolution"] != nil || keys["kind"] != "agent_subprocess" {
			t.Fatalf("%s", line)
		}
	}
}

// Test 48 — the pure helpers: the quality tier, the escaper, the hash, the
// chain link's seed / empty / last-line / seq.
func TestQualityTier(t *testing.T) {
	tests := []struct {
		bn, ps bool
		want   string
	}{
		{true, true, "full"},
		{false, false, "degraded"},
		{true, false, "hybrid"},
		{false, true, "hybrid"},
	}
	for _, tc := range tests {
		if got := QualityTier(tc.bn, tc.ps); got != tc.want {
			t.Errorf("(%v,%v) → %s, want %s", tc.bn, tc.ps, got, tc.want)
		}
	}
}

func TestJSONStringEscape(t *testing.T) {
	for in, want := range map[string]string{`a"b`: `a\"b`, `a\b`: `a\\b`, "a\nb": `a\nb`, "a\rb": `a\rb`, "a\tb": `a\tb`, "plain": "plain"} {
		if got := JSONStringEscape(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

func TestSHA256Hex(t *testing.T) {
	if SHA256Hex("") != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Error("sha256 of the empty string")
	}
}

func TestChainLink_SeedEmptyLastLineSeq(t *testing.T) {
	dir := t.TempDir()
	if prev, seq, err := ChainLink(filepath.Join(dir, "missing.jsonl")); err != nil || prev != LedgerZeroSeed || seq != 0 {
		t.Fatalf("absent: %q %d %v", prev, seq, err)
	}
	p := filepath.Join(dir, "ledger.jsonl")
	_ = os.WriteFile(p, nil, 0o644)
	if prev, seq, err := ChainLink(p); err != nil || prev != LedgerZeroSeed || seq != 0 {
		t.Fatalf("empty: %q %d %v", prev, seq, err)
	}
	_ = os.WriteFile(p, []byte("\n"), 0o644)
	if prev, seq, err := ChainLink(p); err != nil || prev != LedgerZeroSeed || seq != 0 {
		t.Fatalf("a lone newline: %q %d %v", prev, seq, err)
	}
	_ = os.WriteFile(p, []byte("first entry\nsecond entry\n"), 0o644)
	if prev, seq, err := ChainLink(p); err != nil || prev != SHA256Hex("second entry") || seq != 2 {
		t.Fatalf("two lines: %q %d %v", prev, seq, err)
	}
	if _, _, err := ChainLink(dir); err == nil {
		t.Fatal("a directory at the ledger errs")
	}
}
