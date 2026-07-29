//go:build acs

// Package cycle1191 materialises the cycle-1191 acceptance criteria for the
// three fleet-scoped tasks pinned to this lane:
//
//   - ledger-verify-seal-anchor                        (predicates 001–005)
//   - bridgewatch-follow-macos-flake                   (predicates 006–007)
//   - loop-must-base-lanes-on-origin-main-not-stale-local (predicate 008)
//
// CONTINUATION CONTEXT. This lane inherits a salvage snapshot (8395949e,
// ADR-0076 continuation-on-fail) that already carries substantial work for all
// three tasks. The cycle-1191 bar is therefore NOT "build it" but "close the
// gap the salvage left" — every predicate below was authored against the LIVE
// artifacts in this worktree, and the RED ones name a real, reproduced defect:
//
//   - 001/004/005: the salvaged effectiveAnchorSHA (anchor.go) recognises an
//     in-band seal by `entry.Kind` having the `reset-seal-` prefix. The REAL
//     ledger carries the seal marker in `cycle_label` ("reset-seal-cycle-108")
//     with `kind:"reset"` — see .evolve/ledger.jsonl:1880. So the resolver
//     never fires on production data and `evolve ledger verify` STILL emits the
//     line-1740 wolf-cry (reproduced live on 2026-07-29). Unit-green, live-red.
//   - 002: the inbox Guard ("a seal must only anchor if itself hash-valid from
//     its own prev") is unimplemented — effectiveAnchorSHA trusts any
//     operator-role seal line, so a seal whose own prev_hash is forged silences
//     verification of everything before it.
//   - 007: the salvage fixed ONE follow test (SkipsMalformedAndEmptyLines) but
//     left its sibling TestRunBridgeWatchFollow_TailsNewLines with the identical
//     flake shape — a 10ms fixed sleep against an async offset seed inside a
//     200ms deadline (cmd_bridge_watch_test.go:311,317). The acceptance scope is
//     `-run TestRunBridgeWatchFollow`, which matches both.
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban): 001–005 drive the real exported
// ledger.FileLedger.Verify over purpose-built fixture chains and over the LIVE
// ledger; 006 executes the follow test suite under -race; 008 runs the real
// looppreflight.Run against a real git repo whose local base is genuinely
// behind a real (file-remote) origin. 007 is the one shape assertion, and it is
// legitimate precisely because this task's DELIVERABLE IS the test's timing
// shape: it parses the Go AST and checks the numeric deadline literal, so it
// cannot be satisfied by adding a magic string.
package cycle1191

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// ledger fixture helpers
// ---------------------------------------------------------------------------

// forgedPrev is a syntactically valid but chain-wrong prev_hash used to plant a
// deliberate break. It is a constant so no fixture accidentally collides with a
// real line SHA (which would turn a planted break into a valid link).
const forgedPrev = "dead000000000000000000000000000000000000000000000000000000000beef"

// liveSealLabelPrefix is the marker the PRODUCTION ledger uses for an operator
// re-anchor: the `cycle_label` field, not `kind`. Verified against
// .evolve/ledger.jsonl:1880 — {"cycle_label":"reset-seal-cycle-108",
// "role":"operator","kind":"reset",...}. Predicates 001/004/005 exist because
// the salvaged resolver matches on `kind` and therefore never sees this.
const liveSealLabelPrefix = "reset-seal-"

// lineSHA mirrors the ledger's own sha256Hex over the line bytes EXCLUDING the
// terminating newline (ledger.go splitLines drops it before hashing).
func lineSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fixtureLine is one row of a fixture chain. breakPrev plants a forged
// prev_hash at that row (the historical damage / post-seal tamper).
type fixtureLine struct {
	entry     core.LedgerEntry
	breakPrev bool
}

// phaseLine is an ordinary orchestrator entry.
func phaseLine(kind string) fixtureLine {
	return fixtureLine{entry: core.LedgerEntry{
		TS: "2026-07-29T00:00:00Z", Role: "orchestrator", Kind: kind,
	}}
}

// brokenLine is an ordinary entry whose prev_hash does NOT chain — the shape of
// the ledger-1740 damage (and, after a seal, of a genuine post-seal tamper).
func brokenLine(kind string) fixtureLine {
	l := phaseLine(kind)
	l.breakPrev = true
	return l
}

// liveShapeSeal is an operator re-anchor in the shape the PRODUCTION ledger
// actually writes: the marker lives in cycle_label, kind is "reset".
func liveShapeSeal(label string) fixtureLine {
	return fixtureLine{entry: core.LedgerEntry{
		TS: "2026-07-29T00:00:00Z", Role: "operator", Kind: "reset",
		CycleLabel: liveSealLabelPrefix + label,
	}}
}

// dualMarkerSeal carries the seal marker in BOTH kind and cycle_label so the
// predicate is agnostic to which field the implementation keys on — whichever
// it honours, the assertion under test must still hold.
func dualMarkerSeal(label string) fixtureLine {
	return fixtureLine{entry: core.LedgerEntry{
		TS: "2026-07-29T00:00:00Z", Role: "operator",
		Kind:       liveSealLabelPrefix + label,
		CycleLabel: liveSealLabelPrefix + label,
	}}
}

// writeFixtureLedger materialises rows as a real ledger.jsonl + ledger.tip in a
// fresh dir and returns that dir. Row 0 is the zero-seeded genesis; every other
// row chains from the previous line's SHA unless it is marked breakPrev. The
// tip is written to match the LAST line, so a fixture only ever fails on the
// chain property under test, never on an incidental tip mismatch.
func writeFixtureLedger(t *testing.T, rows []fixtureLine) string {
	t.Helper()
	dir := t.TempDir()

	var buf strings.Builder
	prevSHA := ""
	lastSeq := 0
	for i, row := range rows {
		e := row.entry
		e.EntrySeq = i
		switch {
		case row.breakPrev:
			e.PrevHash = forgedPrev
		case i == 0:
			e.PrevHash = ledger.ZeroSeed
		default:
			e.PrevHash = prevSHA
		}
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal fixture row %d: %v", i, err)
		}
		buf.Write(raw)
		buf.WriteByte('\n')
		prevSHA = lineSHA(raw)
		lastSeq = e.EntrySeq
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(buf.String()), 0o644); err != nil {
		t.Fatalf("write fixture ledger: %v", err)
	}
	tip := fmt.Sprintf("%d:%s", lastSeq, prevSHA)
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write fixture tip: %v", err)
	}
	return dir
}

// verifyFixture runs the REAL exported verifier over a fixture dir.
func verifyFixture(t *testing.T, rows []fixtureLine) error {
	t.Helper()
	return ledger.New(writeFixtureLedger(t, rows)).Verify(context.Background())
}

// stateRoot resolves the MAIN project root (the STATE root): the suite exports
// EVOLVE_PROJECT_ROOT (issue #12), else the repo root (the redteam idiom).
func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// ---------------------------------------------------------------------------
// Task: ledger-verify-seal-anchor
// ---------------------------------------------------------------------------

// TestC1191_001_seal_anchor_resolves_production_seal_shape is the cycle-1191
// CRUX. Inbox acceptance #1: "a fixture ledger with break→seal→valid-chain
// verifies OK with an informational sealed-prefix note."
//
// The fixture writes the seal in the shape the LIVE ledger uses (cycle_label
// marker, kind "reset"). The salvaged resolver keys on kind, so today the seal
// is invisible, the walk starts at line 0, and the planted break at row 2 is
// reported as BROKEN — the exact wolf-cry the task must retire.
func TestC1191_001_seal_anchor_resolves_production_seal_shape(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		phaseLine("phase_end"),
		brokenLine("rewritten_history"), // the adjudicated, preserved damage
		liveShapeSeal("cycle-108"),      // operator re-anchor, PRODUCTION shape
		phaseLine("phase_start"),
		phaseLine("phase_end"),
	})
	if err != nil {
		t.Errorf("break→seal→valid-chain must verify OK once the walk anchors at the last operator reset-seal; got BROKEN: %v\n"+
			"the seal marker in the real ledger is cycle_label=%q with kind=%q (.evolve/ledger.jsonl:1880), "+
			"not a kind prefix — effectiveAnchorSHA must resolve the production shape", err,
			liveSealLabelPrefix+"cycle-108", "reset")
	}
}

// TestC1191_002_self_invalid_seal_must_not_anchor encodes the inbox GUARD: "a
// seal must only anchor if itself hash-valid from its own prev". A seal whose
// own prev_hash is forged is exactly what an attacker (or a corrupt writer)
// would append to silence verification of everything behind it, so anchoring on
// it would convert the fix into a chain-integrity bypass.
//
// NEGATIVE predicate — the strongest anti-no-op signal here: an implementation
// that simply "anchors at the last operator seal" passes 001 and 004 and FAILS
// this one.
func TestC1191_002_self_invalid_seal_must_not_anchor(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		phaseLine("phase_end"),
		brokenLine("rewritten_history"),
		// Seal carries both markers, but its OWN prev_hash does not chain from
		// the line before it — it is not hash-valid from its own prev.
		func() fixtureLine { s := dualMarkerSeal("cycle-forged"); s.breakPrev = true; return s }(),
		phaseLine("phase_start"),
		phaseLine("phase_end"),
	})
	if err == nil {
		t.Error("a reset-seal whose OWN prev_hash does not chain from its predecessor must NOT anchor the walk — " +
			"verify returned OK, so a forged seal can silence the entire prefix (inbox Guard)")
		return
	}
	if !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("self-invalid seal must fail as a chain break (ErrLedgerChainBroken); got %v", err)
	}
}

// TestC1191_003_break_after_last_seal_still_broken is inbox acceptance #1's
// second half: "a break AFTER the last seal still reports BROKEN." This is the
// over-anchoring guard — it fails any implementation that treats a seal as
// "stop verifying" rather than "start verifying here".
func TestC1191_003_break_after_last_seal_still_broken(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		phaseLine("phase_end"),
		liveShapeSeal("cycle-111"),
		phaseLine("phase_start"),
		brokenLine("post_seal_tamper"), // AFTER the anchor ⇒ must stay BROKEN
		phaseLine("phase_end"),
	})
	if err == nil {
		t.Error("a chain break AFTER the last reset-seal must still report BROKEN — verify returned OK, " +
			"so the seal is being read as 'stop verifying' instead of 'verify from here forward'")
	}
}

// TestC1191_004_two_seals_chain_normally is the inbox's "two seals chain
// normally" edge case: the LATER seal wins, and a break between the two seals
// is inside the sealed (untrusted, preserved) prefix.
func TestC1191_004_two_seals_chain_normally(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		brokenLine("rewritten_history"),
		liveShapeSeal("cycle-108"),
		phaseLine("phase_start"),
		liveShapeSeal("cycle-111"), // later seal must win
		phaseLine("phase_end"),
	})
	if err != nil {
		t.Errorf("two chained operator seals must resolve to the LATER anchor with the damaged prefix preserved-but-untrusted; got BROKEN: %v", err)
	}
}

// TestC1191_005_live_ledger_no_1740_wolf_cry is inbox acceptance #2, asserted
// against the REAL repository ledger: "evolve ledger verify on this repo reports
// the post-seal chain status instead of the line-1740 wolf-cry."
//
// It deliberately does NOT require the live chain to be clean — the inbox itself
// expects the post-seal region may surface genuine findings (the 2026-07-22
// cycle:null promote batch). The criterion is that verification gets PAST the
// adjudicated, sealed 1740 damage. Reproduced RED on 2026-07-29:
// `evolve ledger verify` → "BROKEN: ... line 1740 prev_hash mismatch".
func TestC1191_005_live_ledger_no_1740_wolf_cry(t *testing.T) {
	evolveDir := filepath.Join(stateRoot(t), ".evolve")
	if _, err := os.Stat(filepath.Join(evolveDir, "ledger.jsonl")); err != nil {
		t.Skipf("no live ledger at %s: %v", evolveDir, err)
	}
	err := ledger.New(evolveDir).Verify(context.Background())
	if err == nil {
		return // clean post-seal chain — strictly better than the bar
	}
	if strings.Contains(err.Error(), "line 1740") {
		t.Errorf("live `ledger verify` still cries wolf on the sealed line-1740 damage: %v\n"+
			"the walk must anchor at the LAST operator reset-seal (197 such entries exist in this ledger) "+
			"and report the post-seal chain status instead", err)
	}
}

// ---------------------------------------------------------------------------
// Task: bridgewatch-follow-macos-flake
// ---------------------------------------------------------------------------

// followTestFile is the file the flake lives in.
const followTestFile = "go/cmd/evolve/cmd_bridge_watch_test.go"

// observingFollowTests are the follow tests whose assertion depends on OBSERVING
// a line appended AFTER the follow loop asynchronously seeds its file offset —
// i.e. exactly the tests that can lose the seed race on a loaded macOS runner.
// The other two follow tests assert an ABSENCE (no output / non-fatal exit) and
// cannot flake on a slow runner, so they are correctly out of scope.
var observingFollowTests = []string{
	"TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines", // fixed by the salvage
	"TestRunBridgeWatchFollow_TailsNewLines",               // STILL the old shape
}

// minFollowDeadline is the inbox's floor: "the test's wait is event-driven with
// a deadline >= 10s".
const minFollowDeadline = 10

var (
	// secondsDeadlineRe matches a context deadline expressed in whole seconds.
	secondsDeadlineRe = regexp.MustCompile(`WithTimeout\([^,]+,\s*(\d+)\s*\*\s*time\.Second\s*\)`)
	// msSleepRe matches a fixed millisecond sleep — a bare wall-clock
	// assumption. A sleep of the poll interval (time.Sleep(watchFollowInterval))
	// is a retry cadence, not a deadline, and is deliberately NOT matched.
	msSleepRe = regexp.MustCompile(`time\.Sleep\([^)]*time\.Millisecond`)
)

// goFuncBody returns the source text of funcName in the Go file at path. A
// missing function is a LOUD error, never a silently-satisfied ==0 assertion.
func goFuncBody(path, funcName string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != funcName {
			continue
		}
		return string(raw[fset.Position(fd.Pos()).Offset:fset.Position(fd.End()).Offset]), nil
	}
	return "", fmt.Errorf("function %q not found in %s", funcName, path)
}

// TestC1191_006_follow_tests_race_clean_under_repetition executes the acceptance
// command (inbox acceptance #2/#4): the follow suite must be green under -race
// and repetition. This is the behavioural half of the flake criterion — a shape
// change that broke the tests' meaning cannot pass it.
func TestC1191_006_follow_tests_race_clean_under_repetition(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-race", "-count=25",
		"-run", "TestRunBridgeWatchFollow", "./cmd/evolve/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test -race -count=25 -run TestRunBridgeWatchFollow ./cmd/evolve/ must be green: %v\n%s", err, out)
	}
}

// TestC1191_007_follow_waits_are_event_driven_with_long_deadline encodes inbox
// acceptance #1: "the test's wait is event-driven with a deadline >= 10s, no
// bare sleeps shorter than the deadline."
//
// It parses the AST and reads the NUMERIC deadline, so it cannot be satisfied by
// planting a magic string — the assertion is on the timing shape itself, which
// IS this task's deliverable. Today TailsNewLines carries a 200ms deadline and a
// 10ms fixed sleep against an asynchronous offset seed (the identical race the
// salvage fixed in its sibling), so this predicate is RED.
func TestC1191_007_follow_waits_are_event_driven_with_long_deadline(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), followTestFile)
	for _, fn := range observingFollowTests {
		body, err := goFuncBody(path, fn)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		m := secondsDeadlineRe.FindStringSubmatch(body)
		if m == nil {
			t.Errorf("%s: no seconds-scale context deadline found — a follow test that must OBSERVE an appended "+
				"line needs an event wait bounded by a deadline >= %ds, not a millisecond window that a loaded "+
				"macOS runner can miss", fn, minFollowDeadline)
			continue
		}
		secs, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			t.Errorf("%s: unparsable deadline literal %q: %v", fn, m[1], convErr)
			continue
		}
		if secs < minFollowDeadline {
			t.Errorf("%s: context deadline is %ds, want >= %ds", fn, secs, minFollowDeadline)
		}
		if n := len(msSleepRe.FindAllString(body, -1)); n != 0 {
			t.Errorf("%s: %d fixed millisecond sleep(s) remain — the wait must synchronise on the observable "+
				"event (the rendered line), retrying at the poll interval, never on a wall-clock guess", fn, n)
		}
	}
}

// ---------------------------------------------------------------------------
// Task: loop-must-base-lanes-on-origin-main-not-stale-local
// ---------------------------------------------------------------------------

// git runs a git command in dir, failing the test loudly on error — a fixture
// that half-built would make the predicate assert on the wrong topology.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=acs@example.com",
		"-c", "user.name=acs",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// commitFile writes a file and commits it.
func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-m", "acs: "+name)
}

// behindBaseRepo builds a real work tree on branch main whose local base is one
// commit BEHIND a real (file-remote) origin/main — the exact topology that made
// every cycle-969 lane ship GIT_PUSH_REJECTED. No network is involved.
func behindBaseRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	for _, d := range []string{origin, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	git(t, origin, "init", "--bare", "-b", "main")
	git(t, work, "init", "-b", "main")
	git(t, work, "remote", "add", "origin", origin)
	commitFile(t, work, "a.txt", "one")
	git(t, work, "push", "origin", "main")
	commitFile(t, work, "b.txt", "two")
	git(t, work, "push", "origin", "main")
	// Rewind the LOCAL base only: origin/main keeps b.txt ⇒ local is behind 1.
	git(t, work, "reset", "--hard", "HEAD~1")
	return work
}

// TestC1191_008_boot_halts_on_base_behind_origin is the WIRING proof for the
// preflight halt: it runs the real looppreflight.Run (not the check function in
// isolation) against the behind-base topology and requires the base-divergence
// check to be REGISTERED, to fire at Halt, and to name the reconcile command.
// A check that exists but is not in Run's list fails here.
func TestC1191_008_boot_halts_on_base_behind_origin(t *testing.T) {
	work := behindBaseRepo(t)
	evolveDir := filepath.Join(work, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	res, err := looppreflight.Run(looppreflight.Options{
		ProjectRoot: work,
		EvolveDir:   evolveDir,
		ProfileDir:  filepath.Join(work, "profiles"),
		Stderr:      io.Discard,
		SkipBoot:    true,
	})
	if err != nil {
		t.Fatalf("looppreflight.Run harness fault: %v", err)
	}
	var found *looppreflight.CheckResult
	for i := range res.Checks {
		if res.Checks[i].Name == "base-divergence" {
			found = &res.Checks[i]
			break
		}
	}
	if found == nil {
		names := make([]string, 0, len(res.Checks))
		for _, c := range res.Checks {
			names = append(names, c.Name)
		}
		t.Fatalf("boot preflight runs no `base-divergence` check — lanes would still be cut from a stale base; checks=%v", names)
	}
	if found.Level != looppreflight.LevelHalt {
		t.Errorf("local main is 1 commit behind origin/main: base-divergence must HALT the batch before any lane spawns, got level=%v (%s / %s)",
			found.Level, found.Message, found.Detail)
	}
	if !strings.Contains(found.Detail, "evolve sync-main") {
		t.Errorf("the halt must name the reconcile command `evolve sync-main` so the stop comes with a next step; detail=%q", found.Detail)
	}
}
