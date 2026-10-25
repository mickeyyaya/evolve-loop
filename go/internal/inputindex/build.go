package inputindex

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type Request struct {
	ProjectRoot string
	Workspace   string
	Cycle       int
	Phase       string
	LineageMax  int
}

type source struct {
	path, class, kind, phase string
	cycle, attempt           int
}

var extraDeliverables = []source{
	{path: committedset.DecisionFile, kind: "triage-decision", phase: "triage"},
	{path: "acs-verdict.json", kind: "acs-verdict", phase: "audit"},
}

func Build(req Request) (Index, error) {
	if !filepath.IsAbs(req.ProjectRoot) || !filepath.IsAbs(req.Workspace) {
		return Index{}, fmt.Errorf("inputindex: the project root %q and the workspace %q must be absolute", req.ProjectRoot, req.Workspace)
	}
	ids := committedset.LanePin(req.Workspace)
	idx := Index{Schema: Schema, Cycle: req.Cycle, Phase: req.Phase, ItemIDs: ids, Entries: []Entry{}, Notes: []string{}}
	if len(ids) == 0 {
		idx.Notes = append(idx.Notes, "no lane-scope.json todo_ids in the workspace, so the cycle has no item id and no lineage")
	}
	all, notes := Lineage(req.ProjectRoot, req.Cycle, ids)
	idx.Notes = append(idx.Notes, notes...)
	limit := req.LineageMax
	if limit <= 0 {
		limit = DefaultLineageMax
	}
	dropped := max(0, len(all)-limit)
	idx.Lineage, idx.Attempt = all[dropped:], len(all)+1
	sources := slices.Concat(deliverables(req, idx.Attempt), lineageSources(req.ProjectRoot, idx.Lineage, dropped+1), lessonSources(req.ProjectRoot, ids, idx.Lineage))
	for _, s := range sources {
		e, ok, err := newEntry(req.ProjectRoot, s)
		if err != nil {
			return Index{}, err
		}
		if ok {
			e.ID = fmt.Sprintf("%c%d", s.class[0], len(idx.Entries)+1)
			idx.Entries = append(idx.Entries, e)
		}
	}
	return idx, nil
}

func deliverables(req Request, attempt int) []source {
	own := phasecontract.RegistryKey(req.Phase)
	var out []source
	for _, c := range phasecontract.Contracts() {
		if c.NoArtifact || c.ArtifactName == "" || c.WriteTarget != phasecontract.TargetWorkspace || c.Phase == own {
			continue
		}
		out = append(out, source{path: c.ArtifactName, kind: kindOf(c.ArtifactName), phase: c.Phase})
	}
	out = append(out, extraDeliverables...)
	handoffs, _ := filepath.Glob(filepath.Join(req.Workspace, "handoff-*.json"))
	for _, h := range handoffs {
		out = append(out, source{path: filepath.Base(h), kind: "handoff-asks"})
	}
	for i := range out {
		out[i].path = filepath.Join(req.Workspace, out[i].path)
		out[i].class, out[i].cycle, out[i].attempt = ClassDeliverable, req.Cycle, attempt
	}
	slices.SortFunc(out, func(a, b source) int { return strings.Compare(a.path, b.path) })
	return out
}

func kindOf(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func newEntry(root string, s source) (Entry, bool, error) {
	info, err := os.Lstat(s.path)
	if errors.Is(err, fs.ErrNotExist) || err == nil && info.Mode().Type() == fs.ModeSymlink {
		return Entry{}, false, nil
	}
	var body []byte
	if err == nil {
		body, err = os.ReadFile(s.path)
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("inputindex: read %s: %w", s.path, err)
	}
	rel, _ := filepath.Rel(root, s.path)
	return Entry{Class: s.class, Kind: s.kind, Path: filepath.ToSlash(rel), Cycle: s.cycle, Phase: s.phase, Attempt: s.attempt,
		Summary: summaryOf(s.path, body), Bytes: int64(len(body)), SHA256: digest(body)}, true, nil
}
