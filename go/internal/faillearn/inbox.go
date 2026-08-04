package faillearn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// inbox.go — the remediation half of the failure floor
// (batch-integrity-review-2026-08-04.md F1(ii)).
//
// Cycle-1255's retrospective filed two remediation items that existed ONLY
// inside retrospective-report.md: the loop's own queue never saw them, so the
// defects were laundered away by later continuations. The floor already
// guaranteed the retrospective survives; it guaranteed nothing about the WORK
// the retrospective asks for. WithInbox closes that gap in the same call, so
// "the retro was written" can no longer be true while "the remediation was
// queued" is false.

// InboxItem is one retro-derived remediation todo destined for `.evolve/inbox`.
//
// The JSON tags mirror inboxbatch.Item's wire shape (id/title/weight/kind/
// priority/files/injected_by) because inboxbatch is the CONSUMER and binds by
// tag. faillearn is a leaf package (stdlib + yaml.v3), so it cannot import
// inboxbatch to borrow the type — parity is by tag and is asserted on the raw
// JSON keys in inbox_transactional_test.go. Renaming a tag here silently
// produces items the loader drops (the cycle-1190 Class-field shape).
//
// InjectedBy is deliberately non-empty for every item this package writes:
// inboxbatch.ConsoleRouted treats an empty InjectedBy as operator-authored and
// honors a route:"lane" override from it. An autofiled item must never inherit
// that authority.
type InboxItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Weight     float64  `json:"weight"`
	Kind       string   `json:"kind"`
	Priority   string   `json:"priority"`
	Files      []string `json:"files"`
	InjectedBy string   `json:"injected_by"`
}

// Option configures a WriteArtifacts call. Functional options keep the three
// existing option-free call sites (core/failure_learning.go, core/reset.go,
// cmd/evolve/cmd_loop_outcome.go) byte-identical.
type Option func(*writeConfig)

// writeConfig is the resolved option set for one WriteArtifacts call.
type writeConfig struct {
	inboxDir   string
	inboxItems []InboxItem
}

// WithInbox routes the retrospective's remediation items into dir as one
// `<id>.json` per item, written in the SAME call as the retrospective and the
// lesson. A nil/empty item list is legal and mints nothing (not even dir) — a
// retro with no remediation must not leave an empty directory of noise.
func WithInbox(dir string, items []InboxItem) Option {
	return func(c *writeConfig) {
		c.inboxDir = dir
		c.inboxItems = items
	}
}

// writeInboxItems persists each remediation item as an addressable inbox file.
// An item with no id is rejected loudly rather than written under a synthesized
// name: an unaddressable remediation item is the exact erasure this closes.
func (c writeConfig) writeInboxItems() error {
	if c.inboxDir == "" || len(c.inboxItems) == 0 {
		return nil
	}
	for _, it := range c.inboxItems {
		if strings.TrimSpace(it.ID) == "" {
			return fmt.Errorf("faillearn: inbox item %q has no id — an unaddressable remediation item cannot be reconciled later", it.Title)
		}
		// The id is concatenated into a path below, so it must be a BARE
		// filename. Rejection, not sanitisation: a silently rewritten id
		// produces an item nobody can address by the id they filed it under,
		// which is the same erasure this package exists to stop. The only
		// current caller emits [a-z0-9-] slugs, so this is a guard on a newly
		// exported API — the next caller is the one that would fall in.
		if it.ID != filepath.Base(it.ID) || it.ID == "." || it.ID == ".." || strings.ContainsRune(it.ID, filepath.Separator) {
			return fmt.Errorf("faillearn: inbox item id %q is not a bare filename — an id that resolves to a path would write the item outside the inbox", it.ID)
		}
		body, err := json.MarshalIndent(it, "", "  ")
		if err != nil {
			return fmt.Errorf("faillearn: encode inbox item %s: %w", it.ID, err)
		}
		path := filepath.Join(c.inboxDir, it.ID+".json")
		skipped, err := writeIfAbsent(path, body)
		if err != nil {
			return fmt.Errorf("faillearn: write inbox item %s: %w", it.ID, err)
		}
		// cycle-1282 DEF-4: ids are deterministic (`retro-<cycle>-<slug>` over
		// agent-authored defect text), so a concurrent fleet lane or stale state
		// from an earlier run of the same cycle number can already hold the
		// filename. writeIfAbsent used to return nil there and WriteArtifacts
		// still reported success — the real remediation item was DROPPED with no
		// error, no diagnostic, no telemetry, reproducing the very 1255 state
		// this package closes. A skip is now only tolerated when the file on
		// disk carries the SAME item: that is an idempotent retry. Different
		// content under our id is an id collision, and dropping our item to
		// honor theirs is not a decision this floor gets to make silently.
		if !skipped {
			continue
		}
		existing, rerr := os.ReadFile(path)
		if rerr != nil {
			return fmt.Errorf("faillearn: inbox item %s already exists but cannot be read to confirm it matches: %w", it.ID, rerr)
		}
		if !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(body)) {
			return fmt.Errorf("faillearn: inbox item %s already exists at %s with DIFFERENT content — refusing to drop the remediation item %q; resolve the id collision", it.ID, path, it.Title)
		}
	}
	return nil
}
