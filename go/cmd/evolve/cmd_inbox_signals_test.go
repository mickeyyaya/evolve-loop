package main

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

func TestWireSimulateOrchestrator_InboxWarningRenders(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "t1.json"), []byte(`{"id":"t1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "processed"), []byte("x"), 0o644); err != nil {
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

func TestApplyCycleFailureOutcome_StampsRunWorkspaceSignals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 7)
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

func TestInboxCenterlessRootsArePinned(t *testing.T) {
	t.Parallel()
	allowed := map[string]string{
		"cmd/evolve/cmd_inbox_mover.go":         "the triage persona's `evolve inbox-mover` root — 06-F1 verifies <run>/signals.ndjson is writable under the triage sandbox first",
		"cmd/evolve/cmd_inbox.go":               "operator commands `evolve inbox batches`, `list` and `show`: the read-only lane-menu placement (ADR-0106 W3, ADR-0116) — never writes",
		"cmd/evolve/cmd_inbox_curate.go":        "operator commands `evolve inbox edit`, `withdraw` and `verify` (ADR-0116) — 06-F1",
		"internal/phases/triage/triage.go":      "the triage menu's read-only dependency partition (ADR-0106 W3) — never writes",
		"cmd/evolve/cmd_inbox_quarantine.go":    "operator command `evolve inbox quarantine release` — 06-F1",
		"cmd/evolve/cmd_inbox_route_console.go": "operator command `evolve inbox route-console` — 06-F1",
		"cmd/evolve/cmd_inbox_claims.go":        "operator commands `evolve inbox claims` and `release` (cycle 1838) — a console verb prints its own lines",
		"cmd/evolve/cmd_inbox_claim.go":         "operator command `evolve inbox claim` (X17d) — a console verb prints its own lines, as `release` does",
		"cmd/evolve/cmd_syncmain.go":            "operator command `evolve sync-main`, the claim absorb after the merge (cycle 1838) — it prints its own lines",
		"cmd/evolve/cmd_loop_wavesync.go":       "the loop's wave-boundary sync, the claim absorb after the fast-forward (cycle 1838) — it prints its own [loop] lines",
		"cmd/evolve/cmd_gc_claims.go":           "operator command `evolve gc`, the empty claim dir sweep (cycle 1838) — read-only survey, no mover warning",
		"cmd/evolve/cmd_gc_continuations.go":    "operator command `evolve gc`, the release of old continuation bindings — a console verb prints its own lines",
		"cmd/evolve/cmd_inbox_route_lane.go":    "operator command `evolve inbox route-lane` — 06-F1",
		"cmd/evolve/cmd_inbox_split.go":         "operator command `evolve inbox split` — 06-F1",
		"cmd/evolve/cmd_inbox_add.go":           "operator command `evolve inbox add` — 06-F1",
		"cmd/evolve/cmd_practice.go":            "operator command `evolve practice watch` and the boundary watch, the one filing of a research item — 06-F1",
		"cmd/evolve/cmd_continuation.go":        "operator command `evolve continuation` — 06-F1",
		"cmd/evolve/cmd_inbox_consume.go":       "operator command `evolve inbox consume` (two literals) — 06-F1",
		"cmd/evolve/cmd_cycle.go":               "read-only probes (failure count, continuation scope, dispatch state) — never write; hostInboxClaimer — WIRED (Signals: signals)",
		"internal/loopwave/launcher.go":         "the wave engine's one read-only lifecycle, rooted at EvolveDir: the launch gate's probe and the plan-time prune (ADR-0106 W3) — WIRED (Signals: e.center())",
		"internal/triagecap/lane_menu.go":       "the ProjectRoot-less read-only lifecycle (the undispatchable prune and the backlog deps check) — never writes",
		"cmd/evolve/cmd_loop_escalate.go":       "the loop's escalation boundary reweight (cycle 1932) — WIRED (Signals: signals)",
		"cmd/evolve/cmd_loop_goal_ids.go":       "the launch check of the items a goal names (goal-text-has-no-selection-authority) — it places pending items read-only and never moves a file, so the mover has nothing to emit",
		"cmd/evolve/cmd_loop_signal_stop.go":    "the claim survey of the fleet signal-stop message (resume-after-a-fleet-signal-stop) — read-only, it names the stopped lanes and moves no file",
		"cmd/evolve/cmd_supply_wiring.go":       "the supply census of `evolve wave supply` and of each loop iteration (ADR-0139 O6) — it scans and ranks the inbox and never moves a file, so the mover has nothing to emit",
		"internal/phases/ship/postship.go":      "the PASS closeout inside `evolve ship` (ship.Options carries no Center) — 06-F1",
		"internal/cycleoutcome/cycleoutcome.go": "the FAIL closeout — WIRED (Signals: in.Signals); listed because its literal is the one that names the field",
		"internal/cycleoutcome/nowork.go":       "the planned-no-work closeout (F30) — WIRED (Signals: in.Signals); listed for the same reason",
		"cmd/evolve/cmd_inbox_curator_apply.go": "the reroute of `evolve inbox curate --apply` — WIRED (Signals: r.signals, the inbox root Center); listed because its literal names the field",
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
	callRE := regexp.MustCompile(`\bapply(?:Cycle|Result)FailureOutcome\(([^)]*)\)`)
	calls := 0
	for _, src := range []string{string(cycleSrc), string(loopSrc)} {
		for _, line := range strings.Split(src, "\n") {
			if strings.Contains(line, "func ") {
				continue
			}
			for _, m := range callRE.FindAllStringSubmatch(line, -1) {
				args := strings.Split(m[1], ",")
				if len(args) < 6 {
					continue
				}
				calls++
				if last := strings.TrimSpace(args[5]); last == "nil" || last == "" {
					t.Errorf("applyCycleFailureOutcome(%s) passes no Center — the FAIL closeout is the wired root", m[1])
				}
			}
		}
	}
	if calls != 4 {
		t.Errorf("the four package-level call sites (cmd_cycle.go ×3 with the result closeout, cmd_loop_sequential_outcome.go ×1) must be present, found %d", calls)
	}
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
