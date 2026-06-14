package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// updatestate.go — CA.3 (concurrency-factory plan, Track C-A): the
// serialized lossless read-modify-write for state.json. The plain
// ReadState→mutate→WriteState sequence has two failure modes this fixes:
//
//  1. Lost updates: two mutators (goroutines or PROCESSES — the 278/279
//     two-session launch race) interleave their reads and the loser's
//     write erases the winner's. UpdateState holds a blocking flock on
//     state.json.lock across the whole RMW, so writes serialize; the
//     stateRevision counter (++ per write) is the audit trail — a gap
//     or repeat means some writer bypassed UpdateState.
//  2. Subset clobber: core.State models only the orchestrator-load-bearing
//     keys, so WriteState drops operator keys (expected_ship_sha …).
//     UpdateState merges the mutated typed view over the raw JSON map —
//     unmodeled keys survive verbatim; modeled keys the mutation cleared
//     are deleted, not resurrected.
//
// Single-mode byte-stability: stateRevision is omitempty and everything
// else round-trips — states never touched by UpdateState are byte-stable.

// UpdateState locks, reads, applies mutate, bumps StateRevision, and writes
// back losslessly. Returns the post-mutation state. The mutate func must be
// fast and side-effect-free — it runs under the cross-process lock — and
// must NOT call UpdateState (the blocking flock would deadlock the
// re-entrant call). A panicking mutate still releases the lock (deferred).
func (s *FilesystemStorage) UpdateState(_ context.Context, mutate func(*core.State)) (core.State, error) {
	path := filepath.Join(s.evolveDir, "state.json")
	release, err := flock.PathLock(path) // CA.3: "<state.json>.lock" sidecar
	if err != nil {
		return core.State{}, fmt.Errorf("update state: %w", err)
	}
	defer release()

	obj := map[string]json.RawMessage{}
	var st core.State
	raw, err := os.ReadFile(path)
	switch {
	case err == nil && len(raw) > 0:
		if uerr := json.Unmarshal(raw, &obj); uerr != nil {
			return core.State{}, fmt.Errorf("update state: state.json malformed (%w); refusing to clobber", uerr)
		}
		if uerr := json.Unmarshal(raw, &st); uerr != nil {
			return core.State{}, fmt.Errorf("update state: decode: %w", uerr)
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return core.State{}, fmt.Errorf("update state: read: %w", err)
	}

	mutate(&st)
	st.StateRevision++

	typed, err := json.Marshal(st)
	if err != nil {
		return core.State{}, fmt.Errorf("update state: marshal: %w", err)
	}
	typedMap := map[string]json.RawMessage{}
	if err := json.Unmarshal(typed, &typedMap); err != nil {
		return core.State{}, fmt.Errorf("update state: remap: %w", err)
	}
	// Modeled keys are owned by the typed view: delete then overlay, so a
	// key the mutation cleared (omitempty → absent from typedMap) is not
	// resurrected from the raw map. Unmodeled keys pass through untouched.
	for _, k := range modeledKeys {
		delete(obj, k)
	}
	for k, v := range typedMap {
		obj[k] = v
	}
	if err := writeJSONAtomic(path, obj); err != nil {
		return core.State{}, fmt.Errorf("update state: write: %w", err)
	}
	return st, nil
}

// modeledKeys caches the reflection walk — the set is fixed at compile time,
// and UpdateState calls it inside the lock-held section.
var modeledKeys = modeledStateKeys()

// modeledStateKeys derives the JSON keys core.State owns from its struct
// tags — reflection keeps the delete-then-overlay set drift-free as fields
// are added.
func modeledStateKeys() []string {
	t := reflect.TypeOf(core.State{})
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			keys = append(keys, name)
		}
	}
	return keys
}
