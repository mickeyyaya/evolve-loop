package guards

import (
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeDenied signalcenter.Code = "GUARD_DENIED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleGuard, CodeDenied, "a kernel guard denied a tool call of a phase cell; the hook process writes it to the stream of the run, run_id and phase come from the dispatch id, fields guard, path and dispatch_id; when a stall limit ends that dispatch, the bridge reads it back and ends the run with cause guard_refused, so the dispatch chain stops (a denial cannot depend on the family)")
}

type Denial struct {
	Guard  string
	Path   string
	Reason string
	At     time.Time
}

func (d Denial) Summary() string { return "guard " + d.Guard + " denied " + d.Path + ": " + d.Reason }

func DeniedPath(in core.GuardInput) string {
	for _, key := range []string{"file_path", "command", "subagent_type"} {
		if v := strField(in, key); v != "" {
			return v
		}
	}
	return ""
}

func DenialStream(projectRoot string, cycle int) string {
	if cycle > 0 {
		return filepath.Join(paths.RunWorkspace(projectRoot, cycle), signalcenter.StreamFileName)
	}
	return filepath.Join(paths.EvolveDirOf(projectRoot), signalcenter.StreamFileName)
}

func RecordDenial(projectRoot, dispatchID string, d Denial) {
	id, ok := proctree.ParseDispatchID(dispatchID)
	if !ok {
		return
	}
	center := signalcenter.New(signalcenter.WithPID(os.Getpid()))
	center.Subscribe(center.NDJSONSink(func(cycle int) string { return DenialStream(projectRoot, cycle) }))
	center.Emit(signalcenter.Event{
		Module: signalcenter.ModuleGuard, Origin: "guards.RecordDenial", Kind: signalcenter.KindIsolationViolation,
		Code: CodeDenied, Severity: signalcenter.SeverityWarn, Cycle: id.Cycle, RunID: id.Run, Phase: id.Agent,
		Reason: d.Reason, Fields: map[string]string{"guard": d.Guard, "path": d.Path, "dispatch_id": dispatchID},
	})
	center.Flush()
}

const denialWindowBytes = 4 << 20

func tailOffset(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return max(0, fi.Size()-denialWindowBytes)
}

func DenialOf(projectRoot, dispatchID string) (Denial, bool) {
	id, ok := proctree.ParseDispatchID(dispatchID)
	if !ok {
		return Denial{}, false
	}
	chunk, _ := signalcenter.ReadStream(DenialStream(projectRoot, id.Cycle), tailOffset(DenialStream(projectRoot, id.Cycle)))
	var denial Denial
	found := false
	for _, e := range chunk.Events {
		if e.Code == CodeDenied && e.Fields["dispatch_id"] == dispatchID {
			at, _ := time.Parse(time.RFC3339Nano, e.TS)
			denial, found = Denial{Guard: e.Fields["guard"], Path: e.Fields["path"], Reason: e.Reason, At: at}, true
		}
	}
	return denial, found
}
