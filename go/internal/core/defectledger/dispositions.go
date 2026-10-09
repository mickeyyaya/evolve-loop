package defectledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func (l *Ledger) ReadDispositions(req Request, ancestorCycle int) (map[string]Entry, []cyclestate.Diagnostic, bool) {
	path := filepath.Join(req.Workspace, DispositionsFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Entry{}, []cyclestate.Diagnostic{{Severity: "warning",
				Message: fmt.Sprintf("defect ledger: no %s in the workspace — every defect inherited from cycle-%d is unaccounted for", DispositionsFile, ancestorCycle)}}, false
		}
		msg := fmt.Sprintf("defect ledger: read %s: %s", DispositionsFile, err.Error())
		l.emit("Ledger.ReadDispositions", req, CodeDispositionsUnreadable, msg, map[string]string{"step": "read", "blocked": "true", "op": "read", "path": path})
		return nil, []cyclestate.Diagnostic{errorDiag(msg)}, true
	}
	var doc dispositionDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		l.emit("Ledger.ReadDispositions", req, CodeDispositionsUnreadable, fmt.Sprintf("defect ledger: %s is unparseable (%s)", DispositionsFile, err.Error()),
			map[string]string{"step": "read", "blocked": "true", "op": "parse", "path": path})
		return nil, []cyclestate.Diagnostic{errorDiag(fmt.Sprintf("defect ledger: %s is unparseable (%s) — a continuation cannot be graded against claims that cannot be read. Expected schema:\n%s\n(`evidence` may also be an array of citation strings; `status` is exactly FIXED — with resolvable evidence — or DEFERRED, with a reason.)",
			DispositionsFile, err.Error(), DispositionsSchemaExample))}, true
	}
	claims := make(map[string]Entry, len(doc.Dispositions))
	for _, d := range doc.Dispositions {
		claims[d.ID] = Entry{ID: d.ID, Status: d.Status, Evidence: d.Evidence.joined(), Reason: d.Reason}
	}
	return claims, nil, false
}

func (l *Ledger) Preflight(req Request, ancestorCycle int, ancestor []Entry, claims map[string]Entry) []cyclestate.Diagnostic {
	var open, uncovered []string
	for _, a := range ancestor {
		if a.Status != StatusOpen {
			continue
		}
		open = append(open, a.ID)
		if _, has := claims[a.ID]; !has {
			uncovered = append(uncovered, a.ID)
		}
	}
	if len(open) == 0 || len(uncovered) == 0 {
		return nil
	}
	path := filepath.Join(req.Workspace, DispositionsFile)
	cycle := strconv.Itoa(ancestorCycle)
	if _, err := os.Stat(path); err != nil {
		msg := fmt.Sprintf("defect ledger: %s — this workspace holds no %s at all, so 0 of %d defect(s) inherited from cycle-%d are dispositioned. This file is re-authored IN FULL every cycle; an ancestor's copy is never inherited. Write one entry per inherited id, status FIXED (with resolvable evidence) or DEFERRED (with a reason).",
			PreflightMissingMarker, DispositionsFile, len(open), ancestorCycle)
		l.emit("Ledger.Preflight", req, CodeDispositionsMissing, msg, map[string]string{"step": "preflight", "blocked": "true", "ancestor_cycle": cycle, "open": strconv.Itoa(len(open)), "path": path})
		return []cyclestate.Diagnostic{errorDiag(msg)}
	}
	covered := len(open) - len(uncovered)
	head := fmt.Sprintf("defect ledger: %s — %s covers %d of %d defect(s) inherited from cycle-%d", PreflightIncompleteMarker, DispositionsFile, covered, len(open), ancestorCycle)
	bounded, truncated := boundedIDs(uncovered)
	l.emit("Ledger.Preflight", req, CodeDispositionsIncomplete, head, map[string]string{
		"step": "preflight", "blocked": "true", "ancestor_cycle": cycle, "open": strconv.Itoa(len(open)), "covered": strconv.Itoa(covered),
		"uncovered": bounded, "uncovered_truncated": truncated, "path": path,
	})
	return []cyclestate.Diagnostic{errorDiag(fmt.Sprintf("%s; uncovered: [%s]. Every inherited id needs its own entry in THIS cycle's file.", head, strings.Join(uncovered, ", ")))}
}
