package phasecoherence

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gitignore_birth_test.go — tracked-corpus birth coherence (2026-08-05,
// cycle-1345/1346/1348 identical-fingerprint batch HALT).
//
// The .gitignore ladder ignores .evolve/* and re-includes the surfaces that
// SHIP (profiles, phases, inbox, plugin, …). The failure class this test
// pins: a directory holds a TRACKED corpus (so the repo's operative design
// says "these files ship") while the ladder has no re-include for it — every
// NEW file there is born ignored. `git check-ignore` cannot flag the
// pathspec (tracked content exempts the dir), so ship staging keeps it, and
// `git add` refuses the whole pathspec (rc=1 "ignored by one of your
// .gitignore files") — a deterministic ship-killer for exactly the lanes
// whose deliverable is a new file in that corpus. .evolve/evals (54 tracked
// eval definitions, corpus since 2026-06-01, ladder line missing) halted the
// 2026-08-05 batch this way.
//
// Contract: every (directory, extension) pair present in the TRACKED corpus
// under .evolve/ must admit a NEW birth in at least ONE of the two shapes the
// ladder's own carve-outs use:
//
//   - a new NAME beside the exemplar (extension corpora: .evolve/evals/*.md), or
//   - the exemplar's exact NAME in a new sibling directory (exact-name
//     corpora: .evolve/phases/<new>/phase.json).
//
// A pair that admits neither is a ship-killer unless it is in the reasoned
// designed-ignored allowlist below (surfaces where tracked entries are
// grandfathered legacy and new births are runtime state BY DESIGN).
func TestTrackedCorpusDirsAllowNewBirths(t *testing.T) {
	t.Parallel()
	root := repoRootForPairing(t)

	// designedIgnoredPrefixes are corpus dirs whose NEW files are runtime
	// state by documented design — tracked entries there are grandfathered,
	// never a template for future births. Every entry needs the reason.
	designedIgnoredPrefixes := map[string]string{
		// Ladder comment: "processed/, processing/ and rejected/ are runtime
		// subdirs (gitignored)" — the tracked interactive-2026-06-* items
		// predate that rule.
		".evolve/inbox/processed/": "runtime archive; tracked items are pre-rule legacy",
		// Ladder comment: "Generated lesson YAMLs are per-project runtime
		// state and remain ignored, but the .keep marker ships".
		".evolve/instincts/lessons/": "only the .keep marker ships by design",
		// One-off manual park (e3509917, R100 rename out of inbox/); zero
		// active Go writers — `grep -rn inbox-parked go/` is empty as of
		// 2026-08-05. "Parked" means held-not-yet-decided: new content there
		// must NOT auto-ship, so allowlist beats carve-out (adversarial
		// review finding 1 on the cycle-1348 fix).
		".evolve/inbox-parked/": "grandfathered manual park; no active writer; parked content must not auto-ship",
	}
	// The .evolve ROOT is governed by an explicit per-file whitelist
	// (policy.json, naming.json, commit-prefix-scope.json) — a new arbitrary
	// root file must stay ignored, so the root is exempt from the birth rule.
	rootExempt := ".evolve"

	out, err := exec.Command("git", "-C", root, "ls-files", "--", ".evolve/").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	type pair struct{ dir, ext string }
	seen := map[pair]string{} // pair -> one tracked exemplar (for the failure message)
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(f))
		if dir == rootExempt {
			continue
		}
		exempt := false
		for prefix := range designedIgnoredPrefixes {
			if strings.HasPrefix(f, prefix) {
				exempt = true
				break
			}
		}
		if exempt {
			continue
		}
		ext := filepath.Ext(f)
		if _, ok := seen[pair{dir, ext}]; !ok {
			seen[pair{dir, ext}] = f
		}
	}
	if len(seen) == 0 {
		t.Fatal("no tracked corpus found under .evolve/ — the probe itself is broken")
	}

	pairs := make([]pair, 0, len(seen))
	for p := range seen {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].dir+pairs[i].ext < pairs[j].dir+pairs[j].ext
	})
	// ONE batched check-ignore over every probe (the dropIgnoredPaths
	// pattern, ship/gitops.go): rc=0 lists the ignored subset one-per-line,
	// rc=1 means none ignored. This test previously spawned one subprocess
	// per probe (218 execs, ~+1.5s on the ship-time repo-contract pack's
	// critical path — adversarial review finding 2); the batch is one exec.
	// Probe names are ASCII-lowercase by construction, so no C-quoted output
	// lines can occur. Methodology note (review finding 4): this asserts
	// SOME rule admits the probe, not that the corpus's OWN carve-out does —
	// an accidentally-broad future negation (e.g. `!.evolve/*/*.md`) would
	// satisfy it for the wrong reason; the ladder currently contains no such
	// rule.
	type probes struct{ sameDir, siblingDir string }
	probeFor := make(map[pair]probes, len(pairs))
	var all []string
	for _, p := range pairs {
		pr := probes{
			sameDir:    p.dir + "/__birth-probe__" + p.ext,
			siblingDir: filepath.ToSlash(filepath.Join(filepath.Dir(p.dir), "__birth-probe__", filepath.Base(seen[p]))),
		}
		probeFor[p] = pr
		all = append(all, pr.sameDir, pr.siblingDir)
	}
	args := append([]string{"-C", root, "check-ignore", "--"}, all...)
	out2, runErr := exec.Command("git", args...).Output()
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
			t.Fatalf("batched check-ignore probe failed: %v", runErr)
		}
	}
	ignored := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out2)), "\n") {
		if line != "" {
			ignored[line] = true
		}
	}
	for _, p := range pairs {
		pr := probeFor[p]
		if ignored[pr.sameDir] && ignored[pr.siblingDir] {
			t.Errorf("corpus (%s, %s) admits NO new births: %q and %q are both born ignored while %q is TRACKED — "+
				"the .gitignore ladder is missing a re-include; a lane shipping a new file there dies with "+
				"GIT_STAGE_FAILED (the cycle-1348 batch-HALT class). Either add the ladder carve-out or move "+
				"the corpus to designedIgnoredPrefixes with a reason.",
				p.dir, p.ext, pr.sameDir, pr.siblingDir, seen[p])
		}
	}
}
