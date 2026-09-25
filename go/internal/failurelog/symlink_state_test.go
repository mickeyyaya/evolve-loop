package failurelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestStateWriters_PreserveSymlinkedStatePath is the cycle-1690 pin: every
// failurelog state writer tmp+renames through atomicWriteJSON, and a rename
// over a worktree's state.json link REPLACES the link with a regular file
// (the cycle-999 sever). Each writer must keep the link and land its write on
// the canonical file.
func TestStateWriters_PreserveSymlinkedStatePath(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	const seed = `{
  "lastCycleNumber": 41,
  "failedApproaches": [
    {"cycle": 7, "classification": "infrastructure-transient", "expiresAt": "2026-01-02T00:00:00Z"}
  ],
  "carryoverTodos": [
    {"id": "expired-todo", "expiresAt": "2026-01-02T00:00:00Z"},
    {"id": "legacy-todo"}
  ]
}`
	writers := []struct {
		name string
		run  func(p string) error
		want string // substring the canonical file must carry after the write
	}{
		{"Record", func(p string) error {
			_, err := Record(p, "", RecordRequest{Cycle: 42, Classification: string(OperatorReset), Summary: "symlink pin", Now: now})
			return err
		}, `"cycle": 42`},
		{"PruneExpired", func(p string) error {
			_, err := PruneExpired(p, now)
			return err
		}, `"failedApproaches": []`},
		{"PruneByClassification", func(p string) error {
			_, err := PruneByClassification(p, []Classification{InfrastructureTransient})
			return err
		}, `"failedApproaches": []`},
		{"PruneExpiredCarryoverTodos", func(p string) error {
			_, err := PruneExpiredCarryoverTodos(p, now)
			return err
		}, `"id": "legacy-todo"`},
		{"BackfillLegacyCarryoverExpiry", func(p string) error {
			_, err := BackfillLegacyCarryoverExpiry(p, DefaultCarryoverBackfillTTL, now)
			return err
		}, `"expiresAt": "2026-`},
		{"IncrementCarryoverUnpicked", func(p string) error {
			_, err := IncrementCarryoverUnpicked(p)
			return err
		}, `"cycles_unpicked": 1`},
	}
	for _, w := range writers {
		for _, relative := range []bool{false, true} {
			name := w.name + "/absolute"
			if relative {
				name = w.name + "/relative"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				canonical := mustWrite(t, filepath.Join(root, "canon", "state.json"), seed)
				target := canonical
				if relative {
					target = filepath.Join("..", "canon", "state.json")
				}
				link := filepath.Join(root, "wt", "state.json")
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}

				if err := w.run(link); err != nil {
					t.Fatalf("%s through a linked state.json: %v", w.name, err)
				}

				if got, err := os.Readlink(link); err != nil || got != target {
					t.Fatalf("%s severed the link: Readlink=%q err=%v, want %q (the cycle-999 defect)", w.name, got, err, target)
				}
				if body := mustRead(t, canonical); !strings.Contains(body, w.want) {
					t.Errorf("%s did not land on the canonical file (want %q):\n%s", w.name, w.want, body)
				}
			})
		}
	}
}
