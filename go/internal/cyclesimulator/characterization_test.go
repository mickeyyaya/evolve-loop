package cyclesimulator

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func offsetClock() time.Time {
	return time.Date(2026, 5, 23, 12, 34, 56, 0, time.FixedZone("UTC+5", 5*3600))
}

func recordingInputs(root string, calls *[]string, refusePhase string, shipRC int, verifyErr error) Inputs {
	return Inputs{
		Cycle:       42,
		Workspace:   filepath.Join(root, "ws"),
		ProjectRoot: root,
		Token:       "tok-42",
		Now:         offsetClock,
		AdvanceFn: func(phase, agent string) error {
			*calls = append(*calls, "advance "+phase+" "+agent)
			if phase == refusePhase {
				return errors.New("refused")
			}
			return nil
		},
		ShipDryRunFn: func(msg string) (int, error) {
			*calls = append(*calls, "ship "+msg)
			return shipRC, nil
		},
		VerifyFn: func() error {
			*calls = append(*calls, "verify")
			return verifyErr
		},
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

func TestRunCharacterization_ValidationLogsExactLines(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	cases := []struct {
		name string
		in   Inputs
		want string
	}{
		{"zero cycle", Inputs{Cycle: 0, Workspace: ws, ProjectRoot: root}, "[simulator] cycle must be positive integer, got: 0\n"},
		{"no workspace", Inputs{Cycle: 1, ProjectRoot: root}, "[simulator] missing workspace arg\n"},
		{"no project root", Inputs{Cycle: 1, Workspace: ws}, "[simulator] missing project root (EVOLVE_PROJECT_ROOT)\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stderr strings.Builder
			rc := Run(c.in, &stderr)
			if rc != ExitRuntimeErr || stderr.String() != c.want {
				t.Errorf("rc=%d stderr=%q, want rc=%d stderr=%q", rc, stderr.String(), ExitRuntimeErr, c.want)
			}
		})
	}
}

func TestRunCharacterization_FullWalkGolden(t *testing.T) {
	root := t.TempDir()
	var calls []string
	var stderr strings.Builder
	if rc := Run(recordingInputs(root, &calls, "", 3, errors.New("anomaly")), &stderr); rc != ExitOK {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}

	wantStderr := `[simulator] starting simulated walk for cycle 42
[simulator]   ✓ intent → wrote intent.md, ledger entry
[simulator]   ✓ research → wrote scout-report.md, ledger entry
[simulator]   ✓ build → wrote build-report.md, ledger entry
[simulator]   ✓ audit → wrote audit-report.md, ledger entry
[simulator]   ▶ ship phase: invoking ship.sh --dry-run
[simulator]   ⚠ ship.sh --dry-run exited rc=3 (acceptable for tree-state-mismatch in simulator context)
[simulator]   ✓ retrospective → wrote retrospective-report.md, ledger entry
[simulator] verifying ledger chain post-simulation...
[simulator] WARN: ledger chain verification flagged anomalies (may be pre-existing; simulator did not break it)
[simulator] DONE: simulated cycle 42 complete
`
	if stderr.String() != wantStderr {
		t.Errorf("stderr:\n%s\nwant:\n%s", stderr.String(), wantStderr)
	}

	wantCalls := strings.Join([]string{
		"advance intent intent", "advance research scout", "advance build builder", "advance audit auditor",
		"advance ship orchestrator", "ship simulator: cycle 42 plumbing test",
		"advance retrospective retrospective", "verify",
	}, "|")
	if got := strings.Join(calls, "|"); got != wantCalls {
		t.Errorf("seam calls %s, want %s", got, wantCalls)
	}

	wantReport := "<!-- challenge-token: tok-42 -->\n# Cycle Simulator Report — Cycle 42\n\n" +
		"All 6 phases advanced cleanly. 6 ledger entries appended (chain intact).\n" +
		"Ship phase exercised via ship.sh --dry-run (rc=3).\n\n" +
		"This is a no-LLM plumbing validation; agent output quality is NOT validated.\n"
	if got := readString(t, filepath.Join(root, "ws", "simulator-report.md")); got != wantReport {
		t.Errorf("simulator-report %q, want %q", got, wantReport)
	}
	if got := readString(t, filepath.Join(root, "ws", "retrospective-report.md")); !strings.HasPrefix(got, "<!-- challenge-token: tok-42 -->") {
		t.Errorf("retrospective-report lacks the token: %q", got)
	}
}

func TestRunCharacterization_LedgerLinesAndTipGolden(t *testing.T) {
	root := t.TempDir()
	var calls []string
	if rc := Run(recordingInputs(root, &calls, "", 0, nil), io.Discard); rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	lines := strings.Split(strings.TrimRight(readString(t, filepath.Join(root, ".evolve", "ledger.jsonl")), "\n"), "\n")
	entries := []struct{ role, file string }{
		{"intent", "intent.md"}, {"scout", "scout-report.md"}, {"builder", "build-report.md"},
		{"auditor", "audit-report.md"}, {"retrospective", "retrospective-report.md"},
	}
	if len(lines) != len(entries) {
		t.Fatalf("ledger has %d lines, want %d", len(lines), len(entries))
	}
	prev := zeroSeed
	for i, e := range entries {
		artifact := filepath.Join(root, "ws", e.file)
		want := fmt.Sprintf(`{"ts":"2026-05-23T07:34:56Z","cycle":42,"role":%q,"kind":"agent_subprocess","model":"simulator","exit_code":0,"duration_s":"0","artifact_path":%q,"artifact_sha256":%q,"challenge_token":"tok-42","git_head":"unknown","tree_state_sha":%q,"entry_seq":%d,"prev_hash":%q,"simulated":true}`,
			e.role, artifact, sha256Hex(readString(t, artifact)), sha256Hex(""), i, prev)
		if lines[i] != want {
			t.Errorf("ledger line %d\n got %s\nwant %s", i, lines[i], want)
		}
		prev = sha256Hex(lines[i])
	}
	if tip := readString(t, filepath.Join(root, ".evolve", "ledger.tip")); tip != "4:"+sha256Hex(lines[4])+"\n" {
		t.Errorf("ledger.tip %q", tip)
	}
}

func TestRunCharacterization_EveryRefusalStopsTheWalk(t *testing.T) {
	cases := map[string]string{
		"build":         "[simulator] FAIL: cycle-state advance to build (builder) refused",
		"ship":          "[simulator] FAIL: cycle-state advance to ship refused",
		"retrospective": "[simulator] FAIL: cycle-state advance to retrospective refused",
	}
	for phase, want := range cases {
		t.Run(phase, func(t *testing.T) {
			var calls []string
			var stderr strings.Builder
			rc := Run(recordingInputs(t.TempDir(), &calls, phase, 0, nil), &stderr)
			if rc != ExitGateRefuse || lastLine(stderr.String()) != want {
				t.Errorf("rc=%d stderr=%s, want rc=%d ending %q", rc, stderr.String(), ExitGateRefuse, want)
			}
		})
	}
}

func TestRunCharacterization_EveryWriteFailureLogsItsStep(t *testing.T) {
	cases := []struct {
		name, wantPrefix string
		prepare          func(root string) error
	}{
		{"workspace mkdir", "[simulator] workspace mkdir failed: ", func(r string) error {
			return os.WriteFile(filepath.Join(r, "ws"), nil, 0o644)
		}},
		{"artifact", "[simulator] FAIL: writing intent.md: ", func(r string) error {
			return os.MkdirAll(filepath.Join(r, "ws", "intent.md"), 0o755)
		}},
		{"ledger append", "[simulator] FAIL: ledger append for intent: ", func(r string) error {
			return os.WriteFile(filepath.Join(r, ".evolve"), nil, 0o644)
		}},
		{"retrospective", "[simulator] FAIL: writing retrospective: ", func(r string) error {
			return os.MkdirAll(filepath.Join(r, "ws", "retrospective-report.md"), 0o755)
		}},
		{"simulator-report", "[simulator] FAIL: writing simulator-report: ", func(r string) error {
			return os.MkdirAll(filepath.Join(r, "ws", "simulator-report.md"), 0o755)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if err := c.prepare(root); err != nil {
				t.Fatal(err)
			}
			var calls []string
			var stderr strings.Builder
			rc := Run(recordingInputs(root, &calls, "", 0, nil), &stderr)
			if rc != ExitRuntimeErr || !strings.HasPrefix(lastLine(stderr.String()), c.wantPrefix) {
				t.Errorf("rc=%d stderr=%s, want rc=%d ending with prefix %q", rc, stderr.String(), ExitRuntimeErr, c.wantPrefix)
			}
		})
	}
}

func TestRunCharacterization_RetrospectiveLedgerFailureLogsRole(t *testing.T) {
	root := t.TempDir()
	var calls []string
	in := recordingInputs(root, &calls, "", 0, nil)
	ledger := filepath.Join(root, ".evolve", "ledger.jsonl")
	in.ShipDryRunFn = func(string) (int, error) { return 0, os.Chmod(ledger, 0o000) }
	t.Cleanup(func() { _ = os.Chmod(ledger, 0o644) })
	var stderr strings.Builder
	rc := Run(in, &stderr)
	if rc != ExitRuntimeErr || !strings.HasPrefix(lastLine(stderr.String()), "[simulator] FAIL: ledger append for retrospective: ") {
		t.Errorf("rc=%d stderr=%s", rc, stderr.String())
	}
}

func TestRunCharacterization_DefaultSeamsShellOutUnderProjectRoot(t *testing.T) {
	root := t.TempDir()
	callLog := filepath.Join(root, "calls.log")
	scripts := map[string]string{
		"lifecycle/cycle-state.sh":             "",
		"lifecycle/ship.sh":                    "exit 4\n",
		"observability/verify-ledger-chain.sh": "",
	}
	for rel, tail := range scripts {
		path := filepath.Join(root, "legacy", "scripts", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "#!/bin/bash\necho \"" + filepath.Base(rel) + " $*\" >> \"" + callLog + "\"\n" + tail
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var stderr strings.Builder
	if rc := Run(Inputs{Cycle: 7, Workspace: filepath.Join(root, "ws"), ProjectRoot: root}, &stderr); rc != ExitOK {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	got := readString(t, callLog)
	for _, want := range []string{"cycle-state.sh advance build builder\n", "ship.sh --dry-run simulator: cycle 7 plumbing test\n", "verify-ledger-chain.sh \n"} {
		if !strings.Contains(got, want) {
			t.Errorf("script calls %q lack %q", got, want)
		}
	}
	if intent := readString(t, filepath.Join(root, "ws", "intent.md")); !strings.Contains(intent, fmt.Sprintf("sim-token-7-%d -->", os.Getpid())) {
		t.Errorf("default token missing from intent.md: %s", intent)
	}
	ledger := readString(t, filepath.Join(root, ".evolve", "ledger.jsonl"))
	ts := strings.SplitN(strings.SplitN(ledger, `"ts":"`, 2)[1], `"`, 2)[0]
	stamped, err := time.Parse("2006-01-02T15:04:05Z", ts)
	if err != nil || time.Since(stamped).Abs() > time.Hour {
		t.Errorf("default clock stamped ts %q (err %v)", ts, err)
	}
}

func TestAppendSimLedgerCharacterization_GitHeadAndTreeStateFromRepo(t *testing.T) {
	root := t.TempDir()
	repo := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "f"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "f")
	repo.Git("commit", "-q", "-m", "seed")
	if err := os.WriteFile(filepath.Join(repo.Dir, "f"), []byte("1\n2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "f")
	artifact := filepath.Join(root, "a.md")
	if err := os.WriteFile(artifact, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(root, ".evolve", "ledger.jsonl")
	if err := appendSimLedger(ledger, 1, "builder", artifact, "tok", repo.Dir, offsetClock); err != nil {
		t.Fatal(err)
	}
	line := readString(t, ledger)
	diff := repo.Git("diff", "HEAD")
	if diff == "" {
		t.Fatal("staged change produced no diff against HEAD")
	}
	if want := `"git_head":"` + repo.Git("rev-parse", "HEAD") + `"`; !strings.Contains(line, want) {
		t.Errorf("line %s lacks %s", line, want)
	}
	if want := `"tree_state_sha":"` + sha256Hex(diff) + `"`; !strings.Contains(line, want) {
		t.Errorf("line %s lacks %s", line, want)
	}
}

func TestAppendSimLedgerCharacterization_FailedTipWriteRemovesTempPath(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, ".evolve", "ledger.tip.tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "a.md")
	if err := os.WriteFile(artifact, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendSimLedger(filepath.Join(root, ".evolve", "ledger.jsonl"), 1, "s", artifact, "tok", root, offsetClock); err == nil {
		t.Fatal("appendSimLedger succeeded with a directory at the tip temp path")
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("tip temp path still present: %v", err)
	}
}
