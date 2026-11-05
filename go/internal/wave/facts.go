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

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
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
	ID           int                `json:"id"`
	Todo         string             `json:"todo,omitempty"`
	Phase        string             `json:"phase,omitempty"`
	Verdict      string             `json:"verdict,omitempty"`
	Termination  string             `json:"termination,omitempty"`
	Shipped      bool               `json:"shipped"`
	Resumed      bool               `json:"resumed,omitempty"`
	QuotaPauses  int                `json:"quota_pauses"`
	Conflict     bool               `json:"conflict,omitempty"`
	Spend        cyclespend.Spend   `json:"spend"`
	LLMVerdict   cyclespend.Verdict `json:"llm_verdict"`
	HandoffWould int                `json:"handoff_would_stops,omitempty"`
	HandoffStop  string             `json:"handoff_stop,omitempty"`
	SpendLimit   string             `json:"spend_limit,omitempty"`
	StartedAt    time.Time          `json:"-"`
	EndedAt      time.Time          `json:"-"`
	failAt       time.Time
}

func ReadCycles(root string, floor, ceiling int) ([]Cycle, []string) {
	cycles, warnings, failures := readCycleRange(root, floor, ceiling)
	return cycles, foldFailures(warnings, failures)
}

func readCycleRange(root string, floor, ceiling int) ([]Cycle, []string, []ReadFailure) {
	w := Bounds{Floor: floor, Ceiling: ceiling}
	evolveDir := paths.EvolveDirOf(root)
	dossiers := map[int]*dossier.Dossier{}
	committed, failures := dossier.ReadDirChecked(dossier.CyclesDir(root), floor+1, ceiling)
	for _, d := range committed {
		if w.Holds(d.Cycle) {
			dossiers[d.Cycle] = d
		}
	}
	ids := map[int]bool{}
	for id := range dossiers {
		ids[id] = true
	}
	failures = appendFailure(failures, filepath.Join(evolveDir, "runs"), collectIDs(filepath.Join(evolveDir, "runs"), runDirName, w, ids))
	failures = appendFailure(failures, filepath.Join(evolveDir, "landing"), collectIDs(filepath.Join(evolveDir, "landing"), landingFileName, w, ids))
	live, liveFailures := readLive(evolveDir)
	failures = append(failures, liveFailures...)
	var cycles []Cycle
	var warnings []string
	for _, id := range sortedKeys(ids) {
		c, warn, bad := readCycle(root, id, live, dossiers[id])
		cycles = append(cycles, c)
		warnings = append(warnings, warn...)
		failures = append(failures, bad...)
	}
	return cycles, warnings, failures
}

func readLive(evolveDir string) (cyclestate.CycleState, []ReadFailure) {
	livePath := core.ResolveCycleStatePath(evolveDir)
	live, _, err := readState(livePath)
	if errors.Is(err, fs.ErrNotExist) {
		return live, nil
	}
	return live, appendFailure(nil, livePath, err)
}

func appendFailure(failures []ReadFailure, path string, err error) []ReadFailure {
	if err == nil {
		return failures
	}
	return append(failures, ReadFailure{Path: path, Err: err})
}

func foldFailures(warnings []string, failures []ReadFailure) []string {
	for _, f := range failures {
		warnings = append(warnings, f.Error())
	}
	return warnings
}

func collectIDs(dir string, name *regexp.Regexp, w Bounds, ids map[int]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		m := name.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if id, err := strconv.Atoi(m[1]); err == nil && w.Holds(id) {
			ids[id] = true
		}
	}
	return nil
}

func sortedKeys(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func readCycle(root string, id int, live cyclestate.CycleState, d *dossier.Dossier) (Cycle, []string, []ReadFailure) {
	ws := paths.RunWorkspace(root, id)
	c := Cycle{ID: id}
	var failures []ReadFailure
	todo, err := readTodo(ws)
	failures = appendFailure(failures, committedset.LanePinPath(ws), err)
	c.Todo = todo
	st, ok, stateFailures := workspaceState(ws, id, live)
	failures = append(failures, stateFailures...)
	if ok {
		c.Phase, c.Shipped = st.Phase, st.Shipped
	}
	var spendFailures []ReadFailure
	c.Spend, spendFailures = readSpend(ws)
	failures = append(failures, spendFailures...)
	streamPath := filepath.Join(ws, signalcenter.StreamFileName)
	warnings, streamErr := applySignals(&c, streamPath)
	failures = appendFailure(failures, streamPath, streamErr)
	intentPath := landing.IntentPath(paths.EvolveDirOf(root), id)
	intent, found, err := landing.ReadIntent(intentPath)
	failures = appendFailure(failures, intentPath, err)
	c.Shipped = c.Shipped || (found && intent.Status == landing.IntentComplete)
	if d != nil {
		if c.Verdict == "" {
			c.Verdict = d.FinalVerdict
		}
		c.Shipped = c.Shipped || d.CommitSHA != ""
		c.StartedAt, c.EndedAt = phaseSpan(c.StartedAt, c.EndedAt, d.Phases)
	}
	c.LLMVerdict = readVerdict(ws, c)
	return c, warnings, failures
}

func readTodo(ws string) (string, error) {
	scope, err := core.ReadLaneScope(ws)
	if err != nil || scope == nil || len(scope.TodoIDs) == 0 {
		return "", err
	}
	return scope.TodoIDs[0], nil
}

func readVerdict(ws string, c Cycle) cyclespend.Verdict {
	res, err := llmcalls.ReadWorkspace(ws)
	if err != nil || len(res.Records) == 0 {
		return cyclespend.Verdict{}
	}
	return cyclespend.VerdictOf(res.Records, c.failAt, c.Shipped)
}

func readSpend(ws string) (cyclespend.Spend, []ReadFailure) {
	spend, err := cyclespend.Read(ws)
	if !errors.Is(err, cyclespend.ErrLedger) {
		return spend, nil
	}
	return spend, appendFailure(nil, llmcalls.Path(ws), err)
}

func workspaceState(ws string, id int, live cyclestate.CycleState) (cyclestate.CycleState, bool, []ReadFailure) {
	if live.CycleID == id {
		return live, true, nil
	}
	var failures []ReadFailure
	for _, name := range []string{core.CycleStateFile, core.RunStateFile} {
		path := filepath.Join(ws, name)
		st, ok, err := readState(path)
		if ok {
			return st, true, failures
		}
		if !errors.Is(err, fs.ErrNotExist) {
			failures = appendFailure(failures, path, err)
		}
	}
	return cyclestate.CycleState{}, false, failures
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

func applySignals(c *Cycle, path string) ([]string, error) {
	chunk, err := signalcenter.ReadStream(path, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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
		applyStopSignals(c, e)
		if blocksCycle(e) {
			c.failAt = earliest(c.failAt, eventTime(e))
		}
	}
	if chunk.Skipped > 0 {
		return []string{fmt.Sprintf("cycle-%d: skipped %d malformed signal line(s) in %s", c.ID, chunk.Skipped, path)}, nil
	}
	return nil, nil
}

func blocksCycle(e signalcenter.Event) bool {
	return e.Kind == signalcenter.KindGateRejected || (e.Kind == signalcenter.KindPhaseOutcome && e.Fields["verdict"] == "FAIL")
}

func applyStopSignals(c *Cycle, e signalcenter.Event) {
	if e.Code == cyclespend.SignalCode {
		c.SpendLimit = e.Fields[cyclespend.FieldDecision]
	}
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
