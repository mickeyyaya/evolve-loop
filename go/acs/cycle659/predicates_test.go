//go:build acs

// Package cycle659 materialises the cycle-659 acceptance criteria for the single
// triage-committed (`## top_n`) task: statefile-rmw-flock-single-source.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  statefile-rmw-flock-single-source (H) — C659_001..005
//	Every `## deferred` item (difficulty-conditioned-phase-budgets,
//	token-telemetry-s6/s7/s8, tokenopt-*, boot-orphan-sweep, …) gets ZERO
//	predicates here.
//
// FEATURE CONTEXT
//
//	state.json's full-fidelity read-modify-write is implemented THREE times:
//	  (A) internal/phases/ship/statefile.go  readStateMap/writeStateMap  (flocked)
//	  (B) internal/core/reset.go             readJSONMapFile/writeJSONMapFileAtomic
//	  (C) internal/phaseintegrity/repin.go   inline unmarshal→mutate→rename
//	This cycle consolidates all three into ONE leaf package
//	internal/adapters/statemap (ReadStateMap + UpdateStateMap, importing only
//	internal/adapters/flock + stdlib — the verified-acyclic home from cycle-644's
//	retrospective) and deletes the duplicates, so exactly one lock-owning RMW
//	implementation exists. statemap is graduated into go/.apicover-enforce as a
//	new-package obligation (3rd-recurrence class: 640 stateutil, 644 import
//	cycle, 653 tree hygiene).
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it runs `go test -race`/`go vet`/`go build` against the real packages and
// asserts on the actual exit code, never a bare "source file contains text X"
// grep. The single-source pin (C659_002) additionally greps for the ABSENCE of
// the deleted duplicates (unforgeable — a magic string cannot re-satisfy it),
// but its load-bearing assertion is that the consolidated package builds.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : C659_001 statemap package tests pass under -race (primitive
//     works: round-trip + serialization).
//   - Negative  : C659_002 the duplicate RMW func definitions are GONE from
//     reset.go (an implementation that leaves them in place FAILS).
//   - Semantic  : C659_003 seal + repin + ship callers route through statemap.
//   - Hygiene   : C659_004 touched packages vet clean under the consolidation.
//   - Config    : C659_005 statemap graduated into .apicover-enforce.
package cycle659

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the go module directory for `go -C <dir>` subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

// TestC659_001_StatemapPackageTestsPassUnderRace is the behavioral core (AC1
// primitive serialization + AC4 full-fidelity round-trip + AC5 -race green). It
// runs the consolidated leaf package's own test suite under the race detector.
// RED today: internal/adapters/statemap does not exist, so `go test` fails to
// build (non-zero). GREEN once Builder implements ReadStateMap/UpdateStateMap
// with flock.PathLock across the whole RMW and the primitive tests pass.
func TestC659_001_StatemapPackageTestsPassUnderRace(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-race", "-count=1",
		"./internal/adapters/statemap/...",
	)
	if code != 0 || err != nil {
		t.Fatalf("RED: `go test -race ./internal/adapters/statemap/...` failed (exit=%d): %v\n"+
			"Builder must create the leaf package internal/adapters/statemap with\n"+
			"  func ReadStateMap(path string) (map[string]any, error)\n"+
			"  func UpdateStateMap(path string, mutate func(map[string]any)) error\n"+
			"holding flock.PathLock(path) across the whole read-modify-write.\nOutput:\n%s",
			code, err, out+"\n"+errOut)
	}
}

// TestC659_002_ResetDuplicateRMWRemoved is the single-source pin's negative axis
// (AC2): the reset.go copy of the RMW helpers (readJSONMapFile /
// writeJSONMapFileAtomic) MUST be deleted, and reset.go MUST route through
// statemap. Its load-bearing behavioral anchor is that the statemap package
// BUILDS (so this is not a bare source grep — a magic string cannot satisfy the
// build); the grep pins the ABSENCE of the deleted duplicate, which no magic
// string can re-create.
func TestC659_002_ResetDuplicateRMWRemoved(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir, "./internal/adapters/statemap/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: consolidated package does not build (exit=%d): %v\n%s", code, err, errOut)
	}

	resetGo := repoFile(t, "go/internal/core/reset.go")
	for _, dup := range []string{"func readJSONMapFile(", "func writeJSONMapFileAtomic("} {
		// ABSENCE check: use the non-failing FileContainsAny probe. FileContains(t,…)
		// t.Errorf's when the substring is missing, so it cannot express "must be
		// absent" (it would fail in the GREEN state) — corrected in build (cycle-659).
		if acsassert.FileContainsAny(resetGo, dup) {
			t.Errorf("RED: reset.go still defines the duplicate RMW helper %q — it must be deleted "+
				"and SealCycle routed through statemap.UpdateStateMap(", dup)
		}
	}
	if !acsassert.FileContains(t, resetGo, "statemap.") {
		t.Errorf("RED: reset.go does not reference statemap.* — SealCycle must route its state.json " +
			"read-modify-write through statemap.UpdateStateMap(, not an inline copy")
	}
}

// TestC659_003_AllWritersRouteThroughStatemap is the semantic axis (AC2): the
// three former duplicate sites — ship (statefile.go), repin (repin.go), and seal
// (reset.go) — must all reference the single statemap source, and the grep
// target MUST be `statemap.` (never `storage.` — routing core through storage
// closes a forbidden core→storage import cycle, the cycle-644 failure). The
// load-bearing anchor is that the three caller packages BUILD together with the
// new dependency; the grep pins that each names statemap.
func TestC659_003_AllWritersRouteThroughStatemap(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir,
		"./internal/core/...", "./internal/phaseintegrity/...", "./internal/phases/ship/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: caller packages do not build with the statemap dependency (exit=%d): %v\n%s",
			code, err, errOut)
	}

	callers := map[string]string{
		"go/internal/core/reset.go":            "SealCycle",
		"go/internal/phaseintegrity/repin.go":  "RepinShipSHA",
		"go/internal/phases/ship/statefile.go": "ship",
	}
	for rel, who := range callers {
		f := repoFile(t, rel)
		if !acsassert.FileContains(t, f, "statemap.") {
			t.Errorf("RED: %s (%s path) does not route through statemap.* — the single RMW source", rel, who)
		}
		// Guard the exact cycle-644 regression: core must NOT be re-pointed at
		// storage.UpdateStateMap( (import cycle). Only reset.go is core; the grep
		// is a targeted anti-pattern check on that file.
		// ABSENCE check: non-failing FileContainsAny (see C659_002 note).
		if rel == "go/internal/core/reset.go" && acsassert.FileContainsAny(f, "storage.UpdateStateMap(") {
			t.Errorf("RED: reset.go references storage.UpdateStateMap( — forbidden core→storage import " +
				"cycle (cycle-644). The pin target MUST be statemap.UpdateStateMap(")
		}
	}
}

// TestC659_004_TouchedPackagesVetClean is the hygiene axis (AC5): the four
// packages the consolidation edits — statemap, core, phaseintegrity, ship — must
// vet clean together. RED today (statemap missing → vet fails). Behavioral: runs
// the real `go vet`.
func TestC659_004_TouchedPackagesVetClean(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "vet", "-C", dir,
		"./internal/adapters/statemap/...", "./internal/core/...",
		"./internal/phaseintegrity/...", "./internal/phases/ship/...",
	)
	if code != 0 || err != nil {
		t.Fatalf("RED: `go vet` on the consolidated packages failed (exit=%d): %v\n%s",
			code, err, out+"\n"+errOut)
	}
}

// TestC659_005_StatemapGraduatedIntoApicoverEnforce is the config axis: the
// new leaf package is added to go/.apicover-enforce as a covered new-package
// obligation (the 3rd-recurrence-class hardening the item asks for).
//
// acs-predicate: config-check
//
// RED today: .apicover-enforce does not list internal/adapters/statemap.
func TestC659_005_StatemapGraduatedIntoApicoverEnforce(t *testing.T) {
	enforce := repoFile(t, "go/.apicover-enforce")
	if !acsassert.FileContainsAny(enforce,
		"./internal/adapters/statemap",
		"internal/adapters/statemap",
	) {
		t.Errorf("RED: go/.apicover-enforce does not list internal/adapters/statemap — the new leaf " +
			"package must be graduated into the enforced set (new-package obligation, 3rd-recurrence class)")
	}
	// Sanity: the enforce file is non-empty and still lists a known package, so a
	// truncation can't vacuously pass the absence check above.
	if !acsassert.FileContains(t, enforce, "./internal/adapters/flock") {
		t.Errorf("apicover-enforce sanity: expected the file to still list ./internal/adapters/flock")
	}
}
