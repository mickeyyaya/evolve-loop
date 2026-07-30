package dossier

// read.go — the committed-corpus reader. Write/commitPair put each cycle's
// dossier in <projectRoot>/knowledge-base/cycles/cycle-N.{json,md}; this is the
// counterpart that reads a WINDOW of them back.
//
// It exists because a fleet batch's lane cycles are separate `evolve cycle run`
// subprocesses: their CycleResults never return to the parent loop, so the
// committed dossier is the ONLY channel through which a batch-level surface can
// see what its lanes recorded. A batch summary that folds only the parent's
// in-memory results reports zero for every fleet batch — which is exactly the
// silence such a summary is built to end.

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// CyclesDirName is the committed-dossier directory, relative to the project root.
const cyclesDirName = "cycles"

// ReadCommitted reads the committed dossiers for cycles >= minCycle from
// <projectRoot>/knowledge-base/cycles/, ascending by cycle number. The cycle
// number is taken from the FILENAME so the window is applied before any file is
// opened (a batch reads its own handful of dossiers, never the whole history).
//
// Best-effort by design: an absent directory, an unreadable file, or a dossier
// that does not parse is skipped, not an error. Callers are reporting surfaces —
// one corrupt dossier must degrade the report, never break the caller. minCycle
// <= 0 reads the whole corpus, so callers that cannot establish a window must
// decide for themselves whether that is what they want.
func ReadCommitted(projectRoot string, minCycle int) []*Dossier {
	dir := filepath.Join(projectRoot, "knowledge-base", cyclesDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type numbered struct {
		path  string
		cycle int
	}
	var files []numbered
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".json")
		n, cerr := strconv.Atoi(strings.TrimPrefix(base, "cycle-"))
		if cerr != nil || n < minCycle {
			continue
		}
		files = append(files, numbered{path: filepath.Join(dir, e.Name()), cycle: n})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].cycle < files[j].cycle })
	out := make([]*Dossier, 0, len(files))
	for _, f := range files {
		data, rerr := os.ReadFile(f.path)
		if rerr != nil {
			continue
		}
		d, perr := ParseJSON(data)
		if perr != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}
