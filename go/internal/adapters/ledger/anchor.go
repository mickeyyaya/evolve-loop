package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var ErrAmbiguousAnchorSeq = errors.New("ambiguous entry_seq: carried by more than one distinct line")

const resetSealKindPrefix = "reset-seal-"

const operatorRole = "operator"

func isOperatorSeal(e core.LedgerEntry) bool {
	if e.Role != operatorRole {
		return false
	}
	return strings.HasPrefix(e.Kind, resetSealKindPrefix) ||
		strings.HasPrefix(e.CycleLabel, resetSealKindPrefix)
}

func sealChainsFromPrev(hasPrev bool, e core.LedgerEntry, prevLineSHA string) bool {
	if !hasPrev {
		return false
	}
	if prevLineSHA == "" {
		return e.PrevHash == ZeroSeed
	}
	return e.PrevHash == prevLineSHA
}

type VerifiedScope struct {
	AnchorLineSHA     string
	AnchorSeq         int
	FromSealedSegment bool
}

func effectiveAnchorSHA(lines [][]byte, fileAnchorSHA string) (anchorSHA string, anchorSeq int) {
	anchorSHA = fileAnchorSHA
	reached := fileAnchorSHA == ""
	prevLineSHA := ""
	for _, line := range lines {
		sha := sha256Hex(line)
		switch {
		case !reached && sha == fileAnchorSHA:
			reached = true
			if _, e, err := decodeLedgerLine(line); err == nil {
				anchorSeq = e.EntrySeq
			}
		case !reached:
		default:
			if hasPrev, e, err := decodeLedgerLine(line); err == nil &&
				isOperatorSeal(e) && sealChainsFromPrev(hasPrev, e, prevLineSHA) {
				anchorSHA, anchorSeq = sha, e.EntrySeq
			}
		}
		prevLineSHA = sha
	}
	return anchorSHA, anchorSeq
}

type ledgerAnchor struct {
	AnchorSeq     int    `json:"anchor_seq"`
	AnchorLineSHA string `json:"anchor_line_sha256"`
	RecordedAt    string `json:"recorded_at"`
	Note          string `json:"note"`
}

var (
	anchorCreateTemp = os.CreateTemp
	anchorWrite      = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
	anchorClose      = func(f *os.File) error { return f.Close() }
)

func (l *FileLedger) loadAnchorSHA() string {
	raw, err := os.ReadFile(l.anchorPath)
	if err != nil {
		return ""
	}
	var a ledgerAnchor
	if err := json.Unmarshal(raw, &a); err != nil {
		return ""
	}
	return a.AnchorLineSHA
}

func resolveAnchorLine(lines [][]byte, seq int, wantSHA string) (string, error) {
	seqOf := map[string]int{}
	var candidates []string
	for _, line := range lines {
		_, e, derr := decodeLedgerLine(line)
		if derr != nil {
			continue
		}
		sha := sha256Hex(line)
		if _, seen := seqOf[sha]; seen {
			continue
		}
		seqOf[sha] = e.EntrySeq
		if e.EntrySeq == seq {
			candidates = append(candidates, sha)
		}
	}
	if wantSHA != "" {
		got, ok := seqOf[wantSHA]
		if !ok {
			return "", fmt.Errorf("ledger anchor: no line with line SHA %s (searched live tail + sealed segments)", wantSHA)
		}
		if got != seq {
			return "", fmt.Errorf("ledger anchor: the named line carries entry_seq=%d, not entry_seq=%d — <entry_seq> and the line SHA must name the SAME line", got, seq)
		}
		return wantSHA, nil
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("ledger anchor: no line with entry_seq=%d (searched live tail + sealed segments)", seq)
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("ledger anchor: %w: entry_seq=%d is carried by %d distinct lines (%s) — name the exact line to trust",
			ErrAmbiguousAnchorSeq, seq, len(candidates), strings.Join(candidates, ", "))
	}
}

func (l *FileLedger) Anchor(ctx context.Context, seq int, note string) error {
	return l.AnchorLine(ctx, seq, "", note)
}

func (l *FileLedger) AnchorLine(_ context.Context, seq int, lineSHA, note string) error {
	lines, err := l.gatherAllLines()
	if err != nil {
		return fmt.Errorf("ledger anchor: %w", err)
	}
	lineSHA, err = resolveAnchorLine(lines, seq, lineSHA)
	if err != nil {
		return err
	}
	rec := ledgerAnchor{
		AnchorSeq:     seq,
		AnchorLineSHA: lineSHA,
		RecordedAt:    time.Now().UTC().Format(time.RFC3339),
		Note:          note,
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("ledger anchor: marshal: %w", err)
	}
	f, err := anchorCreateTemp(filepath.Dir(l.anchorPath), "ledger-anchor.*.tmp")
	if err != nil {
		return fmt.Errorf("ledger anchor: create temp: %w", err)
	}
	tmp := f.Name()
	if _, err := anchorWrite(f, append(b, '\n')); err != nil {
		_ = anchorClose(f)
		_ = os.Remove(tmp)
		return fmt.Errorf("ledger anchor: write: %w", err)
	}
	if err := anchorClose(f); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ledger anchor: close: %w", err)
	}
	if err := os.Rename(tmp, l.anchorPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ledger anchor: rename: %w", err)
	}
	return nil
}
