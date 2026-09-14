package ciparitygate

import "strings"

// envExclusiveEntry is the SINGLE record for one env-exclusive package: the
// package, the evidence, and where its integration tests actually run instead.
// Every consumer — envExclusivePkg, the emitted WARN, the mixed-scope event,
// the whole-suite filter — projects from tierEnvExclusive; no prose
// restatement elsewhere is authoritative (the 2026-09-01 architecture review
// found three stale copies inside one diff — projections, not narration).
//
// SELECTION CRITERION (the only thing keeping this list a scalpel, pinned by
// TestEnvExclusive_EntriesDeclareNoCIBackstop): a package may be listed ONLY
// when the serialized retake cannot make red-twice trustworthy AND CI
// provides no backstop. A package whose integration tier CI covers belongs IN
// the lane tier. HISTORY: internal/core, cmd/evolve and internal/phases/ship
// were excluded 2026-07-19 (8e2afef0; contention false-REDs, cycles
// 930/931/932) and the serialized retake that properly cures that contention
// landed ONE DAY LATER (3c5ed711) — the never-revisited skip shipped
// cycle-1594's red to main for 2.5 days (20e839ee; #519; #518). Two notes for
// the next adjudicator: the July evidence over-attributed cmd/evolve (its
// fleet-soak suite is in-process fakes, no real tmux — cmd_fleet_soak_test.go;
// its ~69s is CPU), and a compile-only floor would NOT have caught 1594 (an
// assertion failure, not a compile failure) — running the tier is the point.
type envExclusiveEntry struct {
	pkg string // module-relative package dir, e.g. "internal/bridge"
	why string // the exclusion evidence
	// backstop states where the package's integration tests actually run;
	// rendered VERBATIM into the emitted WARN. One dishonest word here
	// recreates the #483 defect: a gate asserting coverage that cannot occur.
	backstop string
}

// envExclusiveNoCIMarker is the machine-checkable half of the selection
// criterion: every entry's backstop must carry it, and the rule test asserts
// against THIS const — one home for the phrase, so rewording the prose cannot
// silently detach the data from the contract.
const envExclusiveNoCIMarker = "NOT covered by CI"

var tierEnvExclusive = []envExclusiveEntry{{
	pkg:      "internal/bridge",
	why:      "requireTmux tests boot real tmux sessions; under a live wave those boots time out (13 offenders on cycle-1543, all exit=80; the same tests 7/7 PASS in 17.2s on a quiet host)",
	backstop: "internal/bridge's requireTmux tier is " + envExclusiveNoCIMarker + " (no tmux on runners — the #483 finding); its backstop is a quiet-host run (loop-boot preflight, or `go test -tags integration` with no wave active)",
}}

// envExclusiveEntryFor resolves a package pattern ("./internal/bridge/...", a
// full import path, or a bare relative dir) to its record.
func envExclusiveEntryFor(p string) (envExclusiveEntry, bool) {
	p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/...")
	p = strings.TrimSuffix(p, "/")
	for _, e := range tierEnvExclusive {
		if p == e.pkg || strings.HasSuffix(p, "/"+e.pkg) {
			return e, true
		}
	}
	return envExclusiveEntry{}, false
}

// envExclusivePkg reports whether a package pattern denotes an env-exclusive
// package — a projection of the record table.
func envExclusivePkg(p string) bool {
	_, ok := envExclusiveEntryFor(p)
	return ok
}

// envExclusiveBackstopNote renders each skipped package's backstop, verbatim
// from its record, deduplicated.
func envExclusiveBackstopNote(pkgs []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, p := range pkgs {
		if e, ok := envExclusiveEntryFor(p); ok && !seen[e.pkg] {
			seen[e.pkg] = true
			parts = append(parts, e.why+" — "+e.backstop)
		}
	}
	return strings.Join(parts, "; ")
}
