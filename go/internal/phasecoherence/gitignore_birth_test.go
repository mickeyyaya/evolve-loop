package phasecoherence

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestTrackedCorpusDirsAllowNewBirths(t *testing.T) {
	t.Parallel()
	root := repoRootForPairing(t)

	designedIgnoredPrefixes := map[string]string{
		".evolve/inbox/processed/":   "runtime archive; tracked items are pre-rule legacy",
		".evolve/instincts/lessons/": "only the .keep marker ships by design",
		".evolve/inbox-parked/":      "grandfathered manual park; no active writer; parked content must not auto-ship",
	}
	// The .evolve root ships an explicit per-file whitelist, so a new root file must stay ignored.
	rootExempt := ".evolve"

	out, err := exec.Command("git", "-C", root, "ls-files", "--", ".evolve/").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	type pair struct{ dir, ext string }
	seen := map[pair]string{} // one tracked exemplar per pair
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
	// One batched check-ignore: rc=0 lists the ignored subset, rc=1 means none is ignored.
	// Probe names are ASCII, so git prints no C-quoted lines.
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
