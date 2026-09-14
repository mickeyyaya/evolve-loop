//go:build acs

// Package cycle1677 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	ledger-verify-seal-anchor  (priority M, weight 0.70, code)
//
// The lane's two other scoped ids, kb-graph-projector and
// inbox-console-worklist-view, were triage-DEFERRED and get ZERO predicates
// here (a predicate gating deferred work starves the committed task —
// cycle-280).
//
// The gap. The anchor RESOLVER is already correct: effectiveAnchorSHA
// (anchor.go) picks the last self-chaining operator `reset-seal-*` at or after
// the sidecar ledger-anchor.json, and walkChain (ledger.go) resumes STRICT
// validation from that exact line SHA. What is missing is OBSERVABILITY at the
// only surface an operator sees: runLedgerVerify (cmd_ledger.go:52) prints
// `[ledger] OK: chain intact (<dir>/ledger.jsonl)` whether it verified every
// byte from genesis or deliberately trusted a 136k-line adjudicated prefix.
// Those two outcomes are NOT the same claim, and today they are the same
// string. This lane makes a successful verification state the scope it
// actually verified.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1  break → eligible seal → valid tail verifies successfully AND
//	     states the sealed prefix informationally                      → 001, 002
//	AC2  a break AFTER the last eligible seal still returns a
//	     chain-broken error                                            → 003
//	AC3  the real repository ledger verifies and reports its scope
//	     without modifying ledger history                              → 004
//	AC-H2 (house rule 2) every path the seam claims is wired: --deep
//	     reports the same provenance as the default path               → 005
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 003 is the
// anti-no-op killer — an implementation that unconditionally prints a
// sealed-prefix line and exits 0 greens 001/005 and fails 003. 002 is the
// anti-hardcode killer — two fixtures whose ONLY difference is the seal's
// identity must produce two different, each-correct outputs, and a
// no-anchor chain must claim neither. 004's no-mutation half is a negative
// over the real 141k-line ledger: verify is a reader.
// EDGE/OOD: a chain whose damage precedes the seal (001), a chain with no
// anchor at all (002's strict arm), a tail forged one line past the anchor
// (003). SEMANTIC: provenance content (001), provenance derivation (002),
// refusal (003), live-corpus behaviour + read-only-ness (004), path parity
// (005) — five distinct behaviours, not one restated.
//
// Flaky-shape contract: no `go test` sweep (the CLI is built ONCE in TestMain
// and every assertion runs that binary), no wall-clock bounds, no literal
// PIDs, no bare `git` (every call is -C anchored), no un-reaped load. The one
// contended read — the LIVE ledger in 004 — is taken as a stat-stable
// snapshot into t.TempDir() and retried, so a concurrent fleet append can
// never make it a false RED.
//
// Reachability probe (cycle-644 rule): this package imports only
// pkg/acsassert and the standard library — a leaf, pinning no import edge.
// Nothing here names an internal symbol, so the Builder is free to choose the
// seam's shape (a returned report value, an out-param, a second method); the
// frozen contract is the CLI's observable output, which is the surface the
// acceptance criteria are written against.
package cycle1677

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// zeroSeed is ledger.ZeroSeed (the genesis prev_hash), duplicated as a literal
// to keep this ACS package a leaf — see the package doc's reachability note.
const zeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// Distinctive seal entry_seqs: long enough that neither can collide with a
// random tempdir suffix, and different from each other so 002 can prove the
// provenance is derived from the ledger rather than printed from a literal.
const (
	seqA = 770101
	seqB = 880202
)

// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648/1659/1666 TestMain shape).
// ---------------------------------------------------------------------------

var (
	evolveBin      string
	evolveBuildErr error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1677-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1677: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
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

// runVerify runs the REAL `evolve ledger verify` against evolveDir and returns
// its merged output with the directory path redacted — the path is a random
// tempdir name and must never be able to satisfy (or defeat) an identity match.
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

// ---------------------------------------------------------------------------
// Fixture: a ledger whose damage is SEALED — the exact shape the acceptance
// criteria describe (break → eligible operator seal → valid tail).
// ---------------------------------------------------------------------------

type fixture struct {
	dir      string
	sealSeq  int
	sealSHA  string
	tailSeq  int
	hasSeal  bool
	numLines int
}

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

// writeFixture materialises a ledger directory.
//
//	sealed=true  : line 0 genesis, line 1 valid, line 2 THE BREAK, line 3 an
//	               operator reset-seal that chains from line 2 (so it is
//	               eligible to move the epoch anchor forward), line 4 the tail.
//	brokenTail=true: line 4's prev_hash is forged — a break one line PAST the
//	               last eligible seal, which no seal may ever excuse.
//	sealed=false : a clean, fully strict chain with no seal anywhere.
func writeFixture(t *testing.T, sealSeq int, sealed, brokenTail bool) fixture {
	t.Helper()
	dir := t.TempDir()
	var lines [][]byte
	l0 := entryLine(map[string]any{"entry_seq": 0, "prev_hash": zeroSeed})
	lines = append(lines, l0)
	l1 := entryLine(map[string]any{"entry_seq": 1, "prev_hash": sha256Hex(l0)})
	lines = append(lines, l1)

	fx := fixture{dir: dir, hasSeal: sealed}
	if !sealed {
		l2 := entryLine(map[string]any{"entry_seq": 2, "prev_hash": sha256Hex(l1)})
		lines = append(lines, l2)
		fx.tailSeq = 2
	} else {
		// The adjudicated historical damage: a forged prev_hash the operator
		// has accepted and preserved rather than rewritten.
		l2 := entryLine(map[string]any{"entry_seq": 2, "prev_hash": strings.Repeat("de", 32)})
		lines = append(lines, l2)
		seal := entryLine(map[string]any{
			"entry_seq": sealSeq,
			"role":      "operator",
			"kind":      fmt.Sprintf("reset-seal-cycle-%d", sealSeq),
			"prev_hash": sha256Hex(l2),
		})
		lines = append(lines, seal)
		fx.sealSeq = sealSeq
		fx.sealSHA = sha256Hex(seal)
		tailPrev := sha256Hex(seal)
		if brokenTail {
			tailPrev = strings.Repeat("ca", 32)
		}
		lines = append(lines, entryLine(map[string]any{"entry_seq": sealSeq + 1, "prev_hash": tailPrev}))
		fx.tailSeq = sealSeq + 1
	}

	var raw bytes.Buffer
	for _, l := range lines {
		raw.Write(l)
		raw.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), raw.Bytes(), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
	last := lines[len(lines)-1]
	tip := fmt.Sprintf("%d:%s", fx.tailSeq, sha256Hex(last))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write ledger.tip: %v", err)
	}
	fx.numLines = len(lines)
	return fx
}

// ---------------------------------------------------------------------------
// Identity matching: what "states the sealed prefix informationally" means,
// checked WITHOUT dictating prose.
//
// The output must name the anchor the walk actually started from, by either of
// the two stable identities the ledger gives that line: its entry_seq (what
// `evolve ledger anchor` reports) or its line SHA (what the anchor BINDS to).
// Either is accepted; a 12-hex prefix is enough for the SHA. Both are derived
// from the ledger's bytes, so no literal in the source can satisfy them for two
// different fixtures — which is what 002 proves.
// ---------------------------------------------------------------------------

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
// AC1 — a sealed prefix verifies, and says so.
// ---------------------------------------------------------------------------

// TestC1677_001_SealedPrefixIsStatedOnSuccessfulVerify drives the production
// CLI over a ledger whose damage is covered by an eligible operator seal. The
// exit code is already correct today (the resolver landed in cycle-1191); what
// must change is that success no longer hides WHICH scope it verified.
func TestC1677_001_SealedPrefixIsStatedOnSuccessfulVerify(t *testing.T) {
	fx := writeFixture(t, seqA, true, false)
	out, code := runVerify(t, fx.dir)
	if code != 0 {
		t.Fatalf("RED: sealed-prefix ledger must verify; exit=%d output=%q", code, out)
	}
	if !namesAnchor(out, fx.sealSeq, fx.sealSHA) {
		t.Errorf("RED: successful verify does not state the sealed prefix — output must name the epoch anchor "+
			"(entry_seq=%d or line sha %s…); got %q", fx.sealSeq, fx.sealSHA[:12], out)
	}
}

// TestC1677_002_ProvenanceIsDerivedFromTheLedgerNotHardcoded is the
// anti-hardcode axis. Two fixtures differ in NOTHING but the identity of the
// seal, and a third has no seal at all. A literal string in the success path
// passes at most one of the three arms.
func TestC1677_002_ProvenanceIsDerivedFromTheLedgerNotHardcoded(t *testing.T) {
	a := writeFixture(t, seqA, true, false)
	b := writeFixture(t, seqB, true, false)
	strict := writeFixture(t, 0, false, false)

	outA, codeA := runVerify(t, a.dir)
	outB, codeB := runVerify(t, b.dir)
	outStrict, codeStrict := runVerify(t, strict.dir)
	if codeA != 0 || codeB != 0 || codeStrict != 0 {
		t.Fatalf("RED: all three fixtures must verify; exits A=%d B=%d strict=%d", codeA, codeB, codeStrict)
	}

	if !namesAnchor(outA, a.sealSeq, a.sealSHA) {
		t.Errorf("RED: fixture A does not name its own anchor (entry_seq=%d / %s…): %q", a.sealSeq, a.sealSHA[:12], outA)
	}
	if !namesAnchor(outB, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: fixture B does not name its own anchor (entry_seq=%d / %s…): %q", b.sealSeq, b.sealSHA[:12], outB)
	}
	// Cross-contamination: naming the OTHER fixture's anchor means the value
	// is a literal, not a reading of this ledger.
	if namesAnchor(outA, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: fixture A names fixture B's anchor — provenance is hardcoded, not derived: %q", outA)
	}
	if namesAnchor(outB, a.sealSeq, a.sealSHA) {
		t.Errorf("RED: fixture B names fixture A's anchor — provenance is hardcoded, not derived: %q", outB)
	}
	// A chain with no anchor has no sealed prefix to report, and must not
	// borrow either fixture's identity.
	if namesAnchor(outStrict, a.sealSeq, a.sealSHA) || namesAnchor(outStrict, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: a fully strict chain reports a sealed prefix it does not have: %q", outStrict)
	}
}

// ---------------------------------------------------------------------------
// AC2 — the seal covers the prefix, never the tail. THE anti-no-op predicate.
// ---------------------------------------------------------------------------

// TestC1677_003_BreakAfterTheLastEligibleSealStillFails forges one line past
// the anchor. An implementation that greens 001/005 by always announcing a
// sealed prefix and returning 0 dies here.
func TestC1677_003_BreakAfterTheLastEligibleSealStillFails(t *testing.T) {
	fx := writeFixture(t, seqA, true, true)
	out, code := runVerify(t, fx.dir)
	if code == 0 {
		t.Fatalf("RED: a break AFTER the last eligible seal must NOT verify; exit=0 output=%q", out)
	}
	if code != 2 {
		t.Errorf("RED: chain-broken must exit 2 (the documented verify contract); got %d, output=%q", code, out)
	}
	if !strings.Contains(out, "BROKEN") || !strings.Contains(out, "chain broken") {
		t.Errorf("RED: a post-seal break must be reported as a broken chain; got %q", out)
	}
	if strings.Contains(out, "OK:") {
		t.Errorf("RED: broken verification must not also emit a success line; got %q", out)
	}
}

// ---------------------------------------------------------------------------
// AC3 — the REAL ledger: verifies, states its scope, and is never written to.
// ---------------------------------------------------------------------------

// ledgerFiles are the inputs a non-deep verify reads: the chain, its tip, and
// the out-of-band epoch anchor. Sealed segments are --deep-only and are
// deliberately not part of this snapshot.
var ledgerFiles = []string{"ledger.jsonl", "ledger.tip", "ledger-anchor.json"}

// liveEvolveDir resolves the AUTHORITATIVE .evolve directory. In a fleet
// worktree, .evolve/ledger.jsonl is a symlink into the operator's runtime
// tree (and ledger.tip / ledger-anchor.json exist only there), so the link is
// followed and its parent is the real directory. In the main tree the path
// resolves to itself.
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

// effectiveAnchor mirrors ledger.effectiveAnchorSHA over the snapshot: the LAST
// of (the sidecar anchor line, any self-chaining operator reset-seal at or
// after it). Deriving the expectation from the same bytes the CLI reads is what
// makes this predicate ungameable — the answer is not written down anywhere.
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

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	for _, l := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

// TestC1677_004_RealLedgerVerifiesNamesItsScopeAndIsNotMutated runs the
// production CLI over the operator's real 140k-line ledger (snapshotted, so a
// concurrent fleet append cannot make it flaky) and asserts three things: it
// verifies, it states the scope it verified — the anchor identity derived
// independently from the same bytes — and it wrote nothing.
func TestC1677_004_RealLedgerVerifiesNamesItsScopeAndIsNotMutated(t *testing.T) {
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
		t.Fatalf("RED: the real ledger must verify; exit=%d output=%q", code, out)
	}
	if haveAnchor {
		if !namesAnchor(out, wantSeq, wantSHA) {
			t.Errorf("RED: the real ledger is verified from an epoch anchor (entry_seq=%d / %s…) but the "+
				"successful output does not state that scope; got %q", wantSeq, wantSHA[:12], out)
		}
	} else {
		t.Logf("live ledger currently has no epoch anchor — full-strict verification; scope assertion is the strict arm of 002")
	}

	// Verify is a READER: the snapshot it was pointed at must be byte-identical
	// after the run (no rewrite, no re-hash, no tip fixup).
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
// AC-H2 (house rule 2) — every path the seam claims.
// ---------------------------------------------------------------------------

// TestC1677_005_DeepPathReportsTheSameProvenance pins the OTHER production path
// through the same command. `--deep` runs VerifyDeep, which resolves the epoch
// anchor with the same helper; if only the default path gained provenance, an
// operator's two verification commands would disagree about what was verified —
// the wired-into-one-path-only defect (#373).
func TestC1677_005_DeepPathReportsTheSameProvenance(t *testing.T) {
	fx := writeFixture(t, seqA, true, false)
	out, code := runVerify(t, fx.dir, "--deep")
	if code != 0 {
		t.Fatalf("RED: --deep must verify the sealed-prefix ledger; exit=%d output=%q", code, out)
	}
	if !namesAnchor(out, fx.sealSeq, fx.sealSHA) {
		t.Errorf("RED: --deep success does not state the sealed prefix (entry_seq=%d / %s…); got %q",
			fx.sealSeq, fx.sealSHA[:12], out)
	}
}
