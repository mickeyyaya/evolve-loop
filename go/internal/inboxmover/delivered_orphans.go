package inboxmover

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func ReportOrphanDelivered(opts Options, inWave func(id string) bool, reported map[string]bool) ([]string, error) {
	if reported == nil {
		return nil, fmt.Errorf("%w: report-orphan-delivered requires a reported set", ErrBadArgs)
	}
	opts, err := opts.resolved()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(opts.InboxDir, StateDelivered)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var orphans []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := itemIDAt(filepath.Join(dir, e.Name()))
		if id == "" || inWave(id) || reported[id] {
			continue
		}
		reported[id] = true
		orphans = append(orphans, id)
		opts.emitOrphanDelivered(id)
	}
	return orphans, nil
}

func (o Options) emitOrphanDelivered(id string) {
	if o.Signals == nil {
		return
	}
	o.Signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleInbox, Origin: "inboxmover.ReportOrphanDelivered", Kind: signalcenter.KindInboxWarning,
		Severity: signalcenter.SeverityIncident, Code: lifecycle.CodeDeliveredOrphan,
		Reason: "delivered item " + id + " has no member in any wave record; it stays delivered",
		Fields: map[string]string{"id": id, "step": "orphan_delivered"},
	})
}
