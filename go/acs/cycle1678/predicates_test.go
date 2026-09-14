//go:build acs

// Package cycle1678 materializes the acceptance criteria of the TWO inbox
// items this fleet lane committed (lane-scope.json todo_ids ∩
// triage-report.md ## top_n) — and nothing else (R9.3):
//
//	inbox-console-worklist-view  (priority M, weight 0.70, code)
//	ledger-verify-seal-anchor    (priority M, weight 0.70, code)
//
// The lane's third scoped id, kb-graph-projector, was triage-DEFERRED and gets
// ZERO predicates here (a predicate gating deferred work starves the committed
// task — cycle-280).
//
// # The two gaps
//
// inbox-console-worklist-view (ADR-0074 F8). Routing authority is already
// typed plumbing: inboxbatch.PartitionConsole splits the backlog into
// lane-dispatchable and console-routed (operator-owned) with one reason per
// routed item, triage consumes it (triage.go inboxBatchesSection), and
// inboxmover.Claim refuses a console-routed draw with exit 3. The OPERATOR's
// own command does not: cmd_inbox.go runs Classify over EVERY loaded item, so
// `evolve inbox batches` presents operator-owned work as a selectable lane
// batch with no reason and no separation. Measured on this worktree before the
// change, a mixed fixture renders:
//
//	3 items -> 2 batches
//	- batch 1 (weight 0.96; campaign camp-x): operator-work, lane-work
//	- batch 2 (weight 0.92; no shared signal): role-gate-fix
//
// — both console-routed items inside the selectable listing. That is the RED.
//
// ledger-verify-seal-anchor. The work is CARRIED in this worktree by the
// cycle-1677 continuation snapshot (ADR-0076) and is absent from main, so this
// cycle is the one that ships it. `go/acs/cycle1677` is not in the regression
// set and therefore does not run this cycle; re-binding the criteria here is
// the only way THIS cycle's gate proves the ledger behaviour it merges.
//
// # AC map (1:1 with test-report.md ## AC-Materialization)
//
//	A1  evolve inbox output separates console-routed items with reasons
//	    (route field + protected-files derivation)          → 001, 002, 003, 011
//	A2  console_routed is a TERMINAL bucket — an excluded item is
//	    accounted once, never re-deferred per cycle         → 004
//	A3  go test -race on the touched package passes         → 005
//	B1  break → seal → valid tail verifies with an informational
//	    sealed-prefix note; a break AFTER the last seal is BROKEN → 006, 007, 008
//	B2  the live repository ledger reports its post-seal status → 009
//	B3  go test -race ./internal/adapters/ledger passes      → 010
//
// # Adversarial axes (skills/adversarial-testing §6)
//
// NEGATIVE. 002 is the anti-no-op killer for the inbox half: an implementation
// that prints a console header unconditionally, or that sweeps EVERY item out
// of the lane listing, greens 001 and fails 002. 007 is the killer for the
// ledger half: an implementation that unconditionally prints a sealed-prefix
// line and exits 0 greens 006/009 and fails 007. 004's third arm refuses a
// blanket-refusing claim floor.
//
// EDGE/OOD. 003 covers the empty inbox, an unknown argument, and a --max with
// no value. 008 covers a chain with no anchor at all. 007 covers a tail forged
// one line past the anchor.
//
// SEMANTIC. Separation (001), non-separation when nothing is routed (002),
// argument/empty behaviour (003), terminal-bucket enforcement (004), race
// cleanliness (005, 010), sealed-prefix provenance (006), refusal (007),
// derivation-not-literal (008), live corpus + read-only-ness (009), and
// text/JSON path parity (011) — ten distinct behaviours, not one restated.
//
// # Flaky-shape contract
//
// No `./...` sweep and no banned suite: the only nested `go test` invocations
// are ONE named package each (./internal/inboxbatch, ./internal/adapters/ledger
// — measured 1.4s and 14.4s on this worktree), never ./internal/core or
// ./cmd/evolve. No wall-clock bounds; the one contended read (the LIVE ledger
// in 009) is a stat-stable snapshot with retry, which is a STATE poll. No
// literal PIDs. No bare `git` or `go` resolving cwd — every invocation is
// -C anchored. No un-reaped load generators. The CLI is built ONCE in TestMain
// and every assertion runs that binary.
//
// # Reachability probe (cycle-644 rule)
//
// This package imports only pkg/acsassert and the standard library — a leaf
// that pins no import edge and names no internal symbol. The Builder is free
// to choose each seam's shape (a new render helper, a partition-aware Config,
// a second method); the frozen contract is the CLI's observable output, which
// is the surface both items' acceptance criteria are written against.
package cycle1678

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648/1659/1666/1677 TestMain
// shape).
// ---------------------------------------------------------------------------

var (
	evolveBin      string
	evolveBuildErr error
	repoRoot       string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1678-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1678: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
		repoRoot = root
		out, berr := exec.Command("go", "-C", filepath.Join(root, "go"), "build", "-o", evolveBin, "./cmd/evolve").CombinedOutput()
		if berr != nil {
			evolveBuildErr = fmt.Errorf("go build ./cmd/evolve: %v\n%s", berr, out)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// repoRootFromCwd mirrors acsassert.RepoRoot for TestMain (no *testing.T yet).
// `git -C` anchors the lookup at the test's own directory rather than at the
// process cwd, which differs between the main tree and each fleet worktree.
func repoRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git -C %s rev-parse --show-toplevel: %v", cwd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runCLI runs the REAL evolve binary with EVOLVE_PROJECT_ROOT pinned to root
// and returns its merged output with that path redacted — root is a random
// tempdir name and must never be able to satisfy (or defeat) a match.
//
// The variable is set EXPLICITLY rather than relying on the cwd fallback:
// inside a live cycle the ambient environment already carries an
// EVOLVE_PROJECT_ROOT pointing at the operator's real tree, and a predicate
// that read THAT would assert against the production backlog.
func runCLI(t *testing.T, root string, args ...string) (out string, code int) {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("evolve CLI unavailable: %v", evolveBuildErr)
	}
	cmd := exec.Command(evolveBin, args...)
	cmd.Dir = root
	env := os.Environ()
	kept := env[:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "EVOLVE_PROJECT_ROOT=") {
			kept = append(kept, kv)
		}
	}
	cmd.Env = append(kept, "EVOLVE_PROJECT_ROOT="+root)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run %s %v: %v", filepath.Base(evolveBin), args, err)
		}
		code = ee.ExitCode()
	}
	return strings.ReplaceAll(stdout.String()+stderr.String(), root, "<root>"), code
}

// ---------------------------------------------------------------------------
// Fixture: a project root whose .evolve/inbox holds a MIXED backlog — one
// plainly dispatchable item, one routed by its explicit route field, and one
// routed by the protected-files derivation (no route field at all).
// ---------------------------------------------------------------------------

const (
	laneID     = "lane-work"
	routedID   = "operator-work"
	derivedID  = "role-gate-fix"
	routeQuiet = "camp-x"

	// protectedPath is a ProtectedSurfaceManifest member (the cycle-1036 burn:
	// the role gate is a surface a lane structurally cannot write). If the
	// manifest ever drops it, 004's second arm fails loudly — that is the pin,
	// asserted through a production caller rather than by importing guards,
	// which keeps this package a leaf.
	protectedPath = "go/internal/guards/role.go"

	// routedReason and derivedReason are inboxbatch.PartitionConsole's OWN
	// reason strings (consoleroute.go: "route:"+route and
	// "protected fix surface: "+tok). Asserting them is asserting that the
	// existing partitioner's result is what reaches the operator — not that a
	// new literal was typed into the renderer.
	routedReason  = "route:console-manual"
	derivedReason = "protected fix surface: " + protectedPath
)

func writeInboxItem(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// mixedInbox is the shape AC-A1 names: "fixture with route field +
// protected-files derivation".
func mixedInbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeInboxItem(t, root, "a.json",
		`{"id":"`+laneID+`","title":"lane item","weight":0.90,"campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	writeInboxItem(t, root, "b.json",
		`{"id":"`+routedID+`","title":"console item","weight":0.96,"route":"console-manual","campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	writeInboxItem(t, root, "c.json",
		`{"id":"`+derivedID+`","title":"protected derivation","weight":0.92,"files":["`+protectedPath+` (allowance)"],"kind":"bug","priority":"medium"}`)
	return root
}

// laneOnlyInbox is the same backlog with NOTHING operator-owned in it — the
// control fixture 002 needs.
func laneOnlyInbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeInboxItem(t, root, "a.json",
		`{"id":"`+laneID+`","title":"lane item","weight":0.90,"campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	writeInboxItem(t, root, "b.json",
		`{"id":"second-lane-work","title":"another lane item","weight":0.80,"campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	return root
}

// laneListingLines returns the lines of the LANE (selectable) listing — the
// "- batch N (weight …): id, id" shape inboxbatch.RenderMarkdown already
// emits. Console-routed items must be rendered OUTSIDE these lines: they are
// not selectable batches, which is the whole point of the separation.
func laneListingLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "- batch ") {
			lines = append(lines, l)
		}
	}
	return lines
}

func laneListingContains(out, id string) bool {
	for _, l := range laneListingLines(out) {
		if containsID(l, id) {
			return true
		}
	}
	return false
}

// containsID matches id as a whole token so "lane-work" does not match inside
// "second-lane-work".
func containsID(s, id string) bool {
	return regexp.MustCompile(`(^|[^0-9A-Za-z_-])` + regexp.QuoteMeta(id) + `([^0-9A-Za-z_-]|$)`).MatchString(s)
}

// ---------------------------------------------------------------------------
// A1 — the operator's own command separates console-routed work, with reasons.
// ---------------------------------------------------------------------------

// TestC1678_001_InboxBatchesSeparatesConsoleRoutedItemsWithReasons drives the
// production CLI (`evolve inbox batches` → runInbox, cmd_inbox.go) over the
// mixed backlog. Both routing rules must bite — the explicit route field AND
// the protected-files derivation — and each routed item must arrive with the
// partitioner's reason, so the exclusion is loud rather than a silently
// narrowed backlog that reads as full coverage.
func TestC1678_001_InboxBatchesSeparatesConsoleRoutedItemsWithReasons(t *testing.T) {
	root := mixedInbox(t)
	out, code := runCLI(t, root, "inbox", "batches")
	if code != 0 {
		t.Fatalf("RED: `evolve inbox batches` must succeed on a mixed backlog; exit=%d output=%q", code, out)
	}

	// The dispatchable item is still offered.
	if !laneListingContains(out, laneID) {
		t.Errorf("RED: dispatchable item %q missing from the lane listing; got %q", laneID, out)
	}

	// Neither operator-owned item may sit in the selectable listing.
	for _, id := range []string{routedID, derivedID} {
		if laneListingContains(out, id) {
			t.Errorf("RED: console-routed item %q is still rendered as a selectable lane batch — "+
				"render it OUTSIDE the `- batch …` listing (it is operator-owned, not a batch a lane may draw); got %q",
				id, out)
		}
	}

	// Each must appear SOMEWHERE, with its reason: excluded loudly, not dropped.
	for _, tc := range []struct{ id, reason string }{
		{routedID, routedReason},
		{derivedID, derivedReason},
	} {
		if !containsID(out, tc.id) {
			t.Errorf("RED: console-routed item %q vanished from the output entirely — "+
				"an operator worklist must SHOW operator-owned work, not hide it; got %q", tc.id, out)
			continue
		}
		if !strings.Contains(out, tc.reason) {
			t.Errorf("RED: console-routed item %q is listed without inboxbatch.PartitionConsole's reason %q; got %q",
				tc.id, tc.reason, out)
		}
	}
}

// TestC1678_002_LaneOnlyInboxKeepsEveryItemInTheBatchListing is the anti-no-op
// axis. Two implementations green 001 without doing the work: one that prints
// every id under a console heading, and one that prints a console heading with
// ids duplicated from the lane listing. Over a backlog with NOTHING
// operator-owned, every id must appear ONLY inside the selectable listing —
// a partition that routes work nothing routed is as wrong as one that routes
// nothing at all.
func TestC1678_002_LaneOnlyInboxKeepsEveryItemInTheBatchListing(t *testing.T) {
	root := laneOnlyInbox(t)
	out, code := runCLI(t, root, "inbox", "batches")
	if code != 0 {
		t.Fatalf("RED: `evolve inbox batches` must succeed on a lane-only backlog; exit=%d output=%q", code, out)
	}

	for _, id := range []string{laneID, "second-lane-work"} {
		if !laneListingContains(out, id) {
			t.Errorf("RED: dispatchable item %q missing from the lane listing; got %q", id, out)
		}
		// Count occurrences: exactly the lane listing, nowhere else.
		total := len(regexp.MustCompile(`(^|[^0-9A-Za-z_-])`+regexp.QuoteMeta(id)+`([^0-9A-Za-z_-]|$)`).FindAllString(out, -1))
		inLane := 0
		for _, l := range laneListingLines(out) {
			inLane += len(regexp.MustCompile(`(^|[^0-9A-Za-z_-])`+regexp.QuoteMeta(id)+`([^0-9A-Za-z_-]|$)`).FindAllString(l, -1))
		}
		if total != inLane {
			t.Errorf("RED: nothing in this backlog is console-routed, yet %q appears %d time(s) outside the "+
				"selectable listing — the console section must be conditional on a real partition result; got %q",
				id, total-inLane, out)
		}
	}

	// Neither reason string may be invented where no item carries it.
	for _, reason := range []string{routedReason, derivedReason} {
		if strings.Contains(out, reason) {
			t.Errorf("RED: output claims routing reason %q over a backlog with no console-routed item; got %q", reason, out)
		}
	}
}

// TestC1678_003_InboxBatchesEdgeAndInvalidArgumentBehavior pins the boundary
// cases the rendering change must not regress: an empty inbox stays quiet and
// green, and a malformed invocation is refused with the usage exit code
// instead of printing a half-rendered worklist.
func TestC1678_003_InboxBatchesEdgeAndInvalidArgumentBehavior(t *testing.T) {
	t.Run("empty inbox", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		out, code := runCLI(t, root, "inbox", "batches")
		if code != 0 {
			t.Fatalf("RED: an empty inbox is not an error; exit=%d output=%q", code, out)
		}
		if lines := laneListingLines(out); len(lines) != 0 {
			t.Errorf("RED: empty inbox rendered %d batch line(s): %q", len(lines), lines)
		}
		for _, reason := range []string{routedReason, derivedReason} {
			if strings.Contains(out, reason) {
				t.Errorf("RED: empty inbox claims routing reason %q; got %q", reason, out)
			}
		}
	})

	t.Run("unknown argument", func(t *testing.T) {
		root := mixedInbox(t)
		out, code := runCLI(t, root, "inbox", "batches", "--console-only")
		if code != 10 {
			t.Errorf("RED: an unknown argument must exit 10 (usage), got %d; output=%q", code, out)
		}
		if len(laneListingLines(out)) != 0 {
			t.Errorf("RED: a refused invocation still rendered a worklist; got %q", out)
		}
	})

	t.Run("--max without a value", func(t *testing.T) {
		root := mixedInbox(t)
		out, code := runCLI(t, root, "inbox", "batches", "--max")
		if code != 10 {
			t.Errorf("RED: `--max` with no value must exit 10 (usage), got %d; output=%q", code, out)
		}
	})
}

// ---------------------------------------------------------------------------
// A2 — console_routed is a TERMINAL bucket, enforced where it counts.
// ---------------------------------------------------------------------------

// TestC1678_004_ConsoleRoutedItemsAreUnclaimableByALane proves the accounting
// half of AC-A2 at the enforcement backstop rather than in prose: an item a
// lane can never CLAIM cannot be re-bucketed as "deferred" cycle after cycle,
// because it never enters a cycle at all. Both routing rules are driven
// through the real `evolve inbox-mover claim` (runInboxMover → inboxmover.Claim,
// exit 3 = ErrConsoleRouted), and the third arm is the negative control: a
// claim floor that refused everything would green the first two.
//
// The derived arm doubles as the ProtectedSurfaceManifest pin — if
// go/internal/guards/role.go ever leaves the manifest this fails loudly, which
// is the pin move surfacing rather than a silent pass.
func TestC1678_004_ConsoleRoutedItemsAreUnclaimableByALane(t *testing.T) {
	for _, tc := range []struct {
		name     string
		id       string
		wantCode int
		wantIn   string
	}{
		{"explicit route field", routedID, 3, routedReason},
		{"protected-files derivation", derivedID, 3, derivedReason},
		{"dispatchable item still claims", laneID, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := mixedInbox(t)
			out, code := runCLI(t, root, "inbox-mover", "claim", tc.id, "1678")
			if code != tc.wantCode {
				t.Fatalf("RED: claim %q exit=%d, want %d — a console-routed item must be refused (3) and a "+
					"dispatchable one claimed (0); output=%q", tc.id, code, tc.wantCode, out)
			}
			if tc.wantIn != "" && !strings.Contains(out, tc.wantIn) {
				t.Errorf("RED: the refusal of %q does not state why (want %q); got %q", tc.id, tc.wantIn, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// A3 / B3 — the touched packages stay race-clean.
// ---------------------------------------------------------------------------

// runNamedPackageRaceTest runs the race detector over ONE named package.
// Never a ./... sweep and never a known 40s+ suite (./internal/core,
// ./cmd/evolve) — see the package doc's flaky-shape contract. `go -C` anchors
// the module root so the lane worktree's cwd is irrelevant.
func runNamedPackageRaceTest(t *testing.T, pkg string) {
	t.Helper()
	if repoRoot == "" {
		t.Fatalf("repo root unresolved: %v", evolveBuildErr)
	}
	out, err := exec.Command("go", "-C", filepath.Join(repoRoot, "go"),
		"test", "-race", "-count=1", pkg).CombinedOutput()
	if err != nil {
		t.Errorf("RED: go test -race -count=1 %s failed: %v\n%s", pkg, err, out)
	}
}

// TestC1678_005_InboxbatchPackagePassesUnderRace is AC-A3's race half: the
// partition the rendering change reuses must stay race-clean while it gains a
// second consumer.
func TestC1678_005_InboxbatchPackagePassesUnderRace(t *testing.T) {
	runNamedPackageRaceTest(t, "./internal/inboxbatch")
}

// TestC1678_010_LedgerPackagePassesUnderRace is AC-B3, verbatim from the inbox
// record ("cd go && go test -race ./internal/adapters/ledger/... N/N PASS"),
// narrowed from the `/...` spelling to the ONE package it names.
func TestC1678_010_LedgerPackagePassesUnderRace(t *testing.T) {
	runNamedPackageRaceTest(t, "./internal/adapters/ledger")
}

// ---------------------------------------------------------------------------
// Ledger fixtures: break → eligible operator seal → valid tail.
// ---------------------------------------------------------------------------

// zeroSeed is ledger.ZeroSeed (the genesis prev_hash), duplicated as a literal
// to keep this ACS package a leaf — see the package doc's reachability note.
const zeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// Distinctive seal entry_seqs: long enough that neither can collide with a
// random tempdir suffix, and different from each other so 008 can prove the
// provenance is derived from the ledger rather than printed from a literal.
const (
	seqA = 771301
	seqB = 881402
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func entryLine(fields map[string]any) []byte {
	base := map[string]any{
		"ts":        "2026-09-14T00:00:00Z",
		"cycle":     0,
		"role":      "phase",
		"kind":      "phase",
		"exit_code": 0,
	}
	for k, v := range fields {
		base[k] = v
	}
	b, err := json.Marshal(base)
	if err != nil { // unreachable: every value is a plain scalar
		panic(err)
	}
	return b
}

type ledgerFixture struct {
	dir     string
	sealSeq int
	sealSHA string
	sealed  bool
}

// writeLedgerFixture materialises a ledger directory.
//
//	sealed=true    : genesis, a valid line, THE BREAK, an operator reset-seal
//	                 that chains from the break (so it is eligible to move the
//	                 epoch anchor forward), then a valid tail.
//	brokenTail=true: the tail's prev_hash is forged — a break one line PAST the
//	                 last eligible seal, which no seal may ever excuse.
//	sealed=false   : a clean, fully strict chain with no seal anywhere.
func writeLedgerFixture(t *testing.T, sealSeq int, sealed, brokenTail bool) ledgerFixture {
	t.Helper()
	dir := t.TempDir()
	l0 := entryLine(map[string]any{"entry_seq": 0, "prev_hash": zeroSeed})
	l1 := entryLine(map[string]any{"entry_seq": 1, "prev_hash": sha256Hex(l0)})
	lines := [][]byte{l0, l1}

	fx := ledgerFixture{dir: dir, sealed: sealed}
	lastSeq := 2
	if !sealed {
		lines = append(lines, entryLine(map[string]any{"entry_seq": 2, "prev_hash": sha256Hex(l1)}))
	} else {
		// The adjudicated historical damage: a forged prev_hash the operator
		// accepted and PRESERVED rather than rewrote.
		l2 := entryLine(map[string]any{"entry_seq": 2, "prev_hash": strings.Repeat("de", 32)})
		seal := entryLine(map[string]any{
			"entry_seq": sealSeq,
			"role":      "operator",
			"kind":      fmt.Sprintf("reset-seal-cycle-%d", sealSeq),
			"prev_hash": sha256Hex(l2),
		})
		tailPrev := sha256Hex(seal)
		if brokenTail {
			tailPrev = strings.Repeat("ca", 32)
		}
		lines = append(lines, l2, seal, entryLine(map[string]any{"entry_seq": sealSeq + 1, "prev_hash": tailPrev}))
		fx.sealSeq, fx.sealSHA, lastSeq = sealSeq, sha256Hex(seal), sealSeq+1
	}

	var raw bytes.Buffer
	for _, l := range lines {
		raw.Write(l)
		raw.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), raw.Bytes(), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
	tip := fmt.Sprintf("%d:%s", lastSeq, sha256Hex(lines[len(lines)-1]))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write ledger.tip: %v", err)
	}
	return fx
}

func runVerify(t *testing.T, evolveDir string, extra ...string) (out string, code int) {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("evolve CLI unavailable: %v", evolveBuildErr)
	}
	args := append([]string{"ledger", "verify", "--evolve-dir", evolveDir}, extra...)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBin, args...)
	if err != nil && code == 0 {
		t.Fatalf("evolve ledger verify: %v", err)
	}
	return strings.ReplaceAll(stdout+stderr, evolveDir, "<evolve-dir>"), code
}

// The anchor must be named by one of the two stable identities the ledger
// gives that line: its entry_seq (what `evolve ledger anchor` speaks) or its
// bound line SHA. Either is accepted, and both are derived from the ledger's
// own bytes — so no literal in the source can satisfy them for two different
// fixtures, which is what 008 proves.
func namesSeq(out string, seq int) bool {
	return regexp.MustCompile(`(^|[^0-9])` + strconv.Itoa(seq) + `([^0-9]|$)`).MatchString(out)
}

func namesSHA(out, sha string) bool {
	return len(sha) >= 12 && strings.Contains(out, sha[:12])
}

func namesAnchor(out string, seq int, sha string) bool {
	return namesSeq(out, seq) || namesSHA(out, sha)
}

// ---------------------------------------------------------------------------
// B1 — a sealed prefix verifies and says so; a break past the seal does not.
// ---------------------------------------------------------------------------

// TestC1678_006_SealedPrefixVerifiesAndNamesItsAnchor is AC-B1's positive
// half: a break covered by an eligible operator seal verifies, and the success
// states WHICH history it accepted instead of the one-string-for-two-claims
// shape that let the ledger-1740 damage stay invisible.
func TestC1678_006_SealedPrefixVerifiesAndNamesItsAnchor(t *testing.T) {
	fx := writeLedgerFixture(t, seqA, true, false)
	out, code := runVerify(t, fx.dir)
	if code != 0 {
		t.Fatalf("RED: a ledger whose break is covered by an eligible operator seal must verify; exit=%d output=%q", code, out)
	}
	if !namesAnchor(out, fx.sealSeq, fx.sealSHA) {
		t.Errorf("RED: successful verify does not state the sealed prefix — it must name the epoch anchor "+
			"(entry_seq=%d or line sha %s…); got %q", fx.sealSeq, fx.sealSHA[:12], out)
	}
}

// TestC1678_007_BreakAfterTheLastSealStillReportsBroken is the safety half and
// the anti-no-op killer: an implementation that unconditionally prints a
// sealed-prefix note and exits 0 greens 006 and 009 and dies here. A seal
// covers the prefix BEHIND it, never the tail ahead of it — otherwise the
// preservation remedy becomes a chain-integrity bypass.
func TestC1678_007_BreakAfterTheLastSealStillReportsBroken(t *testing.T) {
	fx := writeLedgerFixture(t, seqA, true, true)
	out, code := runVerify(t, fx.dir)
	if code == 0 {
		t.Fatalf("RED: a break one line PAST the last eligible seal must NOT verify; exit=0 output=%q", out)
	}
	if code != 2 {
		t.Errorf("RED: a broken chain must exit 2 (chain-broken), got %d; output=%q", code, out)
	}
	if !strings.Contains(strings.ToUpper(out), "BROKEN") {
		t.Errorf("RED: the refusal must say the chain is BROKEN; got %q", out)
	}
}

// TestC1678_008_AnchorProvenanceIsDerivedFromTheLedger is the anti-hardcode
// axis. Two fixtures differ in NOTHING but the identity of their seal, and a
// third has no seal at all. A literal in the success path passes at most one
// of the three arms.
func TestC1678_008_AnchorProvenanceIsDerivedFromTheLedger(t *testing.T) {
	a := writeLedgerFixture(t, seqA, true, false)
	b := writeLedgerFixture(t, seqB, true, false)

	outA, codeA := runVerify(t, a.dir)
	outB, codeB := runVerify(t, b.dir)
	if codeA != 0 || codeB != 0 {
		t.Fatalf("RED: both sealed fixtures must verify; exits %d/%d\nA=%q\nB=%q", codeA, codeB, outA, outB)
	}
	if !namesAnchor(outA, a.sealSeq, a.sealSHA) || !namesAnchor(outB, b.sealSeq, b.sealSHA) {
		t.Fatalf("RED: each success must name its OWN anchor (A: seq=%d/%s…, B: seq=%d/%s…)\nA=%q\nB=%q",
			a.sealSeq, a.sealSHA[:12], b.sealSeq, b.sealSHA[:12], outA, outB)
	}
	// The cross-check: A must not claim B's identity, and vice versa.
	if namesAnchor(outA, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: fixture A's output carries fixture B's anchor identity — the provenance is a literal, not derived; got %q", outA)
	}
	if namesAnchor(outB, a.sealSeq, a.sealSHA) {
		t.Errorf("RED: fixture B's output carries fixture A's anchor identity — the provenance is a literal, not derived; got %q", outB)
	}

	// The strict arm: a chain with NO anchor anywhere validated every byte and
	// must claim no sealed prefix at all.
	strict := writeLedgerFixture(t, 0, false, false)
	outS, codeS := runVerify(t, strict.dir)
	if codeS != 0 {
		t.Fatalf("RED: a clean chain with no seal must verify; exit=%d output=%q", codeS, outS)
	}
	for _, seq := range []int{seqA, seqB} {
		if namesSeq(outS, seq) {
			t.Errorf("RED: a fully strict verification claims an epoch anchor (entry_seq=%d) that this ledger does not have; got %q", seq, outS)
		}
	}
}

// ---------------------------------------------------------------------------
// B2 — the REAL repository ledger: reports its post-seal status, writes nothing.
// ---------------------------------------------------------------------------

// ledgerFiles are the inputs a non-deep verify reads: the chain, its tip, and
// the out-of-band epoch anchor. Sealed segments are --deep-only and are
// deliberately not part of this snapshot.
var ledgerFiles = []string{"ledger.jsonl", "ledger.tip", "ledger-anchor.json"}

// liveEvolveDir resolves the AUTHORITATIVE .evolve directory. In a fleet
// worktree .evolve/ledger.jsonl is a symlink into the operator's runtime tree
// (and the tip / sidecar exist only there), so the link is followed and its
// parent is the real directory.
func liveEvolveDir(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	real, err := filepath.EvalSymlinks(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Skipf("no live ledger to verify (%v)", err)
	}
	return filepath.Dir(real)
}

type fileStamp struct {
	size    int64
	modUnix int64
}

func stampOf(path string) (fileStamp, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{size: fi.Size(), modUnix: fi.ModTime().UnixNano()}, nil
}

// snapshotLiveLedger copies the live ledger into dst and returns the copied
// bytes' SHAs. The live file is appended to by concurrent fleet lanes, so the
// copy is taken between two identical stat readings: a snapshot that raced an
// append is discarded and retaken, never asserted on. This is a STATE poll,
// not a wall-clock bound.
func snapshotLiveLedger(t *testing.T, src, dst string) map[string]string {
	t.Helper()
	for attempt := 1; attempt <= 6; attempt++ {
		before, err := stampOf(filepath.Join(src, "ledger.jsonl"))
		if err != nil {
			t.Skipf("no live ledger.jsonl (%v)", err)
		}
		shas := map[string]string{}
		for _, name := range ledgerFiles {
			raw, rerr := os.ReadFile(filepath.Join(src, name))
			if rerr != nil {
				if os.IsNotExist(rerr) && name == "ledger-anchor.json" {
					continue // no sidecar anchor: full-strict verification
				}
				t.Skipf("live ledger incomplete: %v", rerr)
			}
			if werr := os.WriteFile(filepath.Join(dst, name), raw, 0o644); werr != nil {
				t.Fatalf("snapshot %s: %v", name, werr)
			}
			shas[name] = sha256Hex(raw)
		}
		after, err := stampOf(filepath.Join(src, "ledger.jsonl"))
		if err != nil {
			t.Skipf("live ledger vanished mid-snapshot (%v)", err)
		}
		if before == after {
			return shas
		}
		t.Logf("snapshot attempt %d raced a concurrent append; retaking", attempt)
	}
	t.Fatalf("could not take a stable snapshot of %s/ledger.jsonl in 6 attempts", src)
	return nil
}

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	for _, l := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

// isEligibleSeal reports whether line is an operator reset-seal that is itself
// hash-valid from its predecessor (the seal TRUST GUARD: a seal that cannot
// prove its own linkage may never move the anchor forward).
func isEligibleSeal(line []byte, prevLineSHA string) bool {
	if !bytes.Contains(line, []byte(`"role":"operator"`)) {
		return false // cheap filter: 141k lines, ~200 candidates
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(line, &raw) != nil {
		return false
	}
	if _, hasPrev := raw["prev_hash"]; !hasPrev {
		return false
	}
	var e struct {
		Role       string `json:"role"`
		Kind       string `json:"kind"`
		CycleLabel string `json:"cycle_label"`
		PrevHash   string `json:"prev_hash"`
	}
	if json.Unmarshal(line, &e) != nil || e.Role != "operator" {
		return false
	}
	if !strings.HasPrefix(e.Kind, "reset-seal-") && !strings.HasPrefix(e.CycleLabel, "reset-seal-") {
		return false
	}
	if prevLineSHA == "" {
		return e.PrevHash == zeroSeed
	}
	return e.PrevHash == prevLineSHA
}

// effectiveAnchor mirrors ledger.effectiveAnchorSHA over the snapshot: the LAST
// of (the sidecar anchor line, any self-chaining operator reset-seal at or
// after it). Deriving the expectation from the same bytes the CLI reads is what
// makes this predicate ungameable — the answer is written down nowhere.
func effectiveAnchor(lines [][]byte, sidecarSHA string) (sha string, seq int, found bool) {
	anchorSHA := sidecarSHA
	anchorIdx := -1
	reached := sidecarSHA == ""
	prevLineSHA := ""
	for i, line := range lines {
		lineSHA := sha256Hex(line)
		switch {
		case !reached && lineSHA == sidecarSHA:
			reached = true
			anchorIdx = i
		case !reached:
			// still inside the untrusted prefix
		default:
			if isEligibleSeal(line, prevLineSHA) {
				anchorSHA, anchorIdx = lineSHA, i
			}
		}
		prevLineSHA = lineSHA
	}
	if anchorSHA == "" || anchorIdx < 0 {
		return "", 0, false
	}
	var e struct {
		EntrySeq int `json:"entry_seq"`
	}
	if err := json.Unmarshal(lines[anchorIdx], &e); err != nil {
		return anchorSHA, 0, true
	}
	return anchorSHA, e.EntrySeq, true
}

// TestC1678_009_LiveLedgerVerifiesReportsItsScopeAndIsNotMutated is AC-B2: on
// the operator's real ledger, `evolve ledger verify` must report the post-seal
// chain status instead of the line-1740 wolf-cry. It asserts three things —
// the chain verifies, the success states the scope it verified (the anchor
// identity derived independently from the same bytes), and the read wrote
// nothing.
func TestC1678_009_LiveLedgerVerifiesReportsItsScopeAndIsNotMutated(t *testing.T) {
	src := liveEvolveDir(t)
	snap := t.TempDir()
	before := snapshotLiveLedger(t, src, snap)

	raw, err := os.ReadFile(filepath.Join(snap, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	sidecar := ""
	if anchorRaw, aerr := os.ReadFile(filepath.Join(snap, "ledger-anchor.json")); aerr == nil {
		var a struct {
			AnchorLineSHA string `json:"anchor_line_sha256"`
		}
		if json.Unmarshal(anchorRaw, &a) == nil {
			sidecar = a.AnchorLineSHA
		}
	}
	wantSHA, wantSeq, haveAnchor := effectiveAnchor(splitLines(raw), sidecar)

	out, code := runVerify(t, snap)
	if code != 0 {
		t.Fatalf("RED: the real ledger must verify (the line-1740 damage is adjudicated and sealed); exit=%d output=%q", code, out)
	}
	if haveAnchor {
		if !namesAnchor(out, wantSeq, wantSHA) {
			t.Errorf("RED: the real ledger is verified FROM an epoch anchor (entry_seq=%d / %s…) but the successful "+
				"output does not state that scope — that is the wolf-cry's mirror image, a silent trust claim; got %q",
				wantSeq, wantSHA[:12], out)
		}
	} else {
		t.Logf("live ledger currently has no epoch anchor — full-strict verification; the scope assertion is 008's strict arm")
	}

	// Verify is a READER: the snapshot it was pointed at must be byte-identical
	// afterwards (no rewrite, no re-hash, no tip fixup).
	for name, sha := range before {
		got, rerr := os.ReadFile(filepath.Join(snap, name))
		if rerr != nil {
			t.Errorf("RED: verify removed %s: %v", name, rerr)
			continue
		}
		if sha256Hex(got) != sha {
			t.Errorf("RED: verify MUTATED %s (sha %s → %s) — ledger history must never be rewritten by a read",
				name, sha[:12], sha256Hex(got)[:12])
		}
	}
}

// ---------------------------------------------------------------------------
// House rule 2 — every path the seam claims.
// ---------------------------------------------------------------------------

// TestC1678_011_JSONPathCarriesTheSamePartitionAsTheTextPath pins the OTHER
// production path through `evolve inbox batches`. `--json` is a separate
// branch in runInbox (cmd_inbox.go), and a partition wired only into the text
// renderer would leave every machine consumer reading operator-owned work as
// dispatchable — the wired-into-one-path-only defect (#373).
//
// The assertion is on the partitioner's REASONS rather than on a JSON schema,
// so the Builder stays free to shape the document: today the raw item fields
// (`"route": "console-manual"`, the files entry) are present but PartitionConsole's
// reasons are not, which is exactly the difference between dumping items and
// reporting a routing decision.
func TestC1678_011_JSONPathCarriesTheSamePartitionAsTheTextPath(t *testing.T) {
	root := mixedInbox(t)
	out, code := runCLI(t, root, "inbox", "batches", "--json")
	if code != 0 {
		t.Fatalf("RED: `evolve inbox batches --json` must succeed; exit=%d output=%q", code, out)
	}
	if !json.Valid([]byte(strings.TrimSpace(stdoutOnly(out)))) {
		t.Errorf("RED: --json output is not valid JSON; got %q", out)
	}
	for _, tc := range []struct{ id, reason string }{
		{routedID, routedReason},
		{derivedID, derivedReason},
	} {
		if !strings.Contains(out, tc.reason) {
			t.Errorf("RED: the --json path does not carry the routing reason for %q (want %q) — the partition is "+
				"wired into the text renderer only, so machine consumers still read operator-owned work as dispatchable; got %q",
				tc.id, tc.reason, out)
		}
	}
	// The control: the lane item must still be present in the document.
	if !containsID(out, laneID) {
		t.Errorf("RED: dispatchable item %q missing from the --json document; got %q", laneID, out)
	}
}

// stdoutOnly trims the leading WARN/usage lines the command writes to stderr
// so the JSON validity check reads the document, not the diagnostics. The
// merged capture keeps diagnostics visible in every failure message.
func stdoutOnly(out string) string {
	if i := strings.IndexAny(out, "[{"); i >= 0 {
		return out[i:]
	}
	return out
}
