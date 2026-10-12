package ledger

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestSeal_ALockItCannotTakeIsReportedAndLeavesTheLedgerAloneF2(t *testing.T) {
	t.Parallel()
	for name, blockedLock := range map[string]string{
		"the seal lock":   "ledger.lock.seal",
		"the ledger lock": "ledger.lock",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l, dir := seedLedger(t, 8)
			if err := os.Remove(filepath.Join(dir, "ledger.lock")); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, blockedLock), 0o755); err != nil {
				t.Fatal(err)
			}
			before := fileBytes(t, dir)

			err := l.Seal(context.Background(), 2)

			if err == nil || !strings.Contains(err.Error(), "ledger seal: flock") {
				t.Fatalf("Seal = %v, want a flock failure wrapped as ledger seal", err)
			}
			assertSameFiles(t, before, fileBytes(t, dir))
			if _, statErr := os.Stat(filepath.Join(dir, segmentsDirName)); !os.IsNotExist(statErr) {
				t.Errorf("a refused seal created the segments dir (%v)", statErr)
			}
		})
	}
}

func TestSealAndAnchor_ReturnsNoAnchorWhenTheLedgerLockCannotBeTakenF2(t *testing.T) {
	t.Parallel()
	l, dir := seedLedger(t, 4)
	if err := os.Remove(filepath.Join(dir, "ledger.lock")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "ledger.lock"), 0o755); err != nil {
		t.Fatal(err)
	}

	anchor, err := l.sealAndAnchor(1)

	if anchor != nil || err == nil || !strings.HasPrefix(err.Error(), "ledger seal: flock") {
		t.Errorf("sealAndAnchor = (%+v, %v), want no anchor and the wrapped flock error", anchor, err)
	}
}

func TestReadChain_ReportsALiveLedgerItCannotReadAndTreatsAnAbsentOneAsEmptyF2(t *testing.T) {
	t.Parallel()
	t.Run("live ledger is a directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "ledger.jsonl"), 0o755); err != nil {
			t.Fatal(err)
		}

		segs, live, err := New(dir).readChain()

		if segs != nil || live != nil || err == nil || !strings.Contains(err.Error(), "ledger read:") {
			t.Errorf("readChain = (%v, %v, %v), want the wrapped read error", segs, live, err)
		}
	})
	t.Run("live ledger is absent", func(t *testing.T) {
		t.Parallel()

		segs, live, err := New(t.TempDir()).readChain()

		if err != nil || len(segs) != 0 || len(live) != 0 {
			t.Errorf("readChain = (%v, %v, %v), want an empty chain", segs, live, err)
		}
	})
	t.Run("segments dir is a file", func(t *testing.T) {
		t.Parallel()
		l, dir := ledgerFileOnlyF2(t, rawEntry(t, 0, ZeroSeed, "a"))
		writeFileF2(t, filepath.Join(dir, segmentsDirName), "x")

		segs, live, err := l.readChain()

		if segs != nil || live != nil || err == nil || !strings.Contains(err.Error(), "ledger segments read") {
			t.Errorf("readChain = (%v, %v, %v), want the segments error", segs, live, err)
		}
	})
}

func TestReadChain_ReportsASegmentPathItCannotMakeRelativeF2(t *testing.T) {
	l, dir := seedLedger(t, 6)
	if err := l.Seal(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("rel boom")
	segmentRel = func(string, string) (string, error) { return "", boom }
	t.Cleanup(func() { segmentRel = filepath.Rel })

	segs, live, err := New(dir).readChain()

	if segs != nil || live != nil || !errors.Is(err, boom) || !strings.Contains(err.Error(), "ledger verify: rel:") {
		t.Errorf("readChain = (%v, %v, %v), want the wrapped rel error", segs, live, err)
	}
}

func rewriteSegmentF2(t *testing.T, path string, edit func(raw []byte) []byte) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	var gz bytes.Buffer
	if err := gzipSegment(&gz, edit(raw)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, gz.Bytes(), segmentMode); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyDeep_ASegmentWhoseBytesDifferFromItsAnchorIsAChainBreakEvenWhenItsLinesChainF2(t *testing.T) {
	t.Parallel()
	l, dir := seedLedger(t, 10)
	if err := l.Seal(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if err := l.VerifyDeep(context.Background()); err != nil {
		t.Fatalf("precondition: a sealed ledger verifies: %v", err)
	}
	segPath := filepath.Join(dir, segmentsDirName, "seg-0001.jsonl.gz")
	rewriteSegmentF2(t, segPath, func(raw []byte) []byte { return append(raw, '\n') })

	err := l.VerifyDeep(context.Background())

	if !errors.Is(err, core.ErrLedgerChainBroken) || !strings.Contains(err.Error(), "content does not match its anchor") || !strings.Contains(err.Error(), "ledger-segments/seg-0001.jsonl.gz") {
		t.Errorf("VerifyDeep = %v, want a chain break naming the segment whose content differs from its anchor", err)
	}
}

type failingWriterF2 struct{ allow int }

func (w *failingWriterF2) Write(p []byte) (int, error) {
	if w.allow <= 0 {
		return 0, errors.New("sink boom")
	}
	n := min(w.allow, len(p))
	w.allow -= n
	if n < len(p) {
		return n, errors.New("sink boom")
	}
	return n, nil
}

func TestGzipSegment_ReportsAWriteFailureAndACloseFailureApartF2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		allow int
		want  string
	}{
		{"the sink fails at the header", 0, "ledger seal: gzip write: sink boom"},
		{"the sink fails when the stream is flushed", 10, "ledger seal: gzip close: sink boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := gzipSegment(&failingWriterF2{allow: tc.allow}, []byte("one line\n"))

			if err == nil || err.Error() != tc.want {
				t.Errorf("gzipSegment = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestGzipSegment_RoundTripsAnyPayloadThroughReadSegmentF2(t *testing.T) {
	t.Parallel()
	payloads := []string{"", "a\n", "a\nb\nc\n", strings.Repeat("{\"k\":\"v\"}\n", 5000), "unicode ✓ line\nsecond\n"}
	for _, p := range payloads {
		var gz bytes.Buffer
		if err := gzipSegment(&gz, []byte(p)); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "seg.jsonl.gz")
		if err := os.WriteFile(path, gz.Bytes(), segmentMode); err != nil {
			t.Fatal(err)
		}

		lines, sha, err := readSegment(path)

		if err != nil || sha != sha256Hex([]byte(p)) || string(bytes.Join(lines, []byte("\n"))) != strings.TrimSuffix(p, "\n") {
			t.Errorf("round trip of %d bytes = (%d lines, %s, %v)", len(p), len(lines), sha, err)
		}
	}
}

type brokenBufferF2 struct {
	bytes.Buffer
	failAt int
}

func (b *brokenBufferF2) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.failAt {
		return 0, errors.New("buffer boom")
	}
	return b.Buffer.Write(p)
}

func TestWriteSegmentVia_ReportsAGzipFailureAndWritesNoFileF2(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "seg-0001.jsonl.gz")

	err := writeSegmentVia(&brokenBufferF2{failAt: 0}, path, []byte("line\n"))

	if err == nil || err.Error() != "ledger seal: gzip write: buffer boom" {
		t.Errorf("writeSegmentVia = %v, want the gzip write failure", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("a failed gzip left the segment (%v)", statErr)
	}
}

func TestWriteSegment_FailsWhenTheSegmentCannotBeStoredAndLeavesNoFileF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	writeFileF2(t, blocker, "a file where a dir is needed")

	err := writeSegment(filepath.Join(blocker, "seg-0001.jsonl.gz"), []byte("line\n"))

	if err == nil || !strings.HasPrefix(err.Error(), "ledger seal: ") {
		t.Errorf("writeSegment = %v, want the wrapped store error", err)
	}
}

func rebaselineFixtureF2(t *testing.T) (*FileLedger, string) {
	t.Helper()
	dir := damagedLedger(t)
	return New(dir), dir
}

func TestRebaseline_RefusesBeforeWritingWhenTheChainOrTipCannotBeReadF2(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		build func(t *testing.T) string
		want  string
	}{
		"live ledger is a directory": {func(t *testing.T) string {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "ledger.jsonl"), 0o755); err != nil {
				t.Fatal(err)
			}
			return dir
		}, "ledger rebaseline: ledger read:"},
		"tip is malformed": {func(t *testing.T) string {
			dir := damagedLedger(t)
			writeFileF2(t, filepath.Join(dir, "ledger.tip"), "garbage")
			return dir
		}, "ledger rebaseline: tip malformed"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := tc.build(t)
			before := fileBytes(t, dir)

			err := New(dir).Rebaseline(context.Background(), rebaselineNote)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Rebaseline = %v, want %q", err, tc.want)
			}
			assertSameFiles(t, before, fileBytes(t, dir))
		})
	}
}

func TestRebaseline_AnAppendThatFailsAfterTheDryRunWritesNothingF2(t *testing.T) {
	l, dir := rebaselineFixtureF2(t)
	before := fileBytes(t, dir)
	boom := errors.New("marshal boom on the real append")
	calls := 0
	var err error

	withHooks(ledgerHooks{marshal: func(v any) ([]byte, error) {
		calls++
		if calls > 1 {
			return nil, boom
		}
		return json.Marshal(v)
	}}, func() { err = l.Rebaseline(context.Background(), rebaselineNote) })

	if calls != 2 || !errors.Is(err, boom) || !strings.Contains(err.Error(), "ledger rebaseline: append seal:") {
		t.Fatalf("Rebaseline = %v after %d marshal calls, want the dry run to pass and the append to fail", err, calls)
	}
	assertSameFiles(t, before, fileBytes(t, dir))
}

func TestRebaseline_ASealThatLandsButDoesNotVerifyForwardIsReportedAndKeptF2(t *testing.T) {
	l, dir := rebaselineFixtureF2(t)
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	linesBefore := len(splitLines([]byte(readFileF2(t, ledgerPath))))
	boom := errors.New("read boom after the seal")
	reads := 0
	var err error

	withHooks(ledgerHooks{readF: func(path string) ([]byte, error) {
		if path == ledgerPath {
			reads++
			if reads > 1 {
				return nil, boom
			}
		}
		return os.ReadFile(path)
	}}, func() { err = l.Rebaseline(context.Background(), rebaselineNote) })

	if err == nil || !strings.Contains(err.Error(), "seal appended but the chain still does not verify forward") || !strings.Contains(err.Error(), "read boom after the seal") {
		t.Fatalf("Rebaseline = %v, want the post-append verification failure", err)
	}
	if got := len(splitLines([]byte(readFileF2(t, ledgerPath)))); got != linesBefore+1 {
		t.Errorf("ledger has %d lines, want %d: the appended seal is kept, never rolled back", got, linesBefore+1)
	}
}
