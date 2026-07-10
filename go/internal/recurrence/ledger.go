// Package recurrence is the deterministic recurrence ledger over lesson
// `pattern:` keys plus the automatic inbox weight-escalation policy.
//
// The system already NOTICED every recurrence — retro closeouts wrote
// "6th occurrence, confidence 0.97" chains — but noticing lived in an advisory
// channel with no write access to the priority queue, so the same class of
// defect recurred for dozens of cycles before anything raised its weight. This
// package makes recurrence a first-class, deterministic signal:
//
//	(1) LEDGER      — on retro closeout, upsert {pattern -> cycles[], count,
//	    last_seen, fix_item_id, fix_landed_sha} keyed by the lesson `pattern:`.
//	(2) ESCALATION  — a pure count→weight formula bumps the linked OPEN inbox
//	    item's weight, idempotent per cycle; when no open item exists the
//	    pattern is handed to the retro autofile seam EXACTLY ONCE while open.
//
// Leaf package: stdlib + internal/atomicwrite + internal/adapters/flock only.
// No LLM, no policy import (the caller injects EscalationPolicy so the formula
// constants stay config-driven, not Go literals baked into a consumer).
package recurrence

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// Entry is one pattern's recurrence record. Cycles is the de-duplicated,
// append-ordered list of cycle numbers in which the pattern was closed out;
// Count is len(Cycles). BumpedCycle and Autofiled are dedup guards so a
// re-run of the same cycle's closeout cannot double-escalate.
type Entry struct {
	Pattern      string `json:"pattern"`
	Cycles       []int  `json:"cycles"`
	Count        int    `json:"count"`
	LastSeen     int    `json:"last_seen"`
	FixItemID    string `json:"fix_item_id,omitempty"`
	FixLandedSHA string `json:"fix_landed_sha,omitempty"`
	// BumpedCycle records the last cycle whose closeout escalated this pattern
	// — the per-cycle idempotency guard (re-running one cycle never re-bumps).
	BumpedCycle int `json:"bumped_cycle,omitempty"`
	// Autofiled is true once the pattern has been handed to the autofile seam,
	// so an orphan pattern (no open item) is filed exactly once while open.
	Autofiled bool `json:"autofiled,omitempty"`
	// Generic is true when the pattern is classification-vocabulary noise
	// (denylist or pattern==errorCategory echo), set during backfill via
	// IsGeneric. Consumers exclude generic patterns from escalation and the CLI
	// report so noise never drowns the specific-defect signal.
	Generic bool `json:"generic,omitempty"`
}

// Ledger is the pattern→Entry recurrence map, persisted to
// .evolve/recurrence-ledger.json.
type Ledger struct {
	Entries map[string]*Entry `json:"entries"`
}

// InboxItem is the minimal view of an open inbox item the ledger escalates.
type InboxItem struct {
	ID     string
	Weight float64
}

// Escalator resolves and bumps the open inbox item linked to a recurring
// pattern. Implementations own the inbox I/O; the ledger owns the policy.
type Escalator interface {
	// OpenItemForPattern returns the open inbox item linked to pattern, or
	// ok=false when none is open.
	OpenItemForPattern(pattern string) (item InboxItem, ok bool)
	// Bump persists a new weight for the item identified by id.
	Bump(id string, newWeight float64) error
}

// Autofiler receives a recurring pattern that has no open item to escalate, so
// the recurrence still lands on the priority queue instead of being dropped.
type Autofiler interface {
	Autofile(pattern string, count int) error
}

// EscalationPolicy is the pure count→weight formula, injected by the caller so
// the constants stay config-driven. Target = min(Cap, base + Step*(count-1)).
type EscalationPolicy struct {
	// Threshold is the count at or above which a pattern escalates.
	Threshold int
	// Step is the per-extra-occurrence weight increment.
	Step float64
	// Cap is the maximum weight escalation may reach.
	Cap float64
}

// DefaultEscalationPolicy returns the built-in escalation defaults used when a
// caller supplies no policy override (threshold 2, +0.03 per extra occurrence,
// capped at 0.99). Mirrors the faillearn/policy default-then-override pattern.
func DefaultEscalationPolicy() EscalationPolicy {
	return EscalationPolicy{Threshold: 2, Step: 0.03, Cap: 0.99}
}

// Target computes the escalated weight for a pattern seen count times, starting
// from the item's current base weight: min(Cap, base + Step*(count-1)).
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

// NewLedger returns an empty ledger ready for RecordClosure.
func NewLedger() *Ledger {
	return &Ledger{Entries: map[string]*Entry{}}
}

// Count returns the recorded recurrence count for pattern (0 when unseen).
func (l *Ledger) Count(pattern string) int {
	if l == nil {
		return 0
	}
	if e, ok := l.Entries[pattern]; ok {
		return e.Count
	}
	return 0
}

// Patterns returns every entry sorted by descending count, ties broken by
// pattern name, for a stable CLI report.
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

// RecordClosure upserts a retro closeout for pattern in the given cycle, then
// applies escalation when the pattern has reached the policy threshold:
//   - a linked OPEN inbox item is bumped to policy.Target(item.Weight, count);
//   - otherwise the pattern is handed to the autofile seam exactly once.
//
// Both actions are idempotent per cycle: re-running the same cycle's closeout
// re-appends nothing and re-escalates nothing (BumpedCycle/Autofiled guards).
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
		return nil // below threshold, or this cycle already escalated (idempotent)
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

// Load reads the ledger JSON at path under a shared file lock. A missing file
// yields an empty ledger (first run), not an error.
func Load(path string) (*Ledger, error) {
	l := NewLedger()
	err := flock.WithPathLock(path, func() error {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, l); err != nil {
			return fmt.Errorf("recurrence: parse %s: %w", path, err)
		}
		if l.Entries == nil {
			l.Entries = map[string]*Entry{}
		}
		return nil
	})
	return l, err
}

// Save writes the ledger to path atomically under an exclusive file lock so a
// concurrent retro closeout never observes a torn file.
func (l *Ledger) Save(path string) error {
	return flock.WithPathLock(path, func() error {
		return atomicwrite.JSON(path, l)
	})
}
