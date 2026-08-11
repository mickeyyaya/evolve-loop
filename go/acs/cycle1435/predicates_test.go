//go:build acs

// Package cycle1435 encodes the cycle-1435 ACS predicates for the two tasks
// triage committed from inbox item `ledger-fleet-concurrency-chain`:
//
//   - console-ledger-rebaseline-live — the root-cause code (flock-serialized
//     append, anchor-ambiguity rejection, `evolve ledger rebaseline`) already
//     shipped, but the LIVE console-plane ledger was never repaired:
//     `evolve ledger verify --deep` against the project-root `.evolve/` exits 2
//     with `BROKEN: line 114368 prev_hash mismatch`. The deliverable is an
//     operator action (run rebaseline against the real file) plus its evidence
//     artifact, not a source patch.
//   - ledger-tip-witness-doc — `knowledge/architecture/state-and-ledger.md`
//     documents `Append`'s tip rewrite but never states that `Verify`'s `want`
//     tip is `walkChain`'s re-derived `lastSha` from `effectiveAnchorSHA`
//     forward, NOT a raw `ledger.tip` sidecar read.
//
// Predicate shape notes:
//   - 001 drives the REAL compiled `evolve` CLI (go/cmd/evolve) against the real
//     project-root state directory — the wiring proof: a `Rebaseline` that was
//     never invoked against the live file leaves it RED.
//   - 002 is the anti-no-op guard for 001. 001 could be "greened" by neutering
//     verify itself, so 002 plants a known break in a synthetic ledger and
//     requires verify --deep to still exit non-zero and still say BROKEN.
//   - 004 pins the byte-for-byte sha256 of the ledger's first 114400 lines,
//     measured at RED time (35510006 bytes). Rebaseline is append-only by
//     construction; a rewrite/truncate "repair" must fail this.
//   - 005 is the only content-shaped predicate (a doc criterion) and carries an
//     explicit `acs-predicate: config-check` waiver plus a git-tracking check.
//   - Flaky-shape rules observed: no `./...` sweep, no whole-package `go test`
//     inside a predicate, no wall-clock bound, no literal PID, no bare `git`
//     (every git call is `git -C`), no load generator. One `go build
//     ./cmd/evolve`, done once for the whole package.
//
// `.evolve/ledger.jsonl` and `.evolve/ledger-rebaseline.json` are BOTH gitignored
// (`.gitignore:35 .evolve/*`) — they are runtime state, so the cycle-93
// "assert git-tracked too" rule deliberately applies only to the doc target
// (005), never to the ledger artifacts.
package cycle1435

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

// rebaselineKind is ledger.RebaselineKind (resetSealKindPrefix + "rebaseline",
// go/internal/adapters/ledger/rebaseline.go:37). Literal for leaf-ness.
const rebaselineKind = "reset-seal-rebaseline"

// operatorRole is the Role stamped on the seal (anchor.go:66).
const operatorRole = "operator"

// Prefix pin for 004, measured live at RED time on 2026-08-11:
//
//	head -n 114400 .evolve/ledger.jsonl | shasum -a 256
//	  -> b7088c02441445f50ce8cf8d48dfff7d74663429e450dabc401f566c1a291c43
//	head -n 114400 .evolve/ledger.jsonl | wc -c  -> 35510006
//
// The break verify reports (line 114368) lies inside this prefix, so the pin
// covers the damaged region rebaseline must PRESERVE rather than rewrite.
const (
	prefixLines = 114400
	prefixBytes = 35510006
	prefixSHA   = "b7088c02441445f50ce8cf8d48dfff7d74663429e450dabc401f566c1a291c43"
)

// inboxID and cycleTag must both appear in the operator note so the bulk trust
// decision is attributable to this item and this cycle.
const (
	inboxID  = "ledger-fleet-concurrency-chain"
	cycleTag = "1435"
)

var (
	buildOnce sync.Once
	binPath   string
	buildOut  string
	buildCode int
)

// stateRoot resolves the STATE root: the ACS suite exports EVOLVE_PROJECT_ROOT
// pointing at MAIN even when predicates run from a worktree (issue #12), because
// `.evolve/` runtime data lives on main. Falls back to the repo root.
func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// evolveBin compiles the real `evolve` binary once per test binary and returns
// its path. Driving the compiled CLI — not the ledger package — is what makes
// 001/002/006 wiring proofs rather than library unit tests.
func evolveBin(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "acs1435-bin-")
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

// runEvolve invokes the compiled binary with an explicit working directory: a
// bare invocation resolves relative paths from process cwd, which differs
// between the main tree, the worktree, and each fleet lane.
func runEvolve(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(evolveBin(t), args...)
	cmd.Dir = dir
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

// writeSyntheticLedger materializes a small, correctly chained ledger.jsonl +
// ledger.tip in a fresh temp dir. When breakAt >= 0 the prev_hash of that line
// index is corrupted, planting exactly the failure shape the live file has.
func writeSyntheticLedger(t *testing.T, n, breakAt int) string {
	t.Helper()
	dir := t.TempDir()
	var buf []byte
	prev := zeroSeed
	last := ""
	for i := 0; i < n; i++ {
		p := prev
		if i == breakAt {
			p = strings.Repeat("f", 64)
		}
		rec := map[string]any{
			"ts":        "2026-08-11T00:00:00Z",
			"cycle":     1435,
			"role":      "acs-fixture",
			"kind":      "fixture",
			"exit_code": 0,
			"entry_seq": i,
			"prev_hash": p,
			"message":   "line-" + itoa(i),
		}
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshal fixture line %d: %v", i, err)
		}
		sha := lineSHA(b)
		buf = append(append(buf, b...), '\n')
		prev, last = sha, sha
	}
	if err := os.MkdirAll(filepath.Join(dir, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir fixture .evolve: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".evolve", "ledger.jsonl"), buf, 0o644); err != nil {
		t.Fatalf("write fixture ledger: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".evolve", "ledger.tip"), []byte(itoa(n-1)+":"+last), 0o644); err != nil {
		t.Fatalf("write fixture tip: %v", err)
	}
	return dir
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

// TestC1435_001_ConsoleLedgerDeepVerifyGreen is the load-bearing predicate for
// task console-ledger-rebaseline-live: the LIVE console-plane ledger must
// deep-verify. RED at authoring time (exit 2, "line 114368 prev_hash mismatch").
func TestC1435_001_ConsoleLedgerDeepVerifyGreen(t *testing.T) {
	root := stateRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	if !acsassert.FileExists(t, filepath.Join(evolveDir, "ledger.jsonl")) {
		t.Fatalf("RED: %s/ledger.jsonl missing — cannot assert the live chain is repaired", evolveDir)
	}
	out, code := runEvolve(t, root, "ledger", "verify", "--deep", "--evolve-dir", evolveDir)
	if code != 0 {
		t.Errorf("RED: `evolve ledger verify --deep` on the live console ledger exited %d, want 0.\n%s", code, out)
		return
	}
	// Guard against a vacuous green: a verify that printed nothing recognisable
	// is not proof the deep path ran.
	if !strings.Contains(out, "OK:") {
		t.Errorf("RED: verify exited 0 but did not report OK — suspicious output:\n%s", out)
	}
}

// TestC1435_002_DeepVerifyStillDetectsPlantedBreak is the anti-no-op guard for
// 001: 001 could be greened by weakening verify itself, so this plants a known
// prev_hash break in a synthetic ledger and requires the same command to reject
// it. Expected pre-existing GREEN — its job is to STAY green.
func TestC1435_002_DeepVerifyStillDetectsPlantedBreak(t *testing.T) {
	dir := writeSyntheticLedger(t, 6, 3)
	out, code := runEvolve(t, dir, "ledger", "verify", "--deep", "--evolve-dir", filepath.Join(dir, ".evolve"))
	if code == 0 {
		t.Errorf("RED: verify --deep exited 0 on a ledger with a planted prev_hash break at line 4 — the detector is neutered, so 001's green would prove nothing.\n%s", out)
	}
	if !strings.Contains(out, "BROKEN") {
		t.Errorf("RED: verify --deep did not report BROKEN for a planted break; output:\n%s", out)
	}
}

// TestC1435_003_RebaselineSealBoundToEvidenceArtifact requires that the repair
// was performed by the real, attributable operator action: an in-band
// `reset-seal-rebaseline` entry authored by role=operator sits in the live
// ledger, its note names this inbox item and cycle, and the evidence artifact
// `.evolve/ledger-rebaseline.json` records the SAME note. Binding the two means
// the artifact cannot be hand-written independently of the ledger action.
func TestC1435_003_RebaselineSealBoundToEvidenceArtifact(t *testing.T) {
	root := stateRoot(t)
	ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("RED: read %s: %v", ledgerPath, err)
	}
	sealNote := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, rebaselineKind) {
			continue
		}
		var e struct {
			Role    string `json:"role"`
			Kind    string `json:"kind"`
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.Kind == rebaselineKind && e.Role == operatorRole {
			sealNote = e.Message // last match wins: the most recent seal
		}
	}
	if sealNote == "" {
		t.Errorf("RED: no operator-role %q entry in %s — the live rebaseline was never run (or ran without an attributable note)", rebaselineKind, ledgerPath)
		return
	}
	if !strings.Contains(sealNote, inboxID) || !strings.Contains(sealNote, cycleTag) {
		t.Errorf("RED: rebaseline note %q does not cite both the inbox id %q and cycle %s — an unattributable bulk trust decision", sealNote, inboxID, cycleTag)
	}
	evidence := filepath.Join(root, ".evolve", "ledger-rebaseline.json")
	eraw, err := os.ReadFile(evidence)
	if err != nil {
		t.Errorf("RED: evidence artifact %s missing: %v", evidence, err)
		return
	}
	var ev struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal(eraw, &ev); err != nil {
		t.Errorf("RED: %s is not valid JSON: %v", evidence, err)
		return
	}
	if ev.Note != sealNote {
		t.Errorf("RED: evidence note %q != ledger seal note %q — the artifact is not bound to the actual ledger action", ev.Note, sealNote)
	}
}

// TestC1435_004_LedgerPrefixPreservedByteForByte proves the repair was
// append-only: the first 114400 lines (which contain the damaged region at line
// 114368) must be byte-identical to the RED-time measurement. A destructive
// rebuild, truncation, or hand-edit of the damaged line fails here even though
// it would green 001. Expected pre-existing GREEN — its job is to STAY green.
func TestC1435_004_LedgerPrefixPreservedByteForByte(t *testing.T) {
	root := stateRoot(t)
	ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("RED: read %s: %v", ledgerPath, err)
	}
	var prefix []byte
	seen := 0
	for i, b := range raw {
		if b == '\n' {
			seen++
			if seen == prefixLines {
				prefix = raw[:i+1]
				break
			}
		}
	}
	if prefix == nil {
		t.Fatalf("RED: %s has fewer than %d lines (%d) — the chain was TRUNCATED, not append-only sealed", ledgerPath, prefixLines, seen)
	}
	if len(prefix) != prefixBytes {
		t.Errorf("RED: first %d lines are %d bytes, want %d — the preserved prefix was rewritten", prefixLines, len(prefix), prefixBytes)
	}
	if got := lineSHA(prefix); got != prefixSHA {
		t.Errorf("RED: first %d lines sha256=%s, want %s — rebaseline must PRESERVE the damaged prefix byte-for-byte (append-only)", prefixLines, got, prefixSHA)
	}
}

// TestC1435_005_TipWitnessDocumented covers task ledger-tip-witness-doc.
//
// acs-predicate: config-check — the criterion IS documentation content
// ("the architecture doc states the tip-witness semantics"), so there is no
// system to invoke; the waiver is declared per the cycle-85 rule rather than
// dressing a grep up as behavior. It is strengthened two ways: the doc must
// name the whole re-derivation chain (not one magic token), and it must be
// git-tracked (cycle-93: a gitignored doc silently vanishes at ship).
func TestC1435_005_TipWitnessDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t) // SOURCE root: the doc is a worktree artifact
	rel := filepath.Join("knowledge", "architecture", "state-and-ledger.md")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing", rel)
	}
	for _, tok := range []string{"effectiveAnchorSHA", "walkChain", "ledger.tip", "ledger.go:"} {
		if !acsassert.FileContains(t, path, tok) {
			t.Errorf("RED: %s does not mention %q — the tip-witness re-derivation is still undocumented", rel, tok)
		}
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s is untracked — a gitignored doc is dropped at ship", rel)
	}
}

// TestC1435_006_RebaselineRefusesUnattributedNote is the negative/edge axis for
// the operator gate the live repair relies on: an empty note must be refused,
// writing nothing, so the seal in 003 can never be an anonymous trust decision.
// Expected pre-existing GREEN — its job is to STAY green.
func TestC1435_006_RebaselineRefusesUnattributedNote(t *testing.T) {
	dir := writeSyntheticLedger(t, 4, -1)
	evolveDir := filepath.Join(dir, ".evolve")
	before, err := os.ReadFile(filepath.Join(evolveDir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read fixture ledger: %v", err)
	}
	out, code := runEvolve(t, dir, "ledger", "rebaseline", "--evolve-dir", evolveDir)
	if code == 0 {
		t.Errorf("RED: `ledger rebaseline` with no --note exited 0 — the operator gate is missing.\n%s", out)
	}
	// Refusing must also mean writing nothing: a partial append would leave an
	// unattributed seal behind.
	after, err := os.ReadFile(filepath.Join(evolveDir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("re-read fixture ledger: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("RED: refused rebaseline still mutated the ledger (%d -> %d bytes)", len(before), len(after))
	}
}
