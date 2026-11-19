package core

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeScopeDelta signalcenter.Code = "ORCHESTRATOR_SCOPE_DELTA"

const scopeDeltaFileName = "scope-delta.json"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeScopeDelta, "the build handoff floor found changed paths outside declared scope (ADR-0087). fields.total, fields.disposition_keep, fields.disposition_carve, and fields.disposition_refuse hold the counts; scope-delta.json lists the paths. WARN for carve or refuse, INFO when all are keep; reports only (shadow)")
}

type scopeDeltaArtifactEntry struct {
	Path        string `json:"path"`
	Class       string `json:"class"`
	Disposition string `json:"disposition"`
}

type scopeDeltaArtifactDoc struct {
	Entries []scopeDeltaArtifactEntry `json:"entries"`
}

type scopeDeltaCounts struct {
	total  int
	keep   int
	carve  int
	refuse int
}

func ScopeDeltaFloorChecks(protected func(string) bool) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		if in.Worktree == "" || in.ProjectRoot == "" || in.Cycle < 1 {
			return nil
		}
		evolveDir := filepath.Join(in.ProjectRoot, ".evolve")
		declared, err := LaneDeclaredFiles(evolveDir, in.Cycle)
		if err != nil {
			fmt.Fprintf(in.errOut(), "[build-floor] WARN scope-delta: %v; the declared scope is unknown, so no delta is reported\n", err)
			return nil
		}
		if len(declared) == 0 {
			return nil
		}
		changed := ChangedWorktreePathsSinceBase(ctx, in.Worktree, in.WorktreeBaseSHA)
		entries := outOfScopeEntries(ClassifyLaneChanges(evolveDir, in.Cycle, declared, changed, protected, in.errOut()))
		if len(entries) == 0 {
			if in.Workspace != "" {
				_ = os.Remove(filepath.Join(in.Workspace, scopeDeltaFileName))
			}
			return nil
		}
		if in.Workspace != "" {
			if scopeDeltaMatchesWorkspace(in.Workspace, entries) {
				return nil
			}
			writeScopeDeltaArtifact(in.Workspace, entries, in.errOut())
		}
		counts := countScopeDeltaDispositions(entries)
		in.Signals.Emit(scopeDeltaSummaryEvent(in, counts))
		fmt.Fprintf(in.errOut(), "[build-floor] scope-delta: %d path(s) outside declared scope (keep=%d carve=%d refuse=%d)\n",
			counts.total, counts.keep, counts.carve, counts.refuse)
		return nil
	}
}

func outOfScopeEntries(classified []scopedelta.Entry) []scopeDeltaArtifactEntry {
	var out []scopeDeltaArtifactEntry
	for _, e := range classified {
		if e.Class == scopedelta.ClassInScope {
			continue
		}
		out = append(out, scopeDeltaArtifactEntry{
			Path:        e.Path,
			Class:       string(e.Class),
			Disposition: string(e.Disposition),
		})
	}
	slices.SortFunc(out, compareScopeDeltaEntries)
	return out
}

func compareScopeDeltaEntries(a, b scopeDeltaArtifactEntry) int {
	return cmp.Or(strings.Compare(a.Path, b.Path), strings.Compare(a.Class, b.Class), strings.Compare(a.Disposition, b.Disposition))
}

func scopeDeltaMatchesWorkspace(workspace string, current []scopeDeltaArtifactEntry) bool {
	raw, err := os.ReadFile(filepath.Join(workspace, scopeDeltaFileName))
	if err != nil {
		return false
	}
	var doc scopeDeltaArtifactDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false
	}
	prev := slices.Clone(doc.Entries)
	slices.SortFunc(prev, compareScopeDeltaEntries)
	return slices.Equal(prev, current)
}

func writeScopeDeltaArtifact(workspace string, entries []scopeDeltaArtifactEntry, warn io.Writer) {
	body, _ := json.MarshalIndent(scopeDeltaArtifactDoc{Entries: entries}, "", "  ")
	body = append(body, '\n')
	path := filepath.Join(workspace, scopeDeltaFileName)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fmt.Fprintf(warn, "[build-floor] WARN scope-delta: write %s: %v\n", path, err)
	}
}

func countScopeDeltaDispositions(entries []scopeDeltaArtifactEntry) scopeDeltaCounts {
	c := scopeDeltaCounts{total: len(entries)}
	for _, e := range entries {
		switch scopedelta.Disposition(e.Disposition) {
		case scopedelta.DispositionKeep:
			c.keep++
		case scopedelta.DispositionCarve:
			c.carve++
		case scopedelta.DispositionRefuse:
			c.refuse++
		}
	}
	return c
}

func scopeDeltaSummaryEvent(in ReviewInput, c scopeDeltaCounts) signalcenter.Event {
	severity := signalcenter.SeverityInfo
	if c.keep < c.total {
		severity = signalcenter.SeverityWarn
	}
	return signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: in.Phase,
		Module: signalcenter.ModuleOrchestrator, Origin: "ScopeDeltaFloorChecks",
		Kind: signalcenter.KindGatePassed, Severity: severity, Code: CodeScopeDelta,
		Reason: fmt.Sprintf("%d changed path(s) outside declared scope (%d keep, %d carve, %d refuse)", c.total, c.keep, c.carve, c.refuse),
		Fields: map[string]string{
			"total":              strconv.Itoa(c.total),
			"disposition_keep":   strconv.Itoa(c.keep),
			"disposition_carve":  strconv.Itoa(c.carve),
			"disposition_refuse": strconv.Itoa(c.refuse),
		},
	}
}
