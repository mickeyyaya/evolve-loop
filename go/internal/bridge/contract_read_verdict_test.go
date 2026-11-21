package bridge

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	errReadUnmeasured    = errors.New("unmeasured")
	errReadRecordMissing = errors.New("record missing after the fence")
	errReadNoRecord      = errors.New("no access record")
)

type readPoll struct {
	log     []byte
	tag     string
	path    string
	events  []signalcenter.Event
	elapsed time.Duration
	stderr  string
}

func openRecordFailure(p readPoll) error {
	for _, e := range p.events {
		if e.Code == isolation.CodeReadUnmeasured {
			return fmt.Errorf("%w: cause=%s%s (%s)", errReadUnmeasured, e.Fields["cause"], unmeasuredAfter(e.Fields["elapsed"]), e.Code)
		}
	}
	if fenced(p.log, p.tag) && !slices.Contains(parseGrantReports(p.log), p.path) {
		return fmt.Errorf("%w: the log holds the fence of the cell and no grant report of %s", errReadRecordMissing, p.path)
	}
	return fmt.Errorf("%w names the read of %s after %s of polling for the fence (stderr %q)", errReadNoRecord, p.path, p.elapsed.Round(time.Millisecond), p.stderr)
}

func unmeasuredAfter(elapsed string) string {
	if elapsed == "" {
		return ""
	}
	return " after " + elapsed
}
