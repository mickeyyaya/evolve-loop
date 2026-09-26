package main

// cmd_inbox_signals_test.go — ADR-0103 unit 06 step 4 (tests 54-55): the
// inbox module tag renders at the --simulate root and lands in the run
// workspace's signals.ndjson through the FAIL closeout; the Center-less
// `inboxmover.Options{` literals are an allow-list with reasons (the 06-F1
// debt), the three applyCycleFailureOutcome( call sites pass a non-nil
// signals token, and no production package constructs a second Center.

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// Test 54a — a promote whose destination mkdir fails renders [inbox] on the
// simulate root's console and lands in the cycle-less signals.ndjson.
func TestWireSimulateOrchestrator_InboxWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "t1.json"), []byte(`{"id":"t1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "processed"), []byte("x"), 0o644); err != nil { // a FILE at processed/: the mkdir fault
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	if _, err := inboxmover.Promote(inboxmover.Options{ProjectRoot: root, Signals: d.Signals}, "t1", "processed", inboxmover.PromoteOpts{}); err == nil {
		t.Fatal("the mkdir fault must surface as ErrMvFailed")
	}
	if out := console.String(); !strings.Contains(out, "[inbox]") || !strings.Contains(out, "INBOX_PROMOTE_MOVE_FAILED") || strings.Contains(out, "[inbox-mover] ERROR: ") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate and the fallback line does not print: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"INBOX_PROMOTE_MOVE_FAILED"`) || !strings.Contains(string(data), `"module":"inbox"`) {
		t.Errorf("the cycle-less signal is durable: %v %s", err, data)
	}
}

// Test 54b — the FAIL closeout stamps the run workspace's signals.ndjson with
// the mover's cycle-scoped event through applyCycleFailureOutcome's signals.
func TestApplyCycleFailureOutcome_StampsRunWorkspaceSignals(t *testing.T) {
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 7)
	// The committed item is already claimed under processing/cycle-7 (the
	// triage persona's claim) and a root twin of its basename provokes the
	// double-move when the drain releases it.
	proc := filepath.Join(evolveDir, "inbox", "processing", "cycle-7")
	if err := os.MkdirAll(proc, 0o755); err != nil {
		t.Fatal(err)
	}
	item := filepath.Join(evolveDir, "inbox", "2026-09-13T00-00-00Z-poison.json")
	if err := os.Rename(item, filepath.Join(proc, filepath.Base(item))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(item, []byte(`{"id":"poison-root"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	if err := applyCycleFailureOutcome(root, evolveDir, 7, &console, newFakeLedger(), d.Signals); err != nil {
		t.Fatalf("applyCycleFailureOutcome: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 7), "signals.ndjson"))
	if err != nil || !strings.Contains(string(data), `"code":"INBOX_RELEASE_DOUBLE_MOVE"`) || !strings.Contains(string(data), `"cycle":7`) {
		t.Errorf("the run workspace's signals.ndjson carries the drain's event at the cycle: %v %s", err, data)
	}
	if !strings.Contains(console.String(), "INBOX_RELEASE_DOUBLE_MOVE cycle=7") {
		t.Errorf("the console renders it: %q", console.String())
	}
}

// Test 55 — the Center-less roots are pinned: every non-test
// `inboxmover.Options{` literal without a Signals: field is on the allow-list
// with its reason (06-F1 threads them root by root); the three package-level
// applyCycleFailureOutcome( call sites pass a signals token that is not nil;
// the non-test signalcenter.New( count stays 1 (no library-built Center).
func TestInboxCenterlessRootsArePinned(t *testing.T) {
	allowed := map[string]string{
		"cmd/evolve/cmd_inbox_mover.go":         "the triage persona's `evolve inbox-mover` root — 06-F1 verifies <run>/signals.ndjson is writable under the triage sandbox first",
		"cmd/evolve/cmd_inbox_quarantine.go":    "operator command `evolve inbox quarantine release` — 06-F1",
		"cmd/evolve/cmd_continuation.go":        "operator command `evolve continuation` — 06-F1",
		"cmd/evolve/cmd_inbox_consume.go":       "operator command `evolve inbox consume` (two literals) — 06-F1",
		"cmd/evolve/cmd_cycle.go":               "read-only probes (failure count, continuation scope, dispatch state) — never write; hostInboxClaimer — WIRED (Signals: signals)",
		"internal/loopwave/launcher.go":         "the wave engine's freshness probe (ADR-0103 unit 13) — read-only, WIRED (Signals: e.center())",
		"internal/loopwave/plan.go":             "the wave engine's consumed-top_n prune probe (ADR-0103 unit 13) — read-only, WIRED (Signals: e.center())",
		"internal/triagecap/lane_menu.go":       "the ProjectRoot-less prune probe — never writes",
		"internal/phases/ship/postship.go":      "the PASS closeout inside `evolve ship` (ship.Options carries no Center) — 06-F1",
		"internal/cycleoutcome/cycleoutcome.go": "the FAIL closeout — WIRED (Signals: in.Signals); listed because its literal is the one that names the field",
	}
	literalRE := regexp.MustCompile(`inboxmover\.Options\{`)
	moduleRoot := filepath.Join("..", "..")
	seen := map[string]bool{}
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(literalRE.FindAll(src, -1)) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		if _, ok := allowed[rel]; !ok {
			t.Errorf("%s builds an inboxmover.Options literal — thread the root's Center (Signals:) or pin it here with a reason", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for rel := range allowed {
		if !seen[rel] {
			t.Errorf("%s is pinned but no longer builds an Options literal — drop the pin", rel)
		}
	}
	cycleSrc, _ := os.ReadFile(filepath.Join(moduleRoot, "cmd", "evolve", "cmd_cycle.go"))
	loopSrc, _ := os.ReadFile(filepath.Join(moduleRoot, "cmd", "evolve", "cmd_loop_sequential_outcome.go"))
	callRE := regexp.MustCompile(`\bapplyCycleFailureOutcome\(([^)]*)\)`)
	calls := 0
	for _, src := range []string{string(cycleSrc), string(loopSrc)} {
		for _, line := range strings.Split(src, "\n") {
			if strings.Contains(line, "func ") {
				continue // the two definitions
			}
			for _, m := range callRE.FindAllStringSubmatch(line, -1) {
				args := strings.Split(m[1], ",")
				if len(args) < 6 {
					continue // the method's one-arg call
				}
				calls++
				if last := strings.TrimSpace(args[5]); last == "nil" || last == "" {
					t.Errorf("applyCycleFailureOutcome(%s) passes no Center — the FAIL closeout is the wired root", m[1])
				}
			}
		}
	}
	if calls != 3 {
		t.Errorf("the three package-level call sites (cmd_cycle.go ×2, cmd_loop_sequential_outcome.go ×1) must be present, found %d", calls)
	}
	// Two process roots build a Center: the cycle/loop root (cmd_cycle.go) and
	// the manual `evolve phase observer` subcommand (its own process; ADR-0103
	// unit 12). Every other Center reaches a component through an accessor.
	centerRoots := map[string]bool{"cmd/evolve/cmd_cycle.go": true, "internal/cli/phasecmd/phase_observer.go": true}
	got := nonTestConstructionsOf(t, moduleRoot, `\bsignalcenter\.New\(`)
	unexpected := false
	for _, g := range got {
		if !centerRoots[g] {
			unexpected = true
		}
	}
	if unexpected || len(got) != len(centerRoots) {
		t.Errorf("the production Centers are built only at the two process roots (cmd_cycle.go, phasecmd/phase_observer.go); found %v", got)
	}
}

// nonTestConstructionsOf lists the non-test files under moduleRoot whose
// source matches re.
func nonTestConstructionsOf(t *testing.T, moduleRoot, re string) []string {
	t.Helper()
	callRE := regexp.MustCompile(re)
	var files []string
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if callRE.Match(src) {
			rel, _ := filepath.Rel(moduleRoot, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
