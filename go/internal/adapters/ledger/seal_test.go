package ledger

// L3.3 seal contract: history is never rewritten — gunzip(segments) +
// live tail is byte-identical to the pre-seal file; VerifyDeep runs the
// SAME chain walk as Verify plus per-segment anchor binding; tampering
// with a segment fails; interrupted seals are detected and resumable.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

func seedLedger(t *testing.T, n int) (*FileLedger, string) {
	t.Helper()
	dir := t.TempDir()
	l := New(dir)
	for i := 0; i < n; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{
			Cycle: i, Role: "builder", Kind: "phase_complete",
			Message: fmt.Sprintf("entry-%d", i),
		}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	return l, dir
}

func TestSeal_ChainStaysVerifiableEndToEnd(t *testing.T) {
	l, dir := seedLedger(t, 20)
	pre, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Seal(context.Background(), 5); err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Live file shrank to the tail + the anchor entry.
	liveRaw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	live := splitLines(liveRaw)
	if len(live) != 6 { // 5 kept + 1 segment_seal anchor
		t.Fatalf("live tail = %d lines, want 6 (5 kept + anchor)", len(live))
	}

	// Plain Verify (live only) still passes: the first kept line chains
	// from the sealed prefix, which plain Verify cannot see — it treats the
	// pre-boundary hash like the soft v8.37 boundary. Deep verify covers
	// the full chain.
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep after seal: %v", err)
	}

	// History preserved byte-identically: gunzip(segment) + live-tail
	// (minus the appended anchor) == pre-seal bytes.
	segs, err := segmentFiles(filepath.Join(dir, segmentsDirName))
	if err != nil || len(segs) != 1 {
		t.Fatalf("want exactly 1 segment, got %v (err=%v)", segs, err)
	}
	segLines, _, err := readSegment(segs[0])
	if err != nil {
		t.Fatal(err)
	}
	var rebuilt bytes.Buffer
	for _, ln := range segLines {
		rebuilt.Write(ln)
		rebuilt.WriteByte('\n')
	}
	for _, ln := range live[:len(live)-1] { // drop the anchor (post-seal entry)
		rebuilt.Write(ln)
		rebuilt.WriteByte('\n')
	}
	if !bytes.Equal(rebuilt.Bytes(), pre) {
		t.Error("gunzip(segment)+tail must be byte-identical to the pre-seal ledger (history rewritten!)")
	}

	// And the ledger still accepts appends afterwards.
	if err := l.Append(context.Background(), core.LedgerEntry{Role: "auditor", Kind: "phase_complete"}); err != nil {
		t.Fatalf("Append after seal: %v", err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep after post-seal append: %v", err)
	}
}

func TestSeal_SecondSealAppendsSecondSegment(t *testing.T) {
	l, dir := seedLedger(t, 12)
	if err := l.Seal(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{Cycle: 100 + i, Role: "builder", Kind: "k"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	segs, err := segmentFiles(filepath.Join(dir, segmentsDirName))
	if err != nil || len(segs) != 2 {
		t.Fatalf("want 2 segments, got %v (err=%v)", segs, err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep with 2 segments: %v", err)
	}
}

func TestVerifyDeep_TamperedSegmentFails(t *testing.T) {
	l, dir := seedLedger(t, 10)
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	segs, _ := segmentFiles(filepath.Join(dir, segmentsDirName))
	segLines, _, err := readSegment(segs[0])
	if err != nil {
		t.Fatal(err)
	}
	// Rewrite the segment with one byte of one line flipped.
	tampered := bytes.Join(segLines, []byte("\n"))
	tampered = append(tampered, '\n')
	tampered = bytes.Replace(tampered, []byte(`"entry-0"`), []byte(`"entry-X"`), 1)
	if err := writeSegment(segs[0], tampered); err != nil {
		t.Fatal(err)
	}
	err = l.VerifyDeep(context.Background())
	if err == nil {
		t.Fatal("tampered segment must fail VerifyDeep")
	}
	if !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("want ErrLedgerChainBroken, got: %v", err)
	}
}

func TestVerifyDeep_NoSegmentsEqualsVerify(t *testing.T) {
	l, _ := seedLedger(t, 5)
	if err := l.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep with no segments must behave like Verify: %v", err)
	}
}

func TestSeal_NothingToSealIsNoOp(t *testing.T) {
	l, dir := seedLedger(t, 3)
	if err := l.Seal(context.Background(), 5); err != nil {
		t.Fatalf("Seal with tail>len must no-op: %v", err)
	}
	if segs, _ := segmentFiles(filepath.Join(dir, segmentsDirName)); len(segs) != 0 {
		t.Errorf("no segment may be created: %v", segs)
	}
}

func TestWriteSegment(t *testing.T) {
	t.Run("happy path writes gzip atomically", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "seg-0001.jsonl.gz")
		raw := []byte("alpha\nbeta\n")
		if err := writeSegment(path, raw); err != nil {
			t.Fatalf("writeSegment: %v", err)
		}
		lines, sha, err := readSegment(path)
		if err != nil {
			t.Fatalf("readSegment: %v", err)
		}
		if !linesEqual(lines, [][]byte{[]byte("alpha"), []byte("beta")}) {
			t.Fatalf("segment lines = %q", lines)
		}
		if sha != sha256Hex(raw) {
			t.Fatalf("segment sha = %s, want %s", sha, sha256Hex(raw))
		}
	})

	t.Run("mkdir failure removes no temp file", func(t *testing.T) {
		dir := t.TempDir()
		parentFile := filepath.Join(dir, "not-a-dir")
		if err := os.WriteFile(parentFile, []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := writeSegment(filepath.Join(parentFile, "seg-0001.jsonl.gz"), []byte("x\n"))
		if err == nil {
			t.Fatal("writeSegment with file parent succeeded, want error")
		}
	})

	t.Run("rename failure cleans temp file", func(t *testing.T) {
		dir := t.TempDir()
		targetDir := filepath.Join(dir, "seg-0001.jsonl.gz")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		err := writeSegment(targetDir, []byte("x\n"))
		if err == nil {
			t.Fatal("writeSegment over directory succeeded, want error")
		}
		matches, globErr := filepath.Glob(filepath.Join(dir, "seg-0001.jsonl.gz.*.tmp"))
		if globErr != nil {
			t.Fatal(globErr)
		}
		if len(matches) != 0 {
			t.Fatalf("temporary files leaked after failed rename: %v", matches)
		}
	})
}

func TestReadSegmentRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seg-0001.jsonl.gz")
	raw := []byte("one\ntwo\nthree\n")
	if err := writeSegment(path, raw); err != nil {
		t.Fatalf("writeSegment: %v", err)
	}
	lines, sha, err := readSegment(path)
	if err != nil {
		t.Fatalf("readSegment: %v", err)
	}
	if !linesEqual(lines, [][]byte{[]byte("one"), []byte("two"), []byte("three")}) {
		t.Fatalf("roundtrip lines = %q", lines)
	}
	if sha != sha256Hex(raw) {
		t.Fatalf("roundtrip sha = %s, want %s", sha, sha256Hex(raw))
	}
}

func TestReadSegmentRejectsCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seg-0001.jsonl.gz")
	if err := writeSegment(path, []byte("one\ntwo\n")); err != nil {
		t.Fatalf("writeSegment: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw[:len(raw)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readSegment(path); err == nil {
		t.Fatal("readSegment accepted truncated gzip, want error")
	}
}

func TestReadSegmentErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if _, _, err := readSegment(filepath.Join(t.TempDir(), "missing.jsonl.gz")); err == nil {
			t.Fatal("readSegment missing file succeeded, want error")
		}
	})

	t.Run("invalid gzip header", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "seg-0001.jsonl.gz")
		if err := os.WriteFile(path, []byte("not gzip"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readSegment(path); err == nil {
			t.Fatal("readSegment invalid gzip succeeded, want error")
		}
	})
}

func TestRewriteLive(t *testing.T) {
	t.Run("replaces live file byte for byte", func(t *testing.T) {
		dir := t.TempDir()
		l := New(dir)
		oldPath := filepath.Join(dir, "ledger.jsonl")
		if err := os.WriteFile(oldPath, []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		lines := [][]byte{[]byte(`{"entry":1}`), []byte(`{"entry":2}`)}
		if err := l.rewriteLive(lines); err != nil {
			t.Fatalf("rewriteLive: %v", err)
		}
		got, err := os.ReadFile(oldPath)
		if err != nil {
			t.Fatal(err)
		}
		want := []byte("{\"entry\":1}\n{\"entry\":2}\n")
		if !bytes.Equal(got, want) {
			t.Fatalf("live bytes = %q, want %q", got, want)
		}
	})

	t.Run("temp creation error is reported", func(t *testing.T) {
		dir := t.TempDir()
		parentFile := filepath.Join(dir, "not-a-dir")
		if err := os.WriteFile(parentFile, []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}
		l := &FileLedger{ledgerPath: filepath.Join(parentFile, "ledger.jsonl")}
		if err := l.rewriteLive([][]byte{[]byte("x")}); err == nil {
			t.Fatal("rewriteLive with file parent succeeded, want error")
		}
	})

	t.Run("rename error cleans temp file", func(t *testing.T) {
		dir := t.TempDir()
		l := New(dir)
		if err := os.MkdirAll(filepath.Join(dir, "ledger.jsonl"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := l.rewriteLive([][]byte{[]byte("x")}); err == nil {
			t.Fatal("rewriteLive over directory succeeded, want error")
		}
		matches, err := filepath.Glob(filepath.Join(dir, "ledger.jsonl.*.tmp"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("temporary files leaked after failed rename: %v", matches)
		}
	})
}

func TestLinesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    [][]byte
		b    [][]byte
		want bool
	}{
		{name: "equal", a: [][]byte{[]byte("a"), []byte("b")}, b: [][]byte{[]byte("a"), []byte("b")}, want: true},
		{name: "unequal length", a: [][]byte{[]byte("a")}, b: [][]byte{[]byte("a"), []byte("b")}, want: false},
		{name: "unequal bytes", a: [][]byte{[]byte("a"), []byte("b")}, b: [][]byte{[]byte("a"), []byte("c")}, want: false},
		{name: "empty", a: nil, b: nil, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := linesEqual(tc.a, tc.b); got != tc.want {
				t.Fatalf("linesEqual() = %v, want %v", got, tc.want)
			}
			if got := linesEqual(tc.b, tc.a); got != tc.want {
				t.Fatalf("linesEqual() reverse = %v, want %v", got, tc.want)
			}
		})
	}
}

// Interrupted-seal recovery, case A: segment written, live file never
// truncated. VerifyDeep names the residue; a re-run Seal completes the
// truncation and the chain deep-verifies again.
func TestSeal_ResumeAfterCrashBeforeTruncate(t *testing.T) {
	l, dir := seedLedger(t, 10)
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(raw)
	// Simulate the crash: segment exists for lines 0..6, live file untouched.
	segPath := filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz")
	if err := writeSegment(segPath, raw[:prefixLen(raw, 7)]); err != nil {
		t.Fatal(err)
	}
	if err := l.VerifyDeep(context.Background()); !errors.Is(err, ErrSealResidue) {
		t.Fatalf("want ErrSealResidue before recovery, got: %v", err)
	}
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatalf("resume Seal: %v", err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep after resume: %v", err)
	}
	liveRaw, _ := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if got := len(splitLines(liveRaw)); got != len(lines)-7+1 { // tail + anchor
		t.Errorf("resume must complete the original truncation (7 sealed): live=%d", got)
	}
}

// Interrupted-seal recovery, case B: truncated but the anchor entry never
// landed. VerifyDeep names the missing anchor; re-run Seal appends it.
func TestSeal_ResumeAfterCrashBeforeAnchor(t *testing.T) {
	l, dir := seedLedger(t, 10)
	raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	cut := prefixLen(raw, 7)
	segPath := filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz")
	if err := writeSegment(segPath, raw[:cut]); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), raw[cut:], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := l.VerifyDeep(context.Background()); !errors.Is(err, ErrSealResidue) {
		t.Fatalf("want ErrSealResidue (missing anchor), got: %v", err)
	}
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatalf("resume Seal (anchor only): %v", err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("VerifyDeep after anchor resume: %v", err)
	}
}

// Acceptance (plan L3.3, adjusted to reality): the plan asked for "deep
// verify green on a copy of the real ledger before/after seal" — but the
// REAL ledger has genuine pre-hardening damage (line 1740, 2026-05-26:
// entry 1740 chains from a hash matching nothing — its predecessor's
// bytes were rewritten post-hoc; the same line the Iter regression test
// memorializes). Blessing that class would gut the verifier, so the
// honest chain-preservation property is VERDICT preservation: sealing
// never changes what verification says — green stays green (covered by
// the synthetic tests above), and a broken ledger stays broken at the
// SAME line with the SAME hashes. Skips when the real ledger is not
// reachable (CI sandboxes) — same convention as TestIter_RealLedger.
func TestSeal_RealLedgerCopy(t *testing.T) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", ".evolve"),
		filepath.Join("..", "..", "..", "..", "..", ".evolve"),
		// Interactive kernel worktrees live at <root>/.claude/worktrees/<name>,
		// three levels below the main checkout that owns the real ledger.
		filepath.Join("..", "..", "..", "..", "..", "..", "..", ".evolve"),
	}
	var src string
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "ledger.jsonl")); err == nil {
			src = abs
			break
		}
	}
	if src == "" {
		t.Skip("real .evolve/ledger.jsonl not reachable from this test cwd")
	}
	dir := t.TempDir()
	for _, f := range []string{"ledger.jsonl", "ledger.tip"} {
		raw, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			t.Skipf("cannot copy real %s: %v", f, err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	l := New(dir)
	pre := l.VerifyDeep(context.Background())
	if err := l.Seal(context.Background(), 100); err != nil {
		t.Fatalf("real-copy Seal: %v", err)
	}
	post := l.VerifyDeep(context.Background())
	switch {
	case pre == nil && post != nil:
		t.Fatalf("seal BROKE a green ledger: %v", post)
	case pre != nil && post == nil:
		t.Fatalf("seal MASKED damage: pre-seal verdict was %v", pre)
	case pre != nil && post != nil && pre.Error() != post.Error():
		t.Fatalf("seal changed the verdict:\n pre  %v\n post %v", pre, post)
	}
	liveRaw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(splitLines(liveRaw)); got != 101 { // 100 kept + anchor
		t.Errorf("real-copy live tail = %d lines, want 101", got)
	}
}
