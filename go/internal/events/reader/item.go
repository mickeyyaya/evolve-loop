package reader

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
)

const windowSize = 4096

type Item struct {
	Channel string
	Record  channel.Record
	Next    int64
}

func (i Item) ID() string {
	switch {
	case i.Record.Signal != nil:
		s := i.Record.Signal
		return fmt.Sprintf("%d.%d.%s", s.PID, s.Seq, s.TS)
	case i.Record.Source == channel.SourceWatch:
		return fmt.Sprintf("gap.%s.%d", i.Channel, i.Record.Gap.From)
	}
	return fmt.Sprintf("gap.%s.%d", i.Channel, i.Record.Cursor)
}

func itemsOf(name string, b channel.Batch) []Item {
	items := make([]Item, len(b.Records))
	next := b.Next
	for i := len(b.Records) - 1; i >= 0; i-- {
		rec := b.Records[i]
		items[i] = Item{Channel: name, Record: rec, Next: next}
		next = rec.Cursor
		if rec.Source == channel.SourceWatch {
			next = rec.Gap.From
		}
	}
	return items
}

func merge(lists [][]Item) []Item {
	var out []Item
	heads := make([]int, len(lists))
	for {
		best := -1
		for i, l := range lists {
			if heads[i] < len(l) && (best < 0 || earlier(l[heads[i]], lists[best][heads[best]])) {
				best = i
			}
		}
		if best < 0 {
			return out
		}
		out = append(out, lists[best][heads[best]])
		heads[best]++
	}
}

func earlier(a, b Item) bool {
	rankA, atA := orderKey(a)
	rankB, atB := orderKey(b)
	return rankA < rankB || (rankA == rankB && atA.Before(atB))
}

func orderKey(it Item) (int, time.Time) {
	if it.Record.Signal == nil {
		return 0, time.Time{}
	}
	at := stampOf(it.Record.Signal.TS)
	if at.IsZero() {
		return 2, at
	}
	return 1, at
}

type window struct {
	size int
	ring []string
	next int
	set  map[string]bool
}

func newWindow(size int) *window {
	return &window{size: size, set: map[string]bool{}}
}

func (w *window) seen(id string) bool {
	if w.set[id] {
		return true
	}
	if len(w.ring) < w.size {
		w.ring = append(w.ring, id)
	} else {
		delete(w.set, w.ring[w.next])
		w.ring[w.next] = id
	}
	w.next = (w.next + 1) % w.size
	w.set[id] = true
	return false
}

func (r *Reader) dedupe(items []Item) []Item {
	var out []Item
	for _, it := range items {
		if it.Record.Signal != nil && r.window.seen(it.Record.Source+" "+it.ID()) {
			continue
		}
		out = append(out, it)
	}
	return out
}
