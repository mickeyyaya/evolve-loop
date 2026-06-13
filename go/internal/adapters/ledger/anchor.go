// anchor.go — ADR-0048 ledger epoch-anchor (the ledger-1740 disposition).
//
// A predecessor's bytes were rewritten post-hoc (cycle-107 era), permanently
// breaking the SHA chain at that point; `evolve ledger verify` correctly stays
// RED on it. The non-destructive remedy (ADR-0048 §non-goals, operator-chosen
// over a destructive rebaseline) is an EPOCH-ANCHOR: declare a known-good
// genesis at a post-damage line and verify FORWARD from it. The damaged segment
// is PRESERVED in the file (auditable), it is simply no longer chain-validated.
//
// This is an operator TRUST decision — the ADR requires sign-off — so it is an
// explicit command (`evolve ledger anchor <seq>`), never automatic. The anchor
// binds to the target line's CURRENT SHA, so any later alteration of the trusted
// prefix self-invalidates the anchor (walkChain then fails "anchor not found")
// rather than silently extending trust to tampered bytes.
package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ledgerAnchor is the on-disk shape of <evolveDir>/ledger-anchor.json.
type ledgerAnchor struct {
	AnchorSeq     int    `json:"anchor_seq"`
	AnchorLineSHA string `json:"anchor_line_sha256"`
	RecordedAt    string `json:"recorded_at"`
	Note          string `json:"note"`
}

// loadAnchorSHA returns the recorded epoch-anchor line SHA, or "" when no anchor
// is set or the file is unreadable/corrupt. Degrading to "" means FULL-STRICT
// verification (no relaxation) — a missing/garbled anchor never silently trusts
// a damaged chain; it just verifies everything, as if no anchor existed.
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

// Anchor records an epoch-anchor at the ledger line whose entry_seq == seq,
// binding it to that line's current SHA. After this, Verify/VerifyDeep trust the
// pre-anchor prefix (the preserved, accepted historical damage) and validate
// strictly from the anchor forward. Errors (leaving no anchor file) when no line
// carries that seq. Atomic write (temp + rename).
func (l *FileLedger) Anchor(_ context.Context, seq int, note string) error {
	// Search the FULL chain — sealed segments + live tail — not just
	// ledger.jsonl: the ledger-1740 damage is old enough that its post-damage
	// line has likely been sealed into a segment.
	lines, err := l.gatherAllLines()
	if err != nil {
		return fmt.Errorf("ledger anchor: %w", err)
	}
	lineSHA := ""
	for _, line := range lines {
		_, e, derr := decodeLedgerLine(line)
		if derr != nil {
			continue
		}
		if e.EntrySeq == seq {
			lineSHA = sha256Hex(line)
			break // first line carrying this seq (siblings share a seq; the SHA binds the exact bytes)
		}
	}
	if lineSHA == "" {
		return fmt.Errorf("ledger anchor: no line with entry_seq=%d (searched live tail + sealed segments)", seq)
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
	// os.CreateTemp gives a unique name so concurrent invocations never collide
	// on the temp path; rename is atomic (POSIX). Clean up the temp on any
	// failure after creation so a failed anchor leaves no residue.
	f, err := os.CreateTemp(filepath.Dir(l.anchorPath), "ledger-anchor.*.tmp")
	if err != nil {
		return fmt.Errorf("ledger anchor: create temp: %w", err)
	}
	tmp := f.Name()
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("ledger anchor: write: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ledger anchor: close: %w", err)
	}
	if err := os.Rename(tmp, l.anchorPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ledger anchor: rename: %w", err)
	}
	return nil
}
