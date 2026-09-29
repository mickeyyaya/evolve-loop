//go:build acs

// Package cycle1756 materializes the acceptance criteria of triage top_n task
// rebaseline-dry-run-before-seal (fleet scope: retro-failure-identity, P1):
// FileLedger.Rebaseline must decide whether its seal would verify BEFORE it
// writes, and a refused rebaseline must leave every ledger file byte-identical.
package cycle1756

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const operatorNote = "operator sign-off: damaged prefix accepted"

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func chainedLedger(t *testing.T, n int) (string, *ledger.FileLedger) {
	t.Helper()
	dir := t.TempDir()
	l := ledger.New(dir)
	for i := 0; i < n; i++ {
		e := core.LedgerEntry{TS: fmt.Sprintf("2026-09-01T00:00:%02dZ", i), Role: "orchestrator", Kind: "phase"}
		if err := l.Append(context.Background(), e); err != nil {
			t.Fatalf("fixture Append %d: %v", i, err)
		}
	}
	return dir, l
}

func readLiveLines(t *testing.T, dir string) [][]byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read ledger.jsonl: %v", err)
	}
	return bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
}

func writeLiveLines(t *testing.T, dir string, lines [][]byte) {
	t.Helper()
	var buf bytes.Buffer
	for _, ln := range lines {
		buf.Write(ln)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
}

func appendRawLine(t *testing.T, dir, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "ledger.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open ledger.jsonl: %v", err)
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		t.Fatalf("append raw line: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close ledger.jsonl: %v", err)
	}
}

// snapshot maps every regular file under dir to its bytes. Lock files are
// coordination artifacts with no ledger content, so they are left out.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.Contains(d.Name(), ".lock") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return out
}

func assertByteIdentical(t *testing.T, before, after map[string]string, rerr error) {
	t.Helper()
	for path, was := range before {
		now, ok := after[path]
		switch {
		case !ok:
			t.Errorf("refused rebaseline (err=%v) removed %s", rerr, path)
		case now != was:
			t.Errorf("refused rebaseline (err=%v) rewrote %s: %d -> %d bytes; a refusal must write nothing", rerr, path, len(was), len(now))
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("refused rebaseline (err=%v) created %s; a refusal must write nothing", rerr, path)
		}
	}
}

func assertRefusedWithoutWriting(t *testing.T, dir string, l *ledger.FileLedger, cause error) {
	t.Helper()
	before := snapshot(t, dir)
	rerr := l.Rebaseline(context.Background(), operatorNote)
	if rerr == nil {
		t.Fatal("Rebaseline returned nil on a ledger no seal can make verify")
	}
	if !errors.Is(rerr, cause) {
		t.Errorf("refusal does not carry its cause: errors.Is(%v, %v) = false", rerr, cause)
	}
	assertByteIdentical(t, before, snapshot(t, dir), rerr)
}

func TestC1756_001_StaleAnchorRebaselineRefusesAndWritesNothing(t *testing.T) {
	dir, l := chainedLedger(t, 3)
	ctx := context.Background()
	if err := l.AnchorLine(ctx, 1, "", "operator anchor over entry_seq 1"); err != nil {
		t.Fatalf("fixture AnchorLine: %v", err)
	}
	// The anchored line is rewritten out of band, so the sidecar names a SHA no line carries.
	lines := readLiveLines(t, dir)
	lines[1] = bytes.Replace(lines[1], []byte(`"kind":"phase"`), []byte(`"kind":"phase-migrated"`), 1)
	writeLiveLines(t, dir, lines)

	err := l.VerifyDeep(ctx)
	if err == nil || !strings.Contains(err.Error(), "epoch anchor line not found") {
		t.Fatalf("fixture precondition: want VerifyDeep to fail on the missing epoch anchor, got %v", err)
	}
	assertRefusedWithoutWriting(t, dir, l, core.ErrLedgerChainBroken)
}

func TestC1756_002_UnanchoredSegmentRebaselineRefusesAndWritesNothing(t *testing.T) {
	dir, l := chainedLedger(t, 4)
	lines := readLiveLines(t, dir)
	// The state a Seal leaves when it stops after truncating the live file and before appending the segment anchor.
	var prefix bytes.Buffer
	for _, ln := range lines[:2] {
		prefix.Write(ln)
		prefix.WriteByte('\n')
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(prefix.Bytes()); err != nil {
		t.Fatalf("gzip segment: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	segDir := filepath.Join(dir, "ledger-segments")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatalf("mkdir segments: %v", err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "seg-0001.jsonl.gz"), gz.Bytes(), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}
	writeLiveLines(t, dir, lines[2:])

	if err := l.VerifyDeep(context.Background()); !errors.Is(err, ledger.ErrSealResidue) {
		t.Fatalf("fixture precondition: want VerifyDeep to report seal residue, got %v", err)
	}
	assertRefusedWithoutWriting(t, dir, l, ledger.ErrSealResidue)
}

func damagedPrefixLedger(t *testing.T) (string, *ledger.FileLedger) {
	t.Helper()
	dir := t.TempDir()
	lines := [][]byte{
		[]byte(`{"ts":"2026-09-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0,"prev_hash":"` + ledger.ZeroSeed + `"}`),
		[]byte(`{"ts":"2026-09-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"` + strings.Repeat("a1", 32) + `"}`),
		[]byte(`{"ts":"2026-09-01T00:02:00Z","cycle":1,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"` + strings.Repeat("b2", 32) + `"}`),
	}
	writeLiveLines(t, dir, lines)
	tip := fmt.Sprintf("2:%s", sha256Hex(lines[2]))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write tip: %v", err)
	}
	return dir, ledger.New(dir)
}

const foreignLine = `{"ts":"2026-09-01T00:09:00Z","class":"inbox-lifecycle","action":"promote","task_id":"x","cycle":null,"git_sha":null,"reason":"r"}`

func TestC1756_003_RepairableLedgersStillSealAndVerify(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T) (string, *ledger.FileLedger)
	}{
		{"damaged_prefix", damagedPrefixLedger},
		{"foreign_unchained_tail", func(t *testing.T) (string, *ledger.FileLedger) {
			dir, l := chainedLedger(t, 3)
			appendRawLine(t, dir, foreignLine)
			return dir, l
		}},
		{"break_after_sidecar_anchor", func(t *testing.T) (string, *ledger.FileLedger) {
			dir, l := chainedLedger(t, 3)
			if err := l.AnchorLine(context.Background(), 1, "", "operator anchor over entry_seq 1"); err != nil {
				t.Fatalf("fixture AnchorLine: %v", err)
			}
			appendRawLine(t, dir, `{"ts":"2026-09-01T00:09:00Z","role":"orchestrator","kind":"phase","entry_seq":3,"prev_hash":"`+strings.Repeat("c3", 32)+`"}`)
			return dir, l
		}},
		{"anchored_segment_then_foreign_tail", func(t *testing.T) (string, *ledger.FileLedger) {
			dir, l := chainedLedger(t, 4)
			if err := l.Seal(context.Background(), 1); err != nil {
				t.Fatalf("fixture Seal: %v", err)
			}
			appendRawLine(t, dir, foreignLine)
			return dir, l
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, l := tc.build(t)
			ctx := context.Background()
			if err := l.VerifyDeep(ctx); err == nil {
				t.Fatal("fixture precondition: a ledger that needs a rebaseline verified GREEN")
			}
			before := snapshot(t, dir)
			if err := l.Rebaseline(ctx, operatorNote); err != nil {
				t.Fatalf("Rebaseline refused a ledger one seal repairs: %v", err)
			}
			after := snapshot(t, dir)
			assertOneSealAppended(t, before, after)
			if err := l.VerifyDeep(ctx); err != nil {
				t.Errorf("chain does not verify after the seal: %v", err)
			}
			if err := l.Append(ctx, core.LedgerEntry{TS: "2026-09-01T00:10:00Z", Role: "orchestrator", Kind: "phase"}); err != nil {
				t.Fatalf("Append after seal: %v", err)
			}
			if err := l.VerifyDeep(ctx); err != nil {
				t.Errorf("first append after the seal broke the chain (tip not moved to the seal): %v", err)
			}
		})
	}
}

func assertOneSealAppended(t *testing.T, before, after map[string]string) {
	t.Helper()
	for path, was := range before {
		if path == "ledger.jsonl" || path == "ledger.tip" {
			continue
		}
		if after[path] != was {
			t.Errorf("rebaseline rewrote %s; only ledger.jsonl and ledger.tip may change", path)
		}
	}
	was, now := before["ledger.jsonl"], after["ledger.jsonl"]
	if !strings.HasPrefix(now, was) {
		t.Fatal("rebaseline mutated pre-existing ledger history; the seal must be appended")
	}
	added := strings.TrimSuffix(now[len(was):], "\n")
	if added == "" || strings.Contains(added, "\n") {
		t.Fatalf("rebaseline appended %q; want exactly one seal line", added)
	}
	var seal core.LedgerEntry
	if err := json.Unmarshal([]byte(added), &seal); err != nil {
		t.Fatalf("seal line does not decode: %v", err)
	}
	if seal.Role != "operator" || seal.Kind != ledger.RebaselineKind || seal.Message != operatorNote {
		t.Errorf("seal = role %q kind %q message %q; want operator / %q / the operator note", seal.Role, seal.Kind, seal.Message, ledger.RebaselineKind)
	}
	if want := fmt.Sprintf("%d:%s", seal.EntrySeq, sha256Hex([]byte(added))); after["ledger.tip"] != want {
		t.Errorf("ledger.tip = %q, want %q (the tip must move to the seal)", after["ledger.tip"], want)
	}
}

func TestC1756_004_MalformedTipRebaselineRefusesAndWritesNothing(t *testing.T) {
	dir, l := chainedLedger(t, 2)
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte("not-a-tip"), 0o644); err != nil {
		t.Fatalf("write malformed tip: %v", err)
	}
	before := snapshot(t, dir)
	rerr := l.Rebaseline(context.Background(), operatorNote)
	if rerr == nil {
		t.Fatal("Rebaseline returned nil with a malformed ledger.tip")
	}
	assertByteIdentical(t, before, snapshot(t, dir), rerr)
}

func TestC1756_005_LedgerPackageSuiteIncludingRefusalRegressionPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: "github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger",
		Pattern: ".",
		Names: []string{
			"TestRebaseline_RefusesWriteWhenForwardWalkWouldFail",
			"TestRebaseline_SealsDamagedPrefix",
			"TestRebaseline_PreservesDamagedPrefixBytes",
			"TestRebaseline_RefusesUngatedAndEmptyChain",
			"TestRebaseline_ForeignUnchainedTailLine_SealsAndVerifies",
		},
	})
}
