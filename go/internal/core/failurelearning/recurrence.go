package failurelearning

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

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
