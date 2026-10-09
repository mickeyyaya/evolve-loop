package reader

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
)

const (
	loopChannel = "loop"
	dirMode     = 0o755
)

type follower struct {
	name string
	log  *channel.Log
	cur  int64
}

func hasWildcard(selectors []string) bool {
	return slices.ContainsFunc(selectors, func(s string) bool { return strings.ContainsAny(s, "*>") })
}

func (r *Reader) resolve() error {
	names, err := r.knownNames()
	if err != nil {
		return err
	}
	selected, err := filter.ResolveChannels(r.cfg.Selectors, names)
	if err != nil {
		return fmt.Errorf("reader: %w", err)
	}
	for _, name := range selected {
		if r.follower(name) != nil {
			continue
		}
		if err := r.follow(name); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) knownNames() ([]string, error) {
	names := make([]string, 0, len(r.cfg.Catalog))
	for _, ch := range r.cfg.Catalog {
		names = append(names, ch.Name)
	}
	if !r.wildcard {
		return names, nil
	}
	entries, err := os.ReadDir(r.cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("reader: list channels: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && filter.ValidChannelName(e.Name()) && !slices.Contains(names, e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func (r *Reader) follow(name string) error {
	l, err := r.newLog(name)
	if err != nil {
		return err
	}
	start := int64(0)
	if !r.opened {
		start, err = r.cfg.Since.startOf(l, name)
		if err != nil {
			return err
		}
	}
	r.followers = append(r.followers, &follower{name: name, log: l, cur: start})
	return nil
}

func (r *Reader) newLog(name string) (*channel.Log, error) {
	l, err := channel.New(r.cfg.Root, name, channel.Config{})
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	if err := os.MkdirAll(l.Dir(), dirMode); err != nil {
		return nil, fmt.Errorf("reader: make channel %s: %w", name, err)
	}
	return l, nil
}

func (r *Reader) follower(name string) *follower {
	for _, f := range r.followers {
		if f.name == name {
			return f
		}
	}
	return nil
}

func (r *Reader) readLog(l *channel.Log, from int64) (channel.Batch, error) {
	b, err := r.read(l, from, readBudget)
	if !errors.Is(err, fs.ErrNotExist) {
		return b, err
	}
	if err := os.MkdirAll(l.Dir(), dirMode); err != nil {
		return b, fmt.Errorf("reader: make channel %s: %w", l.Dir(), err)
	}
	r.dirty = true
	return r.read(l, from, readBudget)
}

func (r *Reader) readFollowers() ([]Item, error) {
	batches := make([]channel.Batch, len(r.followers))
	for i, f := range r.followers {
		b, err := r.readLog(f.log, f.cur)
		if err != nil {
			return nil, err
		}
		batches[i] = b
	}
	lists := make([][]Item, len(r.followers))
	for i, f := range r.followers {
		lists[i] = itemsOf(f.name, batches[i])
		f.cur = batches[i].Next
	}
	return merge(lists), nil
}

func (r *Reader) logs() []*channel.Log {
	logs := make([]*channel.Log, 0, len(r.followers)+1)
	for _, f := range r.followers {
		logs = append(logs, f.log)
	}
	if !r.will.selected {
		logs = append(logs, r.will.log)
	}
	return logs
}

func (r *Reader) targets() (wake.Targets, error) {
	var t wake.Targets
	if r.wildcard {
		t.Dirs = append(t.Dirs, r.cfg.Root)
	}
	for _, l := range r.logs() {
		t.Dirs = append(t.Dirs, l.Dir())
		tail, err := tailOf(l)
		if err != nil {
			return t, err
		}
		if tail != "" {
			t.Files = append(t.Files, tail)
		}
	}
	t.Pids = r.will.pids()
	return t, nil
}

func tailOf(l *channel.Log) (string, error) {
	segs, err := l.Segments()
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(segs) == 0 {
		return "", nil
	}
	return segs[len(segs)-1].Path, nil
}

func sameTargets(a, b wake.Targets) bool {
	return slices.Equal(a.Dirs, b.Dirs) && slices.Equal(a.Files, b.Files) && slices.Equal(a.Pids, b.Pids)
}

func (r *Reader) armWith(t wake.Targets) error {
	for _, dir := range t.Dirs {
		if err := os.MkdirAll(dir, dirMode); err != nil {
			return fmt.Errorf("reader: make %s: %w", dir, err)
		}
	}
	if err := r.ports.Waiter.Arm(t); err != nil {
		return fmt.Errorf("reader: arm: %w", err)
	}
	r.armed, r.dirty = t, false
	return nil
}

func (r *Reader) placeWill(recs []channel.Record) []Item {
	var out []Item
	for _, rec := range recs {
		if rec.Gap != nil {
			out = append(out, Item{Channel: loopChannel, Record: rec, Next: rec.Gap.To})
			continue
		}
		if f := r.holder(rec); f != nil {
			out = append(out, Item{Channel: f.name, Record: rec, Next: f.cur})
		}
	}
	return out
}

func (r *Reader) holder(rec channel.Record) *follower {
	for _, f := range r.followers {
		route, ok := r.routes[f.name]
		if ok && route.Match(filter.Record{Source: rec.Source, Signal: rec.Signal}) {
			return f
		}
	}
	return nil
}
