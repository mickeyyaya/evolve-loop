//go:build acs

// Package cycle968 materializes the cycle-968 acceptance criteria for fleet lane
// `carryforward-real-cherrypick-filter`. The lane's cycle-962 deliverables shipped
// INERT: core.CarryforwardCandidateLandable (weight 0.94) and its dependent
// core.PruneSupersededOrphans have ZERO non-test callers (grep-confirmed), which
// violates the pinned goal floor — "No inert API: every new exported surface ships
// with a caller and a naming test." This cycle WIRES the filter into the real
// production fleet-rebase recovery path and adds a guard so the inert class cannot
// silently regrow.
//
// SCOPE (Rule 3, surfaced): fleet_scope names one inbox id; triage expanded it into
// two top_n tasks in ONE worktree. Predicates are authored for BOTH (both top_n,
// built together). The DEFERRED beyond-ask idea (acs-verdict skip-reason emission)
// gets ZERO predicates (R9.3 floor-binding).
//
// ---------------------------------------------------------------------------
// DESIGN DECISION surfaced to Builder (Rule 1 + Rule 3 — do NOT silently deviate)
// ---------------------------------------------------------------------------
// The scout's Task-1 title says "wire CarryforwardCandidateLandable so a superseded
// candidate short-circuits". Taken LITERALLY (landable==false ⇒ short-circuit) that
// is INCORRECT: CarryforwardCandidateLandable returns false for BOTH an already-
// landed (superseded) candidate AND a genuine 3-way CONFLICT. Short-circuiting a
// genuine conflict as "already landed" would SILENTLY DROP real overlapping work
// that must instead route to the debugger (CodeGitFleetRebaseConflict). The
// scout's own Key Finding (report lines 32-35) is the precise, correct framing:
// it is the SUPERSESSION branch that must short-circuit, while a real conflict must
// still flow to the existing conflict route.
//
// Correct, minimal contract the Builder MUST add to package core WITHOUT modifying
// this file (natural home: go/internal/core/carryforward_filter.go):
//
//	type FleetRebaseVerdict int
//	const (
//	    FleetRebaseAlreadyLanded FleetRebaseVerdict = iota // superseded → short-circuit, NO replay/re-audit
//	    FleetRebaseClean                                    // clean & not landed → rebase & replay (existing path)
//	    FleetRebaseConflict                                 // genuine 3-way conflict → debugger route
//	)
//	func ClassifyFleetRebaseCandidate(ctx context.Context, dir, candidateRef, base string) (FleetRebaseVerdict, error)
//	    // Deterministic, zero-LLM. Internally REUSES the inert cycle-962 surface:
//	    //   landable, err := CarryforwardCandidateLandable(...)  // gives it a caller ⇒ not inert
//	    //   if err  → propagate (git-infra failure; NEVER masked as a verdict)
//	    //   if landable                 → FleetRebaseClean
//	    //   else if refSuperseded(...)  → FleetRebaseAlreadyLanded
//	    //   else                        → FleetRebaseConflict
//	    // All git through the gitCapture seam.
//
// Production wiring seam: (*Orchestrator).recoverFromShipError in ship_recovery.go,
// the CodeGitFleetRebaseNeeded branch, MUST call ClassifyFleetRebaseCandidate BEFORE
// rebaseCycleBranchOntoMain and map: AlreadyLanded → short-circuit (no wasted
// re-audit — the explicit 948 "PASS-but-unlanded duplicate" fix); Clean → the
// existing replay; Conflict → the existing debugger reclassification.
//
// core.PruneSupersededOrphans is NOT wired this cycle (branch-deleting housekeeping
// in the hot recovery path is out of a focused fleet-rebase lane's scope). Its
// caller is dispositioned as tracked carryover (manual+checklist, see test-report),
// NOT a fabricated caller — per scout Task-2's explicit allowance and
// no_workaround_root_cause_redesign. Its identity/signature is still pinned below so
// the surface cannot drift while the carryover is open.
//
// ---------------------------------------------------------------------------
// PREDICATE STYLE (cycle-85 rule): go/internal/core is importable from go/acs, so
// every BEHAVIORAL predicate EXERCISES the SUT (calls ClassifyFleetRebaseCandidate
// against a REAL git repo built in a temp dir — git is always present) and asserts
// on the returned verdict. RED here is a COMPILE failure (undefined:
// core.ClassifyFleetRebaseCandidate / core.FleetRebaseVerdict / the consts), which
// fails for the RIGHT reason: the production symbols are absent. The two structural
// WIRING-PROOF predicates (caller-exists, the "no inert API" floor) carry a
// `// acs-predicate: config-check` waiver because a caller-existence assertion is
// inherently a source-structure check; each is PAIRED with the behavioral tests
// above (never the sole load-bearing assertion for the feature).
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C968_001 (clean, non-superseded candidate ⇒ FleetRebaseClean — the
//	           anti-`always-conflict/always-landed` signal a degenerate map cannot fake).
//	NEGATIVE → C968_002 (patch-id-dup ⇒ AlreadyLanded), C968_003 (ancestor ⇒
//	           AlreadyLanded), C968_004 (genuine conflict ⇒ Conflict, NOT AlreadyLanded
//	           — the strongest anti-drop-work signal), C968_005 (bad ref ⇒ error, never
//	           masked as a verdict).
//	SEMANTIC → the three verdicts are DISTINCT outcomes, each asserted separately.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	T1-AC1 clean, not superseded         → FleetRebaseClean          → C968_001 (POSITIVE)
//	T1-AC2 patch-id-dup already landed   → FleetRebaseAlreadyLanded  → C968_002 (NEGATIVE)
//	T1-AC3 is-ancestor already landed    → FleetRebaseAlreadyLanded  → C968_003 (NEGATIVE/EDGE)
//	T1-AC4 genuine 3-way conflict        → FleetRebaseConflict       → C968_004 (NEGATIVE, critical)
//	T1-AC5 git-infra error propagates    → non-nil error             → C968_005 (EDGE/NEGATIVE)
//	T1-AC6 recoverFromShipError CALLS ClassifyFleetRebaseCandidate   → C968_006 (WIRING, config-check)
//	T2-AC1 ClassifyFleetRebaseCandidate CALLS CarryforwardCandidateLandable (kills inert) → C968_007 (WIRING)
//	T2-AC2 CarryforwardCandidateLandable identity/signature pinned    → C968_008 (compile-pin)
//	T2-AC3 PruneSupersededOrphans identity/signature pinned (carryover)→ C968_009 (compile-pin)
//	T1-AC7 / T2-AC4 -race + go vet + repo-wide apicover clean         → manual+checklist (Auditor CI-parity)
package cycle968

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// Compile-time identity/signature pins (T2-AC2 / T2-AC3). These fail to compile if
// the Builder renames or changes the shape of either cycle-962 surface, freezing the
// public contract the naming guard defends. They sit at package scope so they are
// part of the same RED compile failure until the whole package builds.
var (
	_ func(context.Context, string, string, string) (bool, error)                                     = core.CarryforwardCandidateLandable
	_ func(context.Context, string, string, func(string) (bool, error)) ([]core.OrphanVerdict, error) = core.PruneSupersededOrphans
)

// git runs `git -C dir args...` as the TEST HARNESS (not the SUT) and fails the test
// on any non-zero exit — fixtures must build cleanly for the SUT assertion to mean
// anything.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// initRepo creates a fresh git repo on branch `main` with one committed file
// base.txt (two lines) and deterministic identity/config, returning its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "acs@evolve.local")
	git(t, dir, "config", "user.name", "acs")
	git(t, dir, "config", "commit.gpgsign", "false")
	writeCommit(t, dir, "base.txt", "line1\nline2\n", "base commit")
	return dir
}

// writeCommit writes content to name under dir and commits it with msg.
func writeCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", msg)
}

// coreSrc returns the absolute path of a source file under go/internal/core in the
// active worktree (RepoRoot resolves to the worktree top via git rev-parse).
func coreSrc(t *testing.T, file string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", file)
}

// -----------------------------------------------------------------------------
// Task 1 — wire the filter into the fleet-rebase recovery path.
// -----------------------------------------------------------------------------

// C968_001 (T1-AC1, POSITIVE / anti-degenerate) — a candidate adding a NEW file,
// with main diverged only by an unrelated file, cleanly 3-way merges and is not
// superseded ⇒ FleetRebaseClean (the replay is worthwhile). A degenerate classifier
// that always returns AlreadyLanded or Conflict fails here.
func TestClassifyFleetRebaseCandidate_CleanNotSuperseded(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "new feature\n", "cand adds feature")
	git(t, dir, "checkout", "-q", "main")
	writeCommit(t, dir, "mainonly.txt", "only on main\n", "main unrelated change")

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a clean candidate: %v", err)
	}
	if got != core.FleetRebaseClean {
		t.Errorf("clean non-superseded candidate: got verdict %v, want FleetRebaseClean", got)
	}
}

// C968_002 (T1-AC2, NEGATIVE) — the candidate's change is already on main under a
// DIFFERENT sha (cherry-picked), with an extra unrelated main commit so the candidate
// tip is NOT an ancestor. Only the patch-id functional-duplicate screen catches this;
// it MUST short-circuit as AlreadyLanded, not be replayed.
func TestClassifyFleetRebaseCandidate_PatchIdDuplicateAlreadyLanded(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	candSHA := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))

	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "cherry-pick", candSHA) // same change, new sha → patch-id dup
	writeCommit(t, dir, "unrelated.txt", "x\n", "main moves on")

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a superseded candidate: %v", err)
	}
	if got != core.FleetRebaseAlreadyLanded {
		t.Errorf("patch-id-duplicate candidate: got verdict %v, want FleetRebaseAlreadyLanded (short-circuit, no wasted re-audit)", got)
	}
}

// C968_003 (T1-AC3, NEGATIVE / EDGE) — the candidate tip is a strict ancestor of main
// (already fast-forward merged). The is-ancestor arm of the supersession screen must
// classify it AlreadyLanded even though a merge of an ancestor reads as an empty no-op.
func TestClassifyFleetRebaseCandidate_AncestorAlreadyLanded(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--ff-only", "cand") // main tip == cand tip

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on an ancestor candidate: %v", err)
	}
	if got != core.FleetRebaseAlreadyLanded {
		t.Errorf("ancestor candidate: got verdict %v, want FleetRebaseAlreadyLanded", got)
	}
}

// C968_004 (T1-AC4, NEGATIVE — CRITICAL) — a candidate that edits the SAME line main
// later changed a different way produces a REAL 3-way conflict. This is the load-
// bearing correctness distinction the naive "landable==false ⇒ short-circuit" reading
// gets WRONG: the verdict MUST be FleetRebaseConflict (→ debugger), NEVER
// FleetRebaseAlreadyLanded — silently dropping genuine overlapping work is the exact
// failure this test forbids.
func TestClassifyFleetRebaseCandidate_GenuineConflictRoutesToConflict(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "base.txt", "CAND\nline2\n", "cand edits line1")
	git(t, dir, "checkout", "-q", "main")
	writeCommit(t, dir, "base.txt", "MAIN\nline2\n", "main edits line1")

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a conflicting candidate (want a Conflict verdict, not an error): %v", err)
	}
	if got == core.FleetRebaseAlreadyLanded {
		t.Fatalf("genuine conflict classified as FleetRebaseAlreadyLanded — real overlapping work would be SILENTLY DROPPED instead of routed to the debugger")
	}
	if got != core.FleetRebaseConflict {
		t.Errorf("genuine conflict: got verdict %v, want FleetRebaseConflict", got)
	}
}

// C968_005 (T1-AC5, EDGE / NEGATIVE) — a git-infrastructure failure (a candidate ref
// that does not exist) MUST surface as a non-nil error, never be masked as any verdict
// ("landed"/"clean"/"conflict"). This defends fail-loudly: an infra error must abort
// recovery, not be silently swallowed into a short-circuit.
func TestClassifyFleetRebaseCandidate_InfraErrorPropagates(t *testing.T) {
	dir := initRepo(t)

	_, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "does-not-exist-ref", "main")
	if err == nil {
		t.Errorf("nonexistent candidate ref returned nil error; a git-infra failure must propagate, never be masked as a verdict")
	}
}

// C968_006 (T1-AC6, WIRING PROOF — kills inert) — the production recovery method
// (*Orchestrator).recoverFromShipError MUST call ClassifyFleetRebaseCandidate, so the
// classifier is reachable in production and not a second inert surface. Structural
// (AST) caller-exists check, paired with the behavioral tests above.
// acs-predicate: config-check — a caller-existence ("no inert API") assertion is
// inherently a source-structure check; the behavior it gates is proven by C968_001..005.
func TestClassifyFleetRebaseCandidate_WiredIntoRecoverFromShipError(t *testing.T) {
	// acs-predicate: config-check
	src := coreSrc(t, "ship_recovery.go")
	n, err := acsassert.CountInGoFunc(src, "recoverFromShipError", "ClassifyFleetRebaseCandidate")
	if err != nil {
		t.Fatalf("CountInGoFunc(recoverFromShipError, ClassifyFleetRebaseCandidate): %v", err)
	}
	if n < 1 {
		t.Errorf("recoverFromShipError does not call ClassifyFleetRebaseCandidate (count=%d); the fleet-rebase pre-screen is not wired into production", n)
	}
}

// -----------------------------------------------------------------------------
// Task 2 — no-inert-surface naming/identity guard.
// -----------------------------------------------------------------------------

// C968_007 (T2-AC1, WIRING PROOF — kills the cycle-962 inert surface) —
// ClassifyFleetRebaseCandidate MUST call CarryforwardCandidateLandable, giving the
// weight-0.94 cycle-962 filter its first production caller (via the wired classifier).
// This is the assertion that closes the "no inert API" floor violation for the filter.
// acs-predicate: config-check — caller-existence is an inherent source-structure check;
// the filter's behavior is already pinned by cycle962's behavioral predicates.
func TestCarryforwardCandidateLandable_HasProductionCaller(t *testing.T) {
	// acs-predicate: config-check
	src := coreSrc(t, "carryforward_filter.go")
	n, err := acsassert.CountInGoFunc(src, "ClassifyFleetRebaseCandidate", "CarryforwardCandidateLandable")
	if err != nil {
		t.Fatalf("CountInGoFunc(ClassifyFleetRebaseCandidate, CarryforwardCandidateLandable): %v — is ClassifyFleetRebaseCandidate defined in carryforward_filter.go?", err)
	}
	if n < 1 {
		t.Errorf("ClassifyFleetRebaseCandidate does not call CarryforwardCandidateLandable (count=%d); the cycle-962 filter would remain inert", n)
	}
}

// C968_008 (T2-AC2) — CarryforwardCandidateLandable's exact identity/signature is
// frozen. The load-bearing enforcement is the package-scope compile-time pin above
// (a rename/shape change fails to build); this test also documents that a live
// production caller exists (asserted structurally by C968_007), so the naming guard
// and the caller guard travel together.
func TestCarryforwardCandidateLandable_IdentityPinned(t *testing.T) {
	// The compile-time var pin at package scope is the real guard; referencing the
	// symbol here keeps the intent local and self-documenting.
	var fn func(context.Context, string, string, string) (bool, error) = core.CarryforwardCandidateLandable
	if fn == nil {
		t.Fatal("core.CarryforwardCandidateLandable is nil — identity pin lost")
	}
}

// C968_009 (T2-AC3) — PruneSupersededOrphans's identity/signature is frozen while its
// production caller remains tracked carryover (see test-report disposition). The
// package-scope compile-time pin above is the real guard; this test asserts the
// symbol resolves so the surface cannot silently vanish or drift before it is wired.
func TestPruneSupersededOrphans_IdentityPinned(t *testing.T) {
	var fn func(context.Context, string, string, func(string) (bool, error)) ([]core.OrphanVerdict, error) = core.PruneSupersededOrphans
	if fn == nil {
		t.Fatal("core.PruneSupersededOrphans is nil — identity pin lost")
	}
}
