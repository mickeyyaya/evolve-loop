package reader

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
)

type sinceMode int

const (
	sinceNew sinceMode = iota
	sinceAll
	sinceLast
	sinceCursors
	sinceTime
)

type Since struct {
	mode    sinceMode
	cursors map[string]int64
	at      time.Time
}

func ParseSince(value string) (Since, error) {
	switch value {
	case "", "new":
		return Since{mode: sinceNew}, nil
	case "all":
		return Since{mode: sinceAll}, nil
	case "last":
		return Since{mode: sinceLast}, nil
	}
	if at, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return Since{mode: sinceTime, at: at}, nil
	}
	cursors, err := parseCursors(value)
	if err != nil {
		return Since{}, err
	}
	return Since{mode: sinceCursors, cursors: cursors}, nil
}

func parseCursors(value string) (map[string]int64, error) {
	cursors := map[string]int64{}
	for _, part := range strings.Split(value, ",") {
		name, offset, ok := strings.Cut(part, ":")
		cursor, err := strconv.ParseInt(offset, 10, 64)
		_, dup := cursors[name]
		if !ok || err != nil || cursor < 0 || dup || !filter.ValidChannelName(name) {
			return nil, fmt.Errorf("%w: --since %q is not all, new, last, an RFC 3339 time or CHANNEL:CURSOR[,CHANNEL:CURSOR]", filter.ErrUsage, value)
		}
		cursors[name] = cursor
	}
	return cursors, nil
}

func (s Since) startOf(l *channel.Log, name string) (int64, error) {
	switch s.mode {
	case sinceAll:
		return 0, nil
	case sinceLast:
		return l.Last()
	case sinceCursors:
		return s.cursors[name], nil
	case sinceTime:
		return firstAtOrAfter(l, s.at)
	}
	return l.End()
}

func (s Since) willStart(l *channel.Log) (int64, error) {
	if _, named := s.cursors[loopChannel]; s.mode == sinceCursors && !named {
		return l.End()
	}
	return s.startOf(l, loopChannel)
}

func firstAtOrAfter(l *channel.Log, at time.Time) (int64, error) {
	cur := int64(0)
	for {
		b, err := l.ReadN(cur, readBudget)
		if err != nil || len(b.Records) == 0 {
			return b.Next, err
		}
		for _, rec := range b.Records {
			if rec.Signal != nil && !stampOf(rec.Signal.TS).Before(at) {
				return rec.Cursor, nil
			}
		}
		cur = b.Next
	}
}

func stampOf(ts string) time.Time {
	at, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}
	}
	return at
}
