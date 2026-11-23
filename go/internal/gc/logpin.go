package gc

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

type pinInput struct {
	entries []logEntry
	current string
}

type pinStrategy func(pinInput) map[string]bool

var defaultPins = map[gcpolicy.PinRule]pinStrategy{
	gcpolicy.PinCurrentOrLive: pinCurrentOrLive,
	gcpolicy.PinTailSegment:   pinTailSegment,
}

func (lp logPlanner) strategyFor(rule gcpolicy.PinRule) pinStrategy {
	table := lp.pins
	if table == nil {
		table = defaultPins
	}
	if strategy, ok := table[rule]; ok {
		return strategy
	}
	return pinEveryEntry
}

func pinEveryEntry(in pinInput) map[string]bool {
	out := make(map[string]bool, len(in.entries))
	for _, e := range in.entries {
		out[e.path] = true
	}
	return out
}

func pinCurrentOrLive(in pinInput) map[string]bool {
	out := make(map[string]bool, len(in.entries))
	for _, e := range in.entries {
		out[e.path] = e.path == in.current || logWriterLive(e.path, e.isDir)
	}
	return out
}

func pinTailSegment(in pinInput) map[string]bool {
	out := make(map[string]bool, 1)
	tailBase, tail := int64(-1), ""
	for _, e := range in.entries {
		if base, ok := channel.SegmentBase(filepath.Base(e.path)); ok && base > tailBase {
			tailBase, tail = base, e.path
		}
	}
	if tail != "" {
		out[tail] = true
	}
	return out
}
