// seal.go — chain-preserving ledger segmentation (L3.3, concurrency
// campaign). The live ledger.jsonl grows forever (6,345 lines / years of
// history at the time of writing); Seal moves the oldest lines VERBATIM
// into a compressed segment so the hot file stays small while the hash
// chain stays verifiable end-to-end:
//
//	<evolveDir>/ledger-segments/seg-0001.jsonl.gz   (gzip of lines 1..N, byte-identical)
//	<evolveDir>/ledger.jsonl                        (lines N+1.. — the live tail)
//	a chained "segment_seal" anchor entry            (ArtifactPath = segment rel path,
//	                                                  ArtifactSHA256 = sha256 of the
//	                                                  UNCOMPRESSED segment bytes)
//
// History is never rewritten: concat(gunzip(segments...), live tail) is
// byte-identical to the pre-seal file, which is exactly what VerifyDeep
// checks (same chain walk as Verify, plus per-segment anchor binding).
//
// DELIBERATE DEVIATION from the plan's ".jsonl.zst": the stdlib has no
// zstd and this repo has no production dependencies — gzip keeps it that
// way at a compression ratio that's ample for JSONL text.
//
// Crash windows (each detectable, each recoverable by re-running Seal):
//   - segment written, live file NOT truncated → VerifyDeep reports seal
//     residue (segment's first line still present live); Seal resumes by
//     completing the truncation (the segment is trusted only after its
//     bytes re-verify against the live prefix).
//   - truncated, anchor entry NOT appended → VerifyDeep reports a missing
//     anchor; Seal resumes by appending it.
package ledger

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// segmentsDirName is the segments directory under the evolve dir.
const segmentsDirName = "ledger-segments"

// SealKind is the Kind of the chained anchor entry.
const SealKind = "segment_seal"

// ErrSealResidue marks an interrupted seal (segment written, live file not
// yet truncated, or anchor not yet appended). Re-running Seal completes it.
var ErrSealResidue = errors.New("ledger seal residue — re-run `evolve ledger seal` to complete")

// Seal moves all but the newest keepTail lines of ledger.jsonl into the
// next ledger-segments/seg-NNNN.jsonl.gz and appends the chained
// segment_seal anchor. No-op (nil) when there is nothing to seal.
// keepTail < 1 is coerced to 1 — the live file always keeps its last line
// so a plain Verify retains a non-empty chain to check against the tip.
func (l *FileLedger) Seal(ctx context.Context, keepTail int) error {
	if keepTail < 1 {
		keepTail = 1
	}
	// One seal at a time, host-wide: a dedicated flock held across segment
	// write + truncation + anchor append. Without it, two concurrent Seals
	// could both anchor the same segment (B's resume path finds A's not-yet-
	// anchored segment while A is between truncation and Append). Lock order
	// is seal.lock → ledger.lock and Append only ever takes ledger.lock, so
	// there is no inversion.
	sealRelease, err := flock.Lock(l.lockPath + ".seal")
	if err != nil {
		return fmt.Errorf("ledger seal: %w", err)
	}
	defer sealRelease()

	anchor, err := func() (*core.LedgerEntry, error) {
		l.mu.Lock()
		defer l.mu.Unlock()
		release, err := flock.Lock(l.lockPath)
		if err != nil {
			return nil, fmt.Errorf("ledger seal: %w", err)
		}
		defer release()
		return l.sealLocked(keepTail)
	}()
	if err != nil || anchor == nil {
		return err
	}
	// The anchor goes through the normal Append (own lock acquisition, the
	// inner locks above are released) so it is chained like every other
	// entry; the seal flock still guards against a sibling Seal.
	if err := l.Append(ctx, *anchor); err != nil {
		return fmt.Errorf("ledger seal: anchor append: %w", err)
	}
	return nil
}

// sealLocked does the segment write + truncation under the caller-held
// locks and returns the anchor entry to append (nil = nothing to do).
func (l *FileLedger) sealLocked(keepTail int) (*core.LedgerEntry, error) {
	evolveDir := filepath.Dir(l.ledgerPath)
	segDir := filepath.Join(evolveDir, segmentsDirName)

	raw, err := os.ReadFile(l.ledgerPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("ledger seal: read: %w", err)
	}
	lines := splitLines(raw)

	// Resume case A: the newest segment's first line is still the live
	// file's first line → a prior seal crashed before truncation. Verify
	// the whole segment matches the live prefix, then truncate.
	segs, err := segmentFiles(segDir)
	if err != nil {
		return nil, err
	}
	if len(segs) > 0 {
		segLines, segSHA, err := readSegment(segs[len(segs)-1])
		if err != nil {
			return nil, err
		}
		if len(segLines) > 0 && len(lines) > 0 && bytes.Equal(segLines[0], lines[0]) {
			if len(lines) < len(segLines) || !linesEqual(segLines, lines[:len(segLines)]) {
				return nil, fmt.Errorf("ledger seal: residue segment %s does not match the live prefix — manual inspection required", segs[len(segs)-1])
			}
			if err := l.rewriteLive(lines[len(segLines):]); err != nil {
				return nil, err
			}
			return l.anchorFor(segs[len(segs)-1], segSHA, len(segLines))
		}
		// Resume case B: segment fully sealed but its anchor never landed.
		if missing, segSHA, n, err := l.unanchoredSegment(segs, lines); err != nil {
			return nil, err
		} else if missing != "" {
			return l.anchorFor(missing, segSHA, n)
		}
	}

	if len(lines) <= keepTail {
		return nil, nil // nothing to seal
	}
	n := len(lines) - keepTail
	prefix := raw[:prefixLen(raw, n)]

	next := len(segs) + 1
	segPath := filepath.Join(segDir, fmt.Sprintf("seg-%04d.jsonl.gz", next))
	if err := writeSegment(segPath, prefix); err != nil {
		return nil, err
	}
	if err := l.rewriteLive(lines[n:]); err != nil {
		return nil, err
	}
	return l.anchorFor(segPath, sha256Hex(prefix), n)
}

// anchorFor builds the chained segment_seal anchor entry. The artifact
// path is stored relative to the evolve dir so the ledger stays portable
// across checkouts.
func (l *FileLedger) anchorFor(segPath, uncompressedSHA string, lineCount int) (*core.LedgerEntry, error) {
	rel, err := filepath.Rel(filepath.Dir(l.ledgerPath), segPath)
	if err != nil {
		return nil, fmt.Errorf("ledger seal: rel: %w", err)
	}
	return &core.LedgerEntry{
		Role:           "operator",
		Kind:           SealKind,
		ArtifactPath:   filepath.ToSlash(rel),
		ArtifactSHA256: uncompressedSHA,
		Message:        fmt.Sprintf("sealed %d lines into %s (uncompressed sha256 anchored)", lineCount, filepath.Base(segPath)),
	}, nil
}

// unanchoredSegment reports the newest segment that has no segment_seal
// anchor in (segments + live) — the truncated-but-unanchored crash case.
func (l *FileLedger) unanchoredSegment(segs []string, liveLines [][]byte) (path, sha string, lineCount int, err error) {
	type segData struct {
		lines [][]byte
		sha   string
	}
	evolveDir := filepath.Dir(l.ledgerPath)
	anchored := map[string]bool{}
	cache := make([]segData, len(segs))
	for i, s := range segs {
		lines, segSHA, rerr := readSegment(s)
		if rerr != nil {
			return "", "", 0, rerr
		}
		cache[i] = segData{lines, segSHA}
		for _, line := range lines {
			var e core.LedgerEntry
			if json.Unmarshal(line, &e) == nil && e.Kind == SealKind {
				anchored[e.ArtifactPath] = true
			}
		}
	}
	for _, line := range liveLines {
		var e core.LedgerEntry
		if json.Unmarshal(line, &e) == nil && e.Kind == SealKind {
			anchored[e.ArtifactPath] = true
		}
	}
	for i, s := range segs {
		rel, rerr := filepath.Rel(evolveDir, s)
		if rerr != nil {
			return "", "", 0, fmt.Errorf("ledger seal: rel: %w", rerr)
		}
		if !anchored[filepath.ToSlash(rel)] {
			d := cache[i]
			return s, d.sha, len(d.lines), nil
		}
	}
	return "", "", 0, nil
}

// VerifyDeep reconstructs the full history — gunzip(segments, in order) +
// live tail — and runs the SAME chain walk Verify uses, plus:
//   - every segment must re-hash to its chained segment_seal anchor;
//   - seal residue (segment whose first line is still live) is an error
//     naming the recovery (re-run Seal).
//
// gatherAllLines returns every ledger line in chain order: each sealed segment
// (oldest first) followed by the live tail. Anchor uses it to locate a line by
// entry_seq across SEALED history — the ledger-1740 damage is old enough to have
// been sealed, so reading only the live ledger.jsonl would miss it. VerifyDeep
// does the same reconstruction inline because it additionally needs the
// per-segment residue check and segment-SHA anchor binding; this is the plain
// "all lines in order" projection without those verify-only concerns.
func (l *FileLedger) gatherAllLines() ([][]byte, error) {
	evolveDir := filepath.Dir(l.ledgerPath)
	segs, err := segmentFiles(filepath.Join(evolveDir, segmentsDirName))
	if err != nil {
		return nil, err
	}
	var full [][]byte
	for _, s := range segs {
		segLines, _, err := readSegment(s)
		if err != nil {
			return nil, err
		}
		full = append(full, segLines...)
	}
	liveRaw, err := os.ReadFile(l.ledgerPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("ledger read: %w", err)
	}
	full = append(full, splitLines(liveRaw)...)
	return full, nil
}

func (l *FileLedger) VerifyDeep(_ context.Context) error {
	evolveDir := filepath.Dir(l.ledgerPath)
	segs, err := segmentFiles(filepath.Join(evolveDir, segmentsDirName))
	if err != nil {
		return err
	}
	liveRaw, err := os.ReadFile(l.ledgerPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ledger read: %w", err)
	}
	liveLines := splitLines(liveRaw)

	var full [][]byte
	segSHAs := map[string]string{} // rel path → uncompressed sha
	for _, s := range segs {
		segLines, segSHA, err := readSegment(s)
		if err != nil {
			return err
		}
		// Residue check on EVERY segment (not just the newest): a segment
		// whose first line is still the live file's first line means a seal
		// truncation never completed.
		if len(segLines) > 0 && len(liveLines) > 0 && bytes.Equal(segLines[0], liveLines[0]) {
			return fmt.Errorf("%w: segment %s written but live file not truncated", ErrSealResidue, filepath.Base(s))
		}
		rel, rerr := filepath.Rel(evolveDir, s)
		if rerr != nil {
			return fmt.Errorf("ledger verify: rel: %w", rerr)
		}
		segSHAs[filepath.ToSlash(rel)] = segSHA
		full = append(full, segLines...)
	}
	full = append(full, liveLines...)

	lastSeq, lastSha, sawV837, err := walkChain(full, l.loadAnchorSHA())
	if err != nil {
		return err
	}

	// Anchor binding: every segment must have a segment_seal entry whose
	// ArtifactSHA256 matches the segment's uncompressed bytes.
	anchors := map[string]string{}
	for _, line := range full {
		var e core.LedgerEntry
		if json.Unmarshal(line, &e) == nil && e.Kind == SealKind {
			anchors[e.ArtifactPath] = e.ArtifactSHA256
		}
	}
	for rel, sha := range segSHAs {
		anchorSHA, ok := anchors[rel]
		if !ok {
			return fmt.Errorf("%w: segment %s has no segment_seal anchor", ErrSealResidue, rel)
		}
		if anchorSHA != sha {
			return fmt.Errorf("%w: segment %s content does not match its anchor (have %s want %s)", core.ErrLedgerChainBroken, rel, sha, anchorSHA)
		}
	}

	if !sawV837 {
		return nil
	}
	return l.checkTip(lastSeq, lastSha)
}

// --- helpers ---

// segmentFiles lists seg-*.jsonl.gz in lexical order (we mint the names
// with a fixed-width counter, so lexical == chronological).
func segmentFiles(segDir string) ([]string, error) {
	entries, err := os.ReadDir(segDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger segments read: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "seg-") || !strings.HasSuffix(e.Name(), ".jsonl.gz") {
			continue
		}
		out = append(out, filepath.Join(segDir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

// writeSegment gzips raw into path atomically (tmp + rename + dir create).
func writeSegment(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ledger seal: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("ledger seal: tmp: %w", err)
	}
	tmpPath := tmp.Name()
	fail := func(stage string, err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ledger seal: %s: %w", stage, err)
	}
	zw := gzip.NewWriter(tmp)
	if _, err := zw.Write(raw); err != nil {
		return fail("gzip write", err)
	}
	if err := zw.Close(); err != nil {
		return fail("gzip close", err)
	}
	if err := tmp.Sync(); err != nil {
		return fail("sync", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ledger seal: close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ledger seal: rename: %w", err)
	}
	return nil
}

// readSegment gunzips a segment and returns its lines plus the sha256 of
// the uncompressed bytes.
func readSegment(path string) ([][]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("ledger segment open: %w", err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, "", fmt.Errorf("ledger segment %s gunzip: %w", filepath.Base(path), err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, "", fmt.Errorf("ledger segment %s read: %w", filepath.Base(path), err)
	}
	if err := zr.Close(); err != nil {
		return nil, "", fmt.Errorf("ledger segment %s gunzip close: %w", filepath.Base(path), err)
	}
	return splitLines(raw), sha256Hex(raw), nil
}

// rewriteLive atomically replaces ledger.jsonl with the given lines.
// The bytes are the ORIGINAL line bytes — never re-marshaled — so the
// hash chain over them is untouched.
func (l *FileLedger) rewriteLive(lines [][]byte) error {
	var buf bytes.Buffer
	for _, line := range lines {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	tmp, err := os.CreateTemp(filepath.Dir(l.ledgerPath), "ledger.jsonl.*.tmp")
	if err != nil {
		return fmt.Errorf("ledger seal: live rewrite tmp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ledger seal: live rewrite: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ledger seal: live rewrite sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ledger seal: live rewrite close: %w", err)
	}
	if err := os.Rename(tmpPath, l.ledgerPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ledger seal: live rename: %w", err)
	}
	return nil
}

// prefixLen returns the byte length of the first n lines of raw
// (including their trailing newlines).
func prefixLen(raw []byte, n int) int {
	off := 0
	for i := 0; i < n; i++ {
		idx := bytes.IndexByte(raw[off:], '\n')
		if idx < 0 {
			return len(raw)
		}
		off += idx + 1
	}
	return off
}

func linesEqual(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
