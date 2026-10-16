// Package recurrence is the deterministic recurrence ledger over lesson
// `pattern:` keys plus the automatic inbox weight-escalation policy.
package recurrence

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

type Entry struct {
	Pattern      string `json:"pattern"`
	Cycles       []int  `json:"cycles"`
	Count        int    `json:"count"`
	LastSeen     int    `json:"last_seen"`
	FixItemID    string `json:"fix_item_id,omitempty"`
	FixLandedSHA string `json:"fix_landed_sha,omitempty"`
	BumpedCycle  int    `json:"bumped_cycle,omitempty"`
	Autofiled    bool   `json:"autofiled,omitempty"`
	Generic      bool   `json:"generic,omitempty"`
}

type Ledger struct {
	Entries map[string]*Entry `json:"entries"`
}

type InboxItem struct {
	ID     string
	Weight float64
}

type Escalator interface {
	OpenItemForPattern(pattern string) (item InboxItem, ok bool)
	Bump(id string, newWeight float64) error
}

type Autofiler interface {
	Autofile(pattern string, count int) error
}

type EscalationPolicy struct {
	Threshold int
	Step      float64
	Cap       float64
}

func DefaultEscalationPolicy() EscalationPolicy {
	return EscalationPolicy{Threshold: 2, Step: 0.03, Cap: 0.99}
}

func (p EscalationPolicy) Target(base float64, count int) float64 {
	if count < 1 {
		count = 1
	}
	w := base + p.Step*float64(count-1)
	if w > p.Cap {
		return p.Cap
	}
	return w
}

func NewLedger() *Ledger {
	return &Ledger{Entries: map[string]*Entry{}}
}

func (l *Ledger) Count(pattern string) int {
	if l == nil {
		return 0
	}
	if e, ok := l.Entries[pattern]; ok {
		return e.Count
	}
	return 0
}

func (l *Ledger) Patterns() []Entry {
	out := make([]Entry, 0, len(l.Entries))
	for _, e := range l.Entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Pattern < out[j].Pattern
	})
	return out
}

func (l *Ledger) ItemCounts() map[string]int {
	counts := map[string]int{}
	if l == nil {
		return counts
	}
	for pattern, e := range l.Entries {
		if e == nil || e.Generic {
			continue
		}
		for _, id := range []string{pattern, e.FixItemID} {
			if id != "" {
				counts[id] = max(counts[id], e.Count)
			}
		}
	}
	return counts
}

func (l *Ledger) RecordClosure(pattern string, cycle int, esc Escalator, af Autofiler, pol EscalationPolicy) error {
	if l.Entries == nil {
		l.Entries = map[string]*Entry{}
	}
	e := l.Entries[pattern]
	if e == nil {
		e = &Entry{Pattern: pattern}
		l.Entries[pattern] = e
	}
	if !containsInt(e.Cycles, cycle) {
		e.Cycles = append(e.Cycles, cycle)
	}
	e.Count = len(e.Cycles)
	if cycle > e.LastSeen {
		e.LastSeen = cycle
	}

	if e.Count < pol.Threshold || e.BumpedCycle == cycle {
		return nil
	}
	e.BumpedCycle = cycle

	if esc != nil {
		if item, ok := esc.OpenItemForPattern(pattern); ok {
			e.FixItemID = item.ID
			return esc.Bump(item.ID, pol.Target(item.Weight, e.Count))
		}
	}
	if af != nil && !e.Autofiled {
		if err := af.Autofile(pattern, e.Count); err != nil {
			return err
		}
		e.Autofiled = true
	}
	return nil
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func Load(path string) (*Ledger, error) {
	var l *Ledger
	err := flock.WithPathLock(path, func() error {
		var err error
		l, err = ReadSnapshot(path)
		return err
	})
	return l, err
}

func ReadSnapshot(path string) (*Ledger, error) {
	l := NewLedger()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) || err == nil && len(data) == 0 {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, l); err != nil {
		return nil, fmt.Errorf("recurrence: parse %s: %w", path, err)
	}
	if l.Entries == nil {
		l.Entries = map[string]*Entry{}
	}
	return l, nil
}

func (l *Ledger) Save(path string) error {
	return flock.WithPathLock(path, func() error {
		return atomicwrite.JSON(path, l)
	})
}
