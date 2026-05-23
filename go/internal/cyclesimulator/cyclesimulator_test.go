package cyclesimulator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_RejectsBadInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   Inputs
		want int
	}{
		{"non-positive-cycle", Inputs{Cycle: 0, Workspace: "/tmp", ProjectRoot: "/tmp"}, ExitRuntimeErr},
		{"negative-cycle", Inputs{Cycle: -5, Workspace: "/tmp", ProjectRoot: "/tmp"}, ExitRuntimeErr},
		{"no-workspace", Inputs{Cycle: 1, ProjectRoot: "/tmp"}, ExitRuntimeErr},
		{"no-project-root", Inputs{Cycle: 1, Workspace: "/tmp"}, ExitRuntimeErr},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b bytes.Buffer
			if got := Run(tc.in, &b); got != tc.want {
				t.Errorf("got %d, want %d (log=%s)", got, tc.want, b.String())
			}
		})
	}
}

func TestRun_FullPipelineWithStubs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")

	advanceCalls := []string{}
	shipCalled := false
	verifyCalled := false

	rc := Run(Inputs{
		Cycle:       42,
		Workspace:   workspace,
		ProjectRoot: root,
		Now: func() time.Time {
			return time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
		},
		AdvanceFn: func(phase, agent string) error {
			advanceCalls = append(advanceCalls, phase+":"+agent)
			return nil
		},
		ShipDryRunFn: func(msg string) (int, error) {
			shipCalled = true
			return 0, nil
		},
		VerifyFn: func() error {
			verifyCalled = true
			return nil
		},
	}, os.Stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, want 0", rc)
	}
	wantPhases := []string{
		"intent:intent", "research:scout", "build:builder", "audit:auditor",
		"ship:orchestrator", "retrospective:retrospective",
	}
	if len(advanceCalls) != len(wantPhases) {
		t.Fatalf("phase calls: got %v, want %v", advanceCalls, wantPhases)
	}
	for i, want := range wantPhases {
		if advanceCalls[i] != want {
			t.Errorf("phase %d: got %s, want %s", i, advanceCalls[i], want)
		}
	}
	if !shipCalled {
		t.Error("ShipDryRunFn not called")
	}
	if !verifyCalled {
		t.Error("VerifyFn not called")
	}

	// expect 5 artifacts in workspace
	for _, name := range []string{
		"intent.md", "scout-report.md", "build-report.md",
		"audit-report.md", "retrospective-report.md", "simulator-report.md",
	} {
		p := filepath.Join(workspace, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("artifact missing: %s", name)
		}
		// must contain challenge-token
		body, _ := os.ReadFile(p)
		if !strings.Contains(string(body), "challenge-token:") {
			t.Errorf("%s missing challenge-token", name)
		}
	}

	// expect 5 ledger entries (intent, scout, builder, auditor, retrospective)
	ledger := filepath.Join(root, ".evolve", "ledger.jsonl")
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("ledger not written: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("ledger lines: got %d, want 5", len(lines))
	}

	// chain integrity: each line's prev_hash must equal sha256(prev-line)
	for i := 1; i < len(lines); i++ {
		var e map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &e); err != nil {
			t.Fatalf("parse line %d: %v", i, err)
		}
		gotPrev, _ := e["prev_hash"].(string)
		prevSHA := sha256Hex(lines[i-1])
		if gotPrev != prevSHA {
			t.Errorf("chain break line %d: prev_hash=%s, want sha(prev)=%s", i, gotPrev, prevSHA)
		}
	}

	// first entry's prev_hash must be the zero seed
	var first map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &first)
	if first["prev_hash"] != zeroSeed {
		t.Errorf("first prev_hash should be zero seed, got %v", first["prev_hash"])
	}
	if first["simulated"] != true {
		t.Errorf("simulated flag missing on entry 0")
	}
}

func TestRun_AdvanceRefuseReturns2(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rc := Run(Inputs{
		Cycle:        1,
		Workspace:    filepath.Join(root, "ws"),
		ProjectRoot:  root,
		AdvanceFn:    func(phase, agent string) error { return errors.New("gate refused") },
		ShipDryRunFn: func(string) (int, error) { return 0, nil },
		VerifyFn:     func() error { return nil },
	}, os.Stderr)
	if rc != ExitGateRefuse {
		t.Errorf("got %d, want %d", rc, ExitGateRefuse)
	}
}

func TestRun_ShipNonZeroIsTolerated(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rc := Run(Inputs{
		Cycle:       1,
		Workspace:   filepath.Join(root, "ws"),
		ProjectRoot: root,
		AdvanceFn:   func(string, string) error { return nil },
		ShipDryRunFn: func(string) (int, error) {
			return 2, errors.New("tree state mismatch")
		},
		VerifyFn: func() error { return nil },
	}, os.Stderr)
	if rc != ExitOK {
		t.Errorf("ship rc=2 should be tolerated, got run rc=%d", rc)
	}
}

func TestRun_VerifyFailDoesNotAbort(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:        2,
		Workspace:    filepath.Join(root, "ws"),
		ProjectRoot:  root,
		AdvanceFn:    func(string, string) error { return nil },
		ShipDryRunFn: func(string) (int, error) { return 0, nil },
		VerifyFn:     func() error { return errors.New("chain anomaly") },
	}, &stderr)
	if rc != ExitOK {
		t.Errorf("verify failure should not abort, got rc=%d", rc)
	}
	if !strings.Contains(stderr.String(), "WARN: ledger chain") {
		t.Errorf("missing warn line: %s", stderr.String())
	}
}

func TestReadChainLink_MissingFile(t *testing.T) {
	t.Parallel()
	prev, seq, err := readChainLink("/nonexistent/ledger.jsonl")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if prev != zeroSeed || seq != 0 {
		t.Errorf("got prev=%s seq=%d, want zero/0", prev, seq)
	}
}

func TestReadChainLink_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	prev, seq, _ := readChainLink(path)
	if prev != zeroSeed || seq != 0 {
		t.Errorf("empty: got prev=%s seq=%d", prev, seq)
	}
}

func TestReadChainLink_ChainHashCorrect(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	prior := []string{`{"x":1}`, `{"x":2}`}
	if err := os.WriteFile(path, []byte(strings.Join(prior, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev, seq, _ := readChainLink(path)
	sum := sha256.Sum256([]byte(prior[1]))
	wantPrev := hex.EncodeToString(sum[:])
	if prev != wantPrev {
		t.Errorf("got prev=%s, want %s", prev, wantPrev)
	}
	if seq != 2 {
		t.Errorf("got seq=%d, want 2", seq)
	}
}

func TestJSONCompact_StableKeyOrder(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"role":       "scout",
		"ts":         "2026-01-01T00:00:00Z",
		"cycle":      1,
		"kind":       "agent_subprocess",
		"prev_hash":  "abc",
		"entry_seq":  0,
		"simulated":  true,
		"exit_code":  0,
		"unknown":    "skipped",
	}
	got, err := jsonCompact(m)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// keys should appear in canonical order; ts comes before cycle, etc.
	tsIdx := strings.Index(got, `"ts"`)
	cycleIdx := strings.Index(got, `"cycle"`)
	roleIdx := strings.Index(got, `"role"`)
	if !(tsIdx < cycleIdx && cycleIdx < roleIdx) {
		t.Errorf("key order broken: %s", got)
	}
	if strings.Contains(got, "unknown") {
		t.Errorf("unknown key should not be emitted: %s", got)
	}
}

func TestRun_DefaultFnsAreConstructible(t *testing.T) {
	t.Parallel()
	// Drive Run with all defaults — advance/ship/verify will shell out to
	// nonexistent scripts and produce errors, but that's an expected and
	// tolerated path (verify failure logs WARN; advance failure aborts).
	root := t.TempDir()
	var stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:       1,
		Workspace:   filepath.Join(root, "ws"),
		ProjectRoot: root,
		PluginRoot:  root,
	}, &stderr)
	// advance shells out to a nonexistent bash script → error → gate refuse
	if rc != ExitGateRefuse {
		t.Errorf("default shell-outs against nonexistent scripts should refuse gate, got rc=%d", rc)
	}
}

func TestRun_WorkspaceMkdirError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Put a regular file where the workspace should be — mkdir will fail.
	conflict := filepath.Join(root, "conflict")
	if err := os.WriteFile(conflict, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:        1,
		Workspace:    conflict, // not a directory
		ProjectRoot:  root,
		AdvanceFn:    func(string, string) error { return nil },
		ShipDryRunFn: func(string) (int, error) { return 0, nil },
		VerifyFn:     func() error { return nil },
	}, &stderr)
	if rc != ExitRuntimeErr {
		t.Errorf("workspace mkdir failure should rc=%d, got %d", ExitRuntimeErr, rc)
	}
}

func TestRun_AdvanceShipPhaseRefuse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// AdvanceFn returns nil for everything EXCEPT ship → ship-phase advance refused
	advanceCount := 0
	rc := Run(Inputs{
		Cycle:       3,
		Workspace:   filepath.Join(root, "ws"),
		ProjectRoot: root,
		AdvanceFn: func(phase, agent string) error {
			advanceCount++
			if phase == "ship" {
				return errors.New("ship gate refused")
			}
			return nil
		},
		ShipDryRunFn: func(string) (int, error) { return 0, nil },
		VerifyFn:     func() error { return nil },
	}, os.Stderr)
	if rc != ExitGateRefuse {
		t.Errorf("ship-phase refusal should rc=%d, got %d", ExitGateRefuse, rc)
	}
}

func TestRun_AdvanceRetroRefuse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rc := Run(Inputs{
		Cycle:       4,
		Workspace:   filepath.Join(root, "ws"),
		ProjectRoot: root,
		AdvanceFn: func(phase, agent string) error {
			if phase == "retrospective" {
				return errors.New("retro refused")
			}
			return nil
		},
		ShipDryRunFn: func(string) (int, error) { return 0, nil },
		VerifyFn:     func() error { return nil },
	}, os.Stderr)
	if rc != ExitGateRefuse {
		t.Errorf("retro-phase refusal should rc=%d, got %d", ExitGateRefuse, rc)
	}
}

func TestRun_DefaultTokenContainsCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:        77,
		Workspace:    filepath.Join(root, "ws"),
		ProjectRoot:  root,
		AdvanceFn:    func(string, string) error { return nil },
		ShipDryRunFn: func(string) (int, error) { return 0, nil },
		VerifyFn:     func() error { return nil },
	}, &stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	body, _ := os.ReadFile(filepath.Join(root, "ws", "intent.md"))
	if !strings.Contains(string(body), "sim-token-77-") {
		t.Errorf("default token missing cycle prefix: %s", body)
	}
}
