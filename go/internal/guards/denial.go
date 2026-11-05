package guards

import (
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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

func DenialEvent(dispatchID string, d Denial) (signalcenter.Event, bool) {
	id, ok := proctree.ParseDispatchID(dispatchID)
	if !ok {
		return signalcenter.Event{}, false
	}
	return signalcenter.Event{
		Module: signalcenter.ModuleGuard, Origin: "guards.DenialEvent", Kind: signalcenter.KindIsolationViolation,
		Code: CodeDenied, Severity: signalcenter.SeverityWarn, Cycle: id.Cycle, RunID: id.Run, Phase: id.Agent,
		Reason: d.Reason, Fields: map[string]string{"guard": d.Guard, "path": d.Path, "dispatch_id": dispatchID},
	}, true
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
	stream := core.SignalStreamPath(projectRoot, id.Cycle)
	chunk, _ := signalcenter.ReadStream(stream, tailOffset(stream))
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
