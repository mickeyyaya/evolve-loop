//go:build acs

// Package cycle1433 encodes the cycle-1433 ACS predicates for the two tasks
// derived from inbox item `ledger-fleet-concurrency-chain`:
//
//   - ledger-anchor-reject-nonunique-seq — `FileLedger.Anchor` binds the FIRST
//     line whose entry_seq matches (anchor.go:173 self-documents the sibling
//     ambiguity it does not reject), so `evolve ledger anchor <seq>` can silently
//     bind an EARLIER sibling and regress the epoch anchor backward.
//   - ledger-rebaseline-command — no `evolve ledger rebaseline` exists anywhere in
//     go/, which is why the console-plane ledger (~180+ dense breaks) was left
//     broken rather than repaired by 180 sequential `anchor` calls.
//
// Predicate shape notes:
//   - Every predicate drives the REAL production entry point (the compiled
//     `evolve` binary, `go/cmd/evolve`) against a synthetic ledger in t.TempDir().
//     No predicate greps source for a magic string as a load-bearing assertion
//     (the cycle-85 ban), and none calls the ledger package directly — a seam
//     reachable only from a unit test is dead code (the wiring-proof rule).
//   - No `./...` sweep, no whole-package `go test`, no wall-clock bound, no
//     literal PID, no bare `git`, no load generator (the flaky-shape rules that
//     produced the 1173/1175/1178 false-REDs). The single subprocess cost is one
//     `go build ./cmd/evolve`, done once for the whole package.
//   - The rebaseline NEGATIVE predicate (005) explicitly refuses to pass on
//     "unknown subcommand": without that guard it would green vacuously today,
//     while the command does not exist at all.
package cycle1433

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// zeroSeed is the prev_hash of a genesis ledger line (ledger.ZeroSeed). Copied
// as a literal rather than imported so this predicate package stays a leaf.
const zeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// operatorNote is the sign-off text a rebaseline must record. It is checked for
// verbatim in the post-state so a rebaseline that discards the operator's reason
// (and therefore leaves an unattributable trust decision in the audit trail)
// fails.
const operatorNote = "operator sign-off: cycle-1433 fixture reset baseline"

var (
	buildOnce sync.Once
	binPath   string
	buildOut  string
	buildCode int
)

// evolveBin compiles the real `evolve` binary once per test binary and returns
// its path. Driving the compiled CLI — not the library — is what makes these
// predicates wiring proofs: a `Rebaseline` method with no reachable production
// caller leaves them RED.
func evolveBin(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "acs1433-bin-")
		if err != nil {
			buildOut, buildCode = err.Error(), 1
			return
		}
		binPath = filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-C", filepath.Join(root, "go"), "-o", binPath, "./cmd/evolve")
		out, err := cmd.CombinedOutput()
		buildOut = string(out)
		if err != nil {
			buildCode = 1
			if ee, ok := err.(*exec.ExitError); ok {
				buildCode = ee.ExitCode()
			}
		}
	})
	if buildCode != 0 || binPath == "" {
		t.Fatalf("building ./cmd/evolve failed (exit=%d) — the CLI surface these predicates drive does not compile:\n%s", buildCode, buildOut)
	}
	return binPath
}

// runEvolve invokes the compiled binary and returns combined output + exit code.
func runEvolve(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(evolveBin(t), args...)
	// Set Dir explicitly: a bare invocation resolves relative paths from process
	// cwd, which differs between the main tree, the worktree, and each fleet lane.
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return string(out), code
}

func lineSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ledgerLine is one synthetic ledger line spec.
type ledgerLine struct {
	seq int
	// prevOverride, when non-empty, replaces the correctly-chained prev_hash —
	// this is how a chain BREAK is planted.
	prevOverride string
	// tag varies the line bytes so two lines sharing a seq are distinct SHAs
	// (the sibling shape a pre-CA.1 concurrent append produced).
	tag string
}

// writeLedger materializes a synthetic ledger.jsonl + ledger.tip in a fresh temp
// dir and returns the dir plus the per-line SHAs (index-aligned with specs).
func writeLedger(t *testing.T, specs []ledgerLine) (dir string, shas []string) {
	t.Helper()
	dir = t.TempDir()
	var buf []byte
	prev := ""
	for i, s := range specs {
		p := s.prevOverride
		if p == "" {
			p = prev
			if p == "" {
				p = zeroSeed
			}
		}
		rec := map[string]any{
			"ts":         "2026-08-11T00:00:0" + string(rune('0'+i%10)) + "Z",
			"cycle":      1433,
			"role":       "acs-fixture",
			"kind":       "fixture" + s.tag,
			"exit_code":  0,
			"entry_seq":  s.seq,
			"prev_hash":  p,
			"artifact_p": s.tag,
		}
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshal fixture line %d: %v", i, err)
		}
		sha := lineSHA(b)
		shas = append(shas, sha)
		buf = append(append(buf, b...), '\n')
		prev = sha
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), buf, 0o644); err != nil {
		t.Fatalf("write fixture ledger: %v", err)
	}
	tip := []byte(itoa(specs[len(specs)-1].seq) + ":" + shas[len(shas)-1])
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), tip, 0o644); err != nil {
		t.Fatalf("write fixture tip: %v", err)
	}
	return dir, shas
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}

// ambiguousSpecs is the sibling fixture: entry_seq 2 is carried by TWO distinct
// lines (indices 2 and 3). The chain itself is intact — this is precisely the
// dangerous case, because `evolve ledger anchor 2` exits 0 today while binding
// the EARLIER of the two and regressing the trusted epoch backward.
func ambiguousSpecs() []ledgerLine {
	return []ledgerLine{
		{seq: 0, tag: "genesis"},
		{seq: 1, tag: "a"},
		{seq: 2, tag: "sibling-first"},
		{seq: 2, tag: "sibling-second"},
		{seq: 3, tag: "tail"},
	}
}

// anchorFile reads <dir>/ledger-anchor.json, reporting whether it exists.
func anchorFile(t *testing.T, dir string) (map[string]any, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "ledger-anchor.json"))
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("ledger-anchor.json is not valid JSON: %v\n%s", err, raw)
	}
	return m, true
}

// TestC1433_001_AnchorRejectsAmbiguousSeqWithoutDisambiguation is the primary
// RED contract for task 1.
//
// Two distinct lines carry entry_seq=2. Binding either one silently is a TRUST
// decision the operator never made: picking the earlier sibling moves the epoch
// anchor BACKWARD, re-exposing lines the operator believed were already sealed.
// The command must refuse and write nothing.
//
// Today `Anchor` takes the first match (anchor.go:171-175) and exits 0, so this
// is RED on both the exit code and the written-anchor assertion.
func TestC1433_001_AnchorRejectsAmbiguousSeqWithoutDisambiguation(t *testing.T) {
	dir, _ := writeLedger(t, ambiguousSpecs())

	out, code := runEvolve(t, "ledger", "anchor", "2", "--evolve-dir", dir)
	if code == 0 {
		t.Errorf("`evolve ledger anchor 2` exited 0 on a ledger where entry_seq=2 is carried by TWO distinct lines — it silently bound one of them, which is the backward-regression defect this task closes:\n%s", out)
	}
	if m, ok := anchorFile(t, dir); ok {
		t.Errorf("ledger-anchor.json was written despite the ambiguity (anchor_line_sha256=%v) — an ambiguous anchor must leave NO anchor file, so a failed operator command cannot half-apply a trust decision", m["anchor_line_sha256"])
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "entry_seq=2") && !strings.Contains(low, "entry_seq 2") {
		t.Errorf("the rejection does not name the offending seq — an operator cannot act on it:\n%s", out)
	}
	if !strings.Contains(low, "--line-sha") {
		t.Errorf("the rejection does not tell the operator how to disambiguate (expected it to name --line-sha):\n%s", out)
	}
}

// TestC1433_002_AnchorLineSHADisambiguatesAndBindsTheIntendedLine is the
// positive leg: with the exact line SHA supplied, the anchor binds THAT line —
// specifically the LATER sibling, the one first-match would never have chosen.
// This is what makes the rejection in 001 actionable rather than a dead end.
func TestC1433_002_AnchorLineSHADisambiguatesAndBindsTheIntendedLine(t *testing.T) {
	dir, shas := writeLedger(t, ambiguousSpecs())
	want := shas[3] // the SECOND line carrying entry_seq=2

	out, code := runEvolve(t, "ledger", "anchor", "2", "--line-sha", want, "--evolve-dir", dir)
	if code != 0 {
		t.Fatalf("`evolve ledger anchor 2 --line-sha <sha>` exited %d — the disambiguation path the ambiguity rejection points at does not exist:\n%s", code, out)
	}
	m, ok := anchorFile(t, dir)
	if !ok {
		t.Fatalf("anchor reported success but wrote no ledger-anchor.json in %s", dir)
	}
	if got, _ := m["anchor_line_sha256"].(string); got != want {
		t.Errorf("anchor_line_sha256 = %q, want %q (the SECOND seq-2 line) — the anchor bound a different line than the operator named", got, want)
	}
	if got, _ := m["anchor_seq"].(float64); int(got) != 2 {
		t.Errorf("anchor_seq = %v, want 2", m["anchor_seq"])
	}
	// The disambiguated anchor must actually green the chain forward, otherwise
	// the command "succeeded" into an unusable state.
	if vout, vcode := runEvolve(t, "ledger", "verify", "--evolve-dir", dir); vcode != 0 {
		t.Errorf("verify exited %d after a successful disambiguated anchor — the anchored epoch does not validate forward:\n%s", vcode, vout)
	}
}

// TestC1433_003_AnchorLineSHANegativesAndUnambiguousRegression covers the two
// adversarial edges of the new flag plus the regression floor.
//
// Edge A: a --line-sha that resolves to a line carrying a DIFFERENT seq. Trusting
// it would let a typo'd seq bind an arbitrary line — worse than the bug being
// fixed. Edge B: a --line-sha present in no line at all. Regression: the
// unambiguous, no-flag path (today's common case) must stay byte-identical in
// behavior — this task must not make ordinary anchoring harder.
func TestC1433_003_AnchorLineSHANegativesAndUnambiguousRegression(t *testing.T) {
	// A refusal that is really "no such flag" proves nothing about the flag's
	// validation: without this guard both negative subtests green vacuously
	// today, while --line-sha does not exist at all.
	assertNotAFlagParseError := func(t *testing.T, out, what string) {
		t.Helper()
		if strings.Contains(out, "flag provided but not defined") {
			t.Errorf("%s was rejected by flag parsing, not by the anchor's own validation — --line-sha is not wired yet:\n%s", what, out)
		}
	}

	t.Run("line_sha_carrying_a_different_seq_is_refused", func(t *testing.T) {
		dir, shas := writeLedger(t, ambiguousSpecs())
		out, code := runEvolve(t, "ledger", "anchor", "2", "--line-sha", shas[1] /* seq 1 */, "--evolve-dir", dir)
		if code == 0 {
			t.Errorf("anchor accepted a --line-sha whose line carries entry_seq=1 while <seq> said 2 — the two arguments must agree or the flag becomes a way to bind any line at all:\n%s", out)
		}
		assertNotAFlagParseError(t, out, "a --line-sha carrying a different seq")
		if _, ok := anchorFile(t, dir); ok {
			t.Errorf("a refused --line-sha still wrote ledger-anchor.json — a rejected anchor must leave no residue")
		}
	})

	t.Run("unknown_line_sha_is_refused", func(t *testing.T) {
		dir, _ := writeLedger(t, ambiguousSpecs())
		absent := strings.Repeat("de", 32) // 64 hex chars, present in no line
		out, code := runEvolve(t, "ledger", "anchor", "2", "--line-sha", absent, "--evolve-dir", dir)
		if code == 0 {
			t.Errorf("anchor accepted a --line-sha matching no line in the ledger:\n%s", out)
		}
		assertNotAFlagParseError(t, out, "an unknown --line-sha")
		if _, ok := anchorFile(t, dir); ok {
			t.Errorf("an unknown --line-sha still wrote ledger-anchor.json — a stale/typo'd sidecar makes every later verify fail 'anchor not found'")
		}
	})

	t.Run("unambiguous_seq_without_the_flag_still_anchors", func(t *testing.T) {
		dir, shas := writeLedger(t, ambiguousSpecs())
		out, code := runEvolve(t, "ledger", "anchor", "3", "--evolve-dir", dir)
		if code != 0 {
			t.Fatalf("`evolve ledger anchor 3` (a seq carried by exactly ONE line) exited %d — the ambiguity check regressed the ordinary operator path:\n%s", code, out)
		}
		m, ok := anchorFile(t, dir)
		if !ok {
			t.Fatalf("no ledger-anchor.json written for an unambiguous seq")
		}
		if got, _ := m["anchor_line_sha256"].(string); got != shas[4] {
			t.Errorf("anchor_line_sha256 = %q, want %q — the unambiguous path bound the wrong line", got, shas[4])
		}
	})
}

// damagedSpecs is the console-plane shape in miniature: THREE distinct chain
// breaks in the prefix (indices 2, 3, 5), each a prev_hash that chains from
// nothing, followed by an intact tail. Repairing this with `anchor` takes one
// operator invocation per break — the reason the real console ledger was left
// broken by design.
func damagedSpecs() []ledgerLine {
	return []ledgerLine{
		{seq: 0, tag: "genesis"},
		{seq: 1, tag: "ok-1"},
		{seq: 2, tag: "break-1", prevOverride: strings.Repeat("a1", 32)},
		{seq: 3, tag: "break-2", prevOverride: strings.Repeat("b2", 32)},
		{seq: 4, tag: "ok-2"},
		{seq: 5, tag: "break-3", prevOverride: strings.Repeat("c3", 32)},
		{seq: 6, tag: "tail"},
	}
}

// TestC1433_004_RebaselineSealsADenselyDamagedPrefixInOneCall is the primary RED
// contract for task 2: ONE invocation turns a multi-break ledger from RED to
// GREEN, without deleting a single byte of the damaged history.
//
// The pre-state assertion is load-bearing: without it, a rebaseline that greens
// an already-green fixture would prove nothing.
func TestC1433_004_RebaselineSealsADenselyDamagedPrefixInOneCall(t *testing.T) {
	dir, _ := writeLedger(t, damagedSpecs())
	path := filepath.Join(dir, "ledger.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture ledger: %v", err)
	}

	// Pre-state: the fixture must genuinely be broken, or this predicate is vacuous.
	if out, code := runEvolve(t, "ledger", "verify", "--deep", "--evolve-dir", dir); code == 0 {
		t.Fatalf("fixture precondition failed — a ledger with three planted prev_hash breaks verified GREEN, so this predicate would prove nothing:\n%s", out)
	}

	out, code := runEvolve(t, "ledger", "rebaseline", "--evolve-dir", dir, "--note", operatorNote)
	if code != 0 {
		t.Fatalf("`evolve ledger rebaseline` exited %d — a single-operation seal of a densely damaged prefix is what this task adds:\n%s", code, out)
	}

	if vout, vcode := runEvolve(t, "ledger", "verify", "--deep", "--evolve-dir", dir); vcode != 0 {
		t.Errorf("verify --deep exited %d after ONE rebaseline — the damaged prefix was not sealed:\n%s", vcode, vout)
	}

	// Non-destructive: the pre-rebaseline bytes must remain a byte-identical
	// PREFIX on disk. Truncating or rewriting history would "green" the chain by
	// destroying the auditable record, which is the outcome ADR-0048 rejects.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger after rebaseline: %v", err)
	}
	if len(after) < len(before) || string(after[:len(before)]) != string(before) {
		t.Errorf("rebaseline mutated or truncated pre-existing history (%d bytes before, %d after) — the damaged prefix must be PRESERVED, only un-chain-validated", len(before), len(after))
	}

	// Auditability: the post-state must carry the operator's reason and must be
	// distinguishable from an ordinary `anchor`, so a forensics sweep can tell a
	// deliberate rebaseline from a routine epoch anchor.
	post := string(after)
	if m, ok := anchorFile(t, dir); ok {
		b, _ := json.Marshal(m)
		post += string(b)
	}
	if !strings.Contains(post, operatorNote) {
		t.Errorf("the operator note %q appears nowhere in the post-rebaseline state — the trust decision is recorded without its justification", operatorNote)
	}
	if !strings.Contains(strings.ToLower(post), "rebaseline") {
		t.Errorf("nothing in the post-rebaseline state identifies the record as a rebaseline — it is indistinguishable from an ordinary anchor in the audit trail")
	}
}

// TestC1433_005_RebaselineRefusesUngatedAndEmptyChainInvocations is the
// anti-no-op axis: a command that seals whatever it is pointed at, whenever it is
// invoked, is a chain-integrity bypass rather than a repair tool.
//
// Every subtest asserts the failure is a REAL refusal, not "unknown subcommand" —
// without that guard these would pass vacuously while `rebaseline` does not exist.
func TestC1433_005_RebaselineRefusesUngatedAndEmptyChainInvocations(t *testing.T) {
	assertRealRefusal := func(t *testing.T, out string, code int, what string) {
		t.Helper()
		if code == 0 {
			t.Errorf("%s exited 0 — it must be refused:\n%s", what, out)
		}
		if strings.Contains(strings.ToLower(out), "unknown subcommand") {
			t.Errorf("%s failed only because `rebaseline` is not a subcommand at all — the refusal must come from the command's own gate:\n%s", what, out)
		}
	}

	t.Run("missing_operator_note_is_refused", func(t *testing.T) {
		dir, _ := writeLedger(t, damagedSpecs())
		path := filepath.Join(dir, "ledger.jsonl")
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		out, code := runEvolve(t, "ledger", "rebaseline", "--evolve-dir", dir)
		assertRealRefusal(t, out, code, "rebaseline without --note")
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read ledger after refused rebaseline: %v", err)
		}
		if string(after) != string(before) {
			t.Errorf("an ungated rebaseline still mutated ledger.jsonl (%d -> %d bytes) — a refused trust decision must write nothing", len(before), len(after))
		}
		if _, ok := anchorFile(t, dir); ok {
			t.Errorf("an ungated rebaseline still wrote ledger-anchor.json — a refused trust decision must leave no residue")
		}
	})

	t.Run("empty_chain_is_refused_not_fabricated", func(t *testing.T) {
		dir := t.TempDir() // no ledger.jsonl at all
		out, code := runEvolve(t, "ledger", "rebaseline", "--evolve-dir", dir, "--note", operatorNote)
		assertRealRefusal(t, out, code, "rebaseline against an empty chain")
		if _, err := os.Stat(filepath.Join(dir, "ledger.jsonl")); err == nil {
			t.Errorf("rebaseline fabricated a ledger.jsonl where none existed — sealing nothing must never mint a chain")
		}
		if _, ok := anchorFile(t, dir); ok {
			t.Errorf("rebaseline wrote an anchor for a ledger that does not exist — a later verify would fail 'anchor not found' forever")
		}
	})
}
