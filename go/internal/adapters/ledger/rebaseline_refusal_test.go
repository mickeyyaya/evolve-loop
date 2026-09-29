package ledger

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func chainOf(t *testing.T, n int) (string, *FileLedger) {
	t.Helper()
	dir := t.TempDir()
	l := New(dir)
	for i := 0; i < n; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{TS: "2026-09-01T00:00:00Z", Role: "orchestrator", Kind: "phase", Cycle: i}); err != nil {
			t.Fatalf("fixture Append %d: %v", i, err)
		}
	}
	return dir, l
}

func fileBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(d.Name(), ".lock") {
			return err
		}
		b, err := os.ReadFile(path)
		out[path] = string(b)
		return err
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return out
}

func TestRebaseline_RefusesWriteWhenForwardWalkWouldFail(t *testing.T) {
	cases := []struct {
		name  string
		cause error
		build func(t *testing.T) (string, *FileLedger)
	}{
		{"stale_sidecar_anchor", core.ErrLedgerChainBroken, func(t *testing.T) (string, *FileLedger) {
			dir, l := chainOf(t, 3)
			if err := l.AnchorLine(context.Background(), 1, "", "anchor entry_seq 1"); err != nil {
				t.Fatalf("fixture AnchorLine: %v", err)
			}
			raw, err := os.ReadFile(l.ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := splitLines(raw)
			lines[1] = []byte(strings.Replace(string(lines[1]), `"kind":"phase"`, `"kind":"phase-rewritten"`, 1))
			if err := l.rewriteLive(lines); err != nil {
				t.Fatal(err)
			}
			return dir, l
		}},
		{"unanchored_segment", ErrSealResidue, func(t *testing.T) (string, *FileLedger) {
			dir, l := chainOf(t, 4)
			raw, err := os.ReadFile(l.ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := splitLines(raw)
			segDir := filepath.Join(dir, segmentsDirName)
			if err := os.MkdirAll(segDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := writeSegment(filepath.Join(segDir, "seg-0001.jsonl.gz"), raw[:prefixLen(raw, 2)]); err != nil {
				t.Fatal(err)
			}
			if err := l.rewriteLive(lines[2:]); err != nil {
				t.Fatal(err)
			}
			return dir, l
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, l := tc.build(t)
			if err := l.VerifyDeep(context.Background()); !errors.Is(err, tc.cause) {
				t.Fatalf("fixture precondition: VerifyDeep = %v, want %v", err, tc.cause)
			}
			before := fileBytes(t, dir)
			err := l.Rebaseline(context.Background(), rebaselineNote)
			if !errors.Is(err, tc.cause) {
				t.Fatalf("Rebaseline = %v, want a refusal carrying %v", err, tc.cause)
			}
			assertSameFiles(t, before, fileBytes(t, dir))
		})
	}
}

func TestRebaseline_RefusesWithoutWritingOnEmptyLiveTailOrUnmarshalableSeal(t *testing.T) {
	t.Run("sealed_segments_with_empty_live_tail", func(t *testing.T) {
		dir, l := chainOf(t, 3)
		raw, err := os.ReadFile(l.ledgerPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, segmentsDirName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeSegment(filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz"), raw); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(l.ledgerPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		before := fileBytes(t, dir)
		err = l.Rebaseline(context.Background(), rebaselineNote)
		if err == nil || !strings.Contains(err.Error(), "no physical tail") {
			t.Fatalf("Rebaseline = %v, want the empty-live-tail refusal", err)
		}
		assertSameFiles(t, before, fileBytes(t, dir))
	})
	t.Run("seal_marshal_failure", func(t *testing.T) {
		dir, l := chainOf(t, 2)
		before := fileBytes(t, dir)
		boom := errors.New("marshal boom")
		var err error
		withHooks(ledgerHooks{marshal: func(any) ([]byte, error) { return nil, boom }}, func() {
			err = l.Rebaseline(context.Background(), rebaselineNote)
		})
		if !errors.Is(err, boom) {
			t.Fatalf("Rebaseline = %v, want the marshal failure", err)
		}
		assertSameFiles(t, before, fileBytes(t, dir))
	})
}

func assertSameFiles(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(after) != len(before) {
		t.Errorf("refused rebaseline changed the file set: %d -> %d files", len(before), len(after))
	}
	for path, was := range before {
		if after[path] != was {
			t.Errorf("refused rebaseline rewrote %s (%d -> %d bytes)", filepath.Base(path), len(was), len(after[path]))
		}
	}
}
