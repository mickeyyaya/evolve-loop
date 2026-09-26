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

// ErrAmbiguousAnchorSeq means several distinct lines carry the requested entry_seq, so the operator must name one.
var ErrAmbiguousAnchorSeq = errors.New("ambiguous entry_seq: carried by more than one distinct line")

// resetSealKindPrefix marks an in-band epoch anchor, an operator seal the chain itself carries. It and
// ledger-anchor.json resolve through one helper, and the later of the two wins.
// See ADR-0081.
const resetSealKindPrefix = "reset-seal-"

// operatorRole is the only Role whose in-band seal moves the anchor. Role is self-declared, so the unattended
// autoseal path writes "operator-autoseal": a dead owner PID must not mint a trust anchor.
const operatorRole = "operator"

// isOperatorSeal reports whether e is an in-band operator seal. The marker may sit in Kind or in
// CycleLabel, where core.SealCycle writes it; Role is the authority check.
func isOperatorSeal(e core.LedgerEntry) bool {
	if e.Role != operatorRole {
		return false
	}
	return strings.HasPrefix(e.Kind, resetSealKindPrefix) ||
		strings.HasPrefix(e.CycleLabel, resetSealKindPrefix)
}

// sealChainsFromPrev is the seal trust guard: a seal moves the anchor only when it chains from its own
// predecessor, or one forged operator line would silence the whole prefix behind it.
func sealChainsFromPrev(hasPrev bool, e core.LedgerEntry, prevLineSHA string) bool {
	if !hasPrev {
		return false
	}
	if prevLineSHA == "" {
		return e.PrevHash == ZeroSeed // the seal is itself the genesis line
	}
	return e.PrevHash == prevLineSHA
}

// VerifiedScope names the history a successful verification validated; the zero value means full-strict.
type VerifiedScope struct {
	// AnchorLineSHA is the epoch-anchor line strict validation resumed from; "" means from genesis.
	AnchorLineSHA string
	// AnchorSeq is that line's own entry_seq; meaningless when AnchorLineSHA is "".
	AnchorSeq int
}

// effectiveAnchorSHA resolves the later of the ledger-anchor.json line and any self-valid operator seal
// after it, with that line's own entry_seq: the sidecar's anchor_seq goes stale once a seal moves past it.
// A sidecar SHA no line carries is returned unchanged so walkChain fails loudly instead of relaxing.
func effectiveAnchorSHA(lines [][]byte, fileAnchorSHA string) (anchorSHA string, anchorSeq int) {
	anchorSHA = fileAnchorSHA
	// Seals before the sidecar anchor sit in the untrusted prefix, so only later ones may move it.
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
			// still inside the untrusted prefix
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

// ledgerAnchor is the on-disk shape of <evolveDir>/ledger-anchor.json.
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

// loadAnchorSHA returns the recorded anchor line SHA, or "" when none is set or the file is unreadable.
// "" means full-strict verification, so a garbled anchor never relaxes the walk.
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

// resolveAnchorLine picks the one line SHA an anchor may bind: seq must name exactly one distinct line
// (by SHA, so byte-identical duplicates count once), and a named wantSHA must itself carry seq.
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

// Anchor records an epoch anchor bound to the current SHA of the one line whose entry_seq is seq.
func (l *FileLedger) Anchor(ctx context.Context, seq int, note string) error {
	return l.AnchorLine(ctx, seq, "", note)
}

// AnchorLine is Anchor with an explicit line SHA that picks one of several siblings; errors leave no anchor file.
func (l *FileLedger) AnchorLine(_ context.Context, seq int, lineSHA, note string) error {
	// Search sealed segments too: damage old enough to anchor past is usually sealed.
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
	// A unique temp name keeps concurrent anchors apart and rename is atomic; failures remove the temp.
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
