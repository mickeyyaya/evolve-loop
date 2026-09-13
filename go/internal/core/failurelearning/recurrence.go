package failurelearning

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// recurrenceClosure is gap-G1 production wiring (cycle-662): the deterministic
// retro-closeout seam upserts the failing lesson pattern into the recurrence
// ledger keyed by the failing cycle, so Count() reflects real history instead
// of staying 0 forever. Escalator/Autofiler are nil here — escalation APPLY
// stays boundary-only (the live consult site reads the ledger; it must not
// race inboxmover.Claim from the mid-cycle closeout). Best-effort: a ledger
// failure is one WARN naming the failing call (op = load | record | save) and
// never masks the phase failure; op=record is dormant today (RecordClosure
// errs only through the nil-guarded escalator/autofiler) and reported the day
// it is not.
func (e *Engine) recurrenceClosure(f Failure, pattern string) {
	if f.ProjectRoot == "" || strings.TrimSpace(pattern) == "" {
		return
	}
	path := filepath.Join(paths.EvolveDirOf(f.ProjectRoot), "recurrence-ledger.json")
	led, err := recurrence.Load(path)
	op := "load"
	if err == nil {
		op, err = "record", led.RecordClosure(pattern, f.Cycle, nil, nil, recurrence.DefaultEscalationPolicy())
	}
	if err == nil {
		op, err = "save", led.Save(path)
	}
	if err != nil {
		e.warn(f, CodeRecurrenceLedgerFailed, "recurrence ledger "+op+" failed: "+err.Error(),
			map[string]string{"step": "recurrence", "op": op, "path": path})
	}
}
