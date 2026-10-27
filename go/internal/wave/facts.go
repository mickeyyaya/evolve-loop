package wave

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	runDirName      = regexp.MustCompile(`^cycle-(\d+)$`)
	landingFileName = regexp.MustCompile(`^cycle-(\d+)\.json$`)
)

type Cycle struct {
	ID           int              `json:"id"`
	Todo         string           `json:"todo,omitempty"`
	Phase        string           `json:"phase,omitempty"`
	Verdict      string           `json:"verdict,omitempty"`
	Termination  string           `json:"termination,omitempty"`
	Shipped      bool             `json:"shipped"`
	QuotaPauses  int              `json:"quota_pauses"`
	Conflict     bool             `json:"conflict,omitempty"`
	Spend        cyclespend.Spend `json:"spend"`
	HandoffWould int              `json:"handoff_would_stops,omitempty"`
	HandoffStop  string           `json:"handoff_stop,omitempty"`
	StartedAt    time.Time        `json:"-"`
	EndedAt      time.Time        `json:"-"`
}

type cycleWindow struct{ floor, ceiling int }

func (w cycleWindow) holds(id int) bool { return id > w.floor && (w.ceiling == 0 || id <= w.ceiling) }

func ReadCycles(root string, floor, ceiling int) ([]Cycle, []string) {
	w := cycleWindow{floor: floor, ceiling: ceiling}
	evolveDir := paths.EvolveDirOf(root)
	dossiers := map[int]*dossier.Dossier{}
	for _, d := range dossier.ReadCommitted(root, floor+1) {
		if w.holds(d.Cycle) {
			dossiers[d.Cycle] = d
		}
	}
	ids := map[int]bool{}
	for id := range dossiers {
		ids[id] = true
	}
	collectIDs(filepath.Join(evolveDir, "runs"), runDirName, w, ids)
	collectIDs(filepath.Join(evolveDir, "landing"), landingFileName, w, ids)
	livePath := core.ResolveCycleStatePath(evolveDir)
	live, _, err := readState(livePath)
	var cycles []Cycle
	var warnings []string
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		warnings = append(warnings, fmt.Sprintf("%s: %v", livePath, err))
	}
	for _, id := range sortedKeys(ids) {
		c, warn := readCycle(root, id, live, dossiers[id])
		cycles = append(cycles, c)
		warnings = append(warnings, warn...)
	}
	return cycles, warnings
}

func collectIDs(dir string, name *regexp.Regexp, w cycleWindow, ids map[int]bool) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		m := name.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if id, err := strconv.Atoi(m[1]); err == nil && w.holds(id) {
			ids[id] = true
		}
	}
}

func sortedKeys(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func readCycle(root string, id int, live cyclestate.CycleState, d *dossier.Dossier) (Cycle, []string) {
	ws := paths.RunWorkspace(root, id)
	c := Cycle{ID: id}
	if todos := core.LaneScopeIDs(ws); len(todos) > 0 {
		c.Todo = todos[0]
	}
	st, ok, warnings := workspaceState(ws, id, live)
	if ok {
		c.Phase, c.Shipped = st.Phase, st.Shipped
	}
	c.Spend, warnings = readSpend(ws, id, warnings)
	warnings = append(warnings, applySignals(&c, filepath.Join(ws, signalcenter.StreamFileName))...)
	intent, found, err := landing.ReadIntent(landing.IntentPath(paths.EvolveDirOf(root), id))
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("cycle-%d: landing intent: %v", id, err))
	}
	c.Shipped = c.Shipped || (found && intent.Status == landing.IntentComplete)
	if d != nil {
		if c.Verdict == "" {
			c.Verdict = d.FinalVerdict
		}
		c.Shipped = c.Shipped || d.CommitSHA != ""
		c.StartedAt, c.EndedAt = phaseSpan(c.StartedAt, c.EndedAt, d.Phases)
	}
	return c, warnings
}

func readSpend(ws string, id int, warnings []string) (cyclespend.Spend, []string) {
	spend, err := cyclespend.Read(ws)
	if errors.Is(err, cyclespend.ErrLedger) {
		warnings = append(warnings, fmt.Sprintf("cycle-%d: llm spend: %v", id, err))
	}
	return spend, warnings
}

func workspaceState(ws string, id int, live cyclestate.CycleState) (cyclestate.CycleState, bool, []string) {
	if live.CycleID == id {
		return live, true, nil
	}
	var warnings []string
	for _, name := range []string{core.CycleStateFile, core.RunStateFile} {
		st, ok, err := readState(filepath.Join(ws, name))
		if ok {
			return st, true, warnings
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			warnings = append(warnings, fmt.Sprintf("cycle-%d: %s: %v", id, name, err))
		}
	}
	return cyclestate.CycleState{}, false, warnings
}

func readState(path string) (cyclestate.CycleState, bool, error) {
	var st cyclestate.CycleState
	b, err := os.ReadFile(path)
	if err != nil {
		return st, false, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, false, err
	}
	return st, st.CycleID > 0, nil
}

func applySignals(c *Cycle, path string) []string {
	chunk, err := signalcenter.ReadStream(path, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("cycle-%d: %v", c.ID, err)}
	}
	for _, e := range chunk.Events {
		c.StartedAt = earliest(c.StartedAt, eventTime(e))
		switch e.Kind {
		case signalcenter.KindCycleSealed:
			if verdict := e.Fields[cyclestate.SealedVerdictField]; verdict != "" || c.Verdict == "" {
				c.Verdict, c.EndedAt = verdict, eventTime(e)
				c.Termination = e.Fields[cyclestate.SealedTerminationField]
			}
		case signalcenter.KindQuotaPaused:
			c.QuotaPauses++
		case signalcenter.KindShipError:
			c.Conflict = c.Conflict || shiperr.IsLandingConflict(e.Code)
		}
		applyHandoff(c, e)
	}
	if chunk.Skipped > 0 {
		return []string{fmt.Sprintf("cycle-%d: skipped %d malformed signal line(s) in %s", c.ID, chunk.Skipped, path)}
	}
	return nil
}

func applyHandoff(c *Cycle, e signalcenter.Event) {
	if e.Code != core.CodeRepairHandoff {
		return
	}
	switch e.Fields[cyclestate.RepairDecisionField] {
	case cyclestate.RepairDecisionWould:
		c.HandoffWould++
	case cyclestate.RepairDecisionStop:
		c.HandoffStop = e.Fields[cyclestate.RepairReasonField] + ": " + e.Fields[cyclestate.RepairBlockersField]
	}
}

func CycleFloor(evolveDir string) (int, error) {
	st, ok, err := cyclestate.ReadState(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return st.HighestCycleNumber(), nil
}
