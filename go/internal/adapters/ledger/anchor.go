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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// ErrAmbiguousAnchorSeq is returned when more than one DISTINCT line carries the
// requested entry_seq, so no single line can be bound without the operator
// naming it.
//
// entry_seq is not unique in real history: pre-CA.1 concurrent Appends raced the
// tip and wrote fork siblings sharing a seq (walkChain accepts them; see its
// FORK SIBLING carve-out). Binding the FIRST match — what this command did until
// cycle-1433 — silently picks the EARLIER sibling, which moves the epoch anchor
// BACKWARD and re-exposes lines the operator believed were already sealed. An
// anchor is a trust decision, so an ambiguous one must be refused, not guessed;
// the caller disambiguates by line SHA, which names exact bytes.
var ErrAmbiguousAnchorSeq = errors.New("ambiguous entry_seq: carried by more than one distinct line")

// resetSealKindPrefix is the Kind prefix of an IN-BAND epoch anchor: an
// operator entry the chain itself carries (e.g. `reset-seal-cycle1189`).
//
// It exists alongside the out-of-band ledger-anchor.json because the two
// answer different operational needs. The JSON anchor is a one-off remedy an
// operator records ABOUT a line that already exists (`evolve ledger anchor
// <seq>`); the in-band seal is written AS the chain moves on, so a recovery
// that resumes the chain leaves its own sign-off inside the auditable record
// instead of in a sidecar file that can be lost, copied, or diverge from it.
// Both mean the same thing to the walk — "history before me is preserved but
// no longer chain-validated" — so they resolve through one helper and the
// LATER of the two always wins.
const resetSealKindPrefix = "reset-seal-"

// operatorRole is the Role an in-band seal must carry. Sealing is a TRUST
// decision, so a phase agent writing `reset-seal-*` under its own role must
// not be able to silence Verify. This is also why SealCycle's unattended
// boot self-heal path (core.SealOptions.AutomatedRecovery, triggered merely
// by a dead owner PID) writes a DIFFERENT role ("operator-autoseal") rather
// than this one: without that split, arranging for any owning process to die
// was enough to mint a trust-anchor-eligible seal with no human ever
// involved, since Role/CycleLabel are otherwise self-declared fields with no
// authentication of their own.
const operatorRole = "operator"

// isOperatorSeal reports whether e is an in-band operator epoch seal.
//
// The marker is accepted in EITHER field, because the ledger carries two
// shapes and only one of them was ever recognised: the PRODUCTION writer
// (core.SealCycle, go/internal/core/reset.go) emits `kind:"reset"` with the
// marker in `cycle_label` ("reset-seal-cycle-108"), while the doc-level shape
// puts it in `kind`. Matching on Kind alone made this resolver inert on every
// real ledger — unit-green, live-red — so `evolve ledger verify` kept crying
// wolf on the adjudicated line-1740 damage (cycle-1191). Role is still the
// authority check: a phase agent cannot silence Verify under its own role.
func isOperatorSeal(e core.LedgerEntry) bool {
	if e.Role != operatorRole {
		return false
	}
	return strings.HasPrefix(e.Kind, resetSealKindPrefix) ||
		strings.HasPrefix(e.CycleLabel, resetSealKindPrefix)
}

// sealChainsFromPrev is the seal TRUST GUARD: an in-band seal may only move the
// epoch anchor forward when it is itself hash-valid from its own predecessor.
//
// Without it, appending one operator-role `reset-seal-*` line carrying a forged
// prev_hash would silence verification of the ENTIRE prefix behind it, turning
// the epoch anchor from a preservation remedy into a chain-integrity bypass. A
// pre-v8.37 line (no prev_hash key at all) carries no self-proof, so it can
// never anchor either.
func sealChainsFromPrev(hasPrev bool, e core.LedgerEntry, prevLineSHA string) bool {
	if !hasPrev {
		return false
	}
	if prevLineSHA == "" {
		return e.PrevHash == ZeroSeed // the seal is itself the genesis line
	}
	return e.PrevHash == prevLineSHA
}

// effectiveAnchorSHA resolves the epoch anchor the chain walk should start
// from: the LAST of (the out-of-band ledger-anchor.json line, any self-valid
// in-band operator seal at or after it). Returns "" when neither exists, which
// is full-strict verification.
//
// When fileAnchorSHA is set but no line carries it, it is returned unchanged so
// walkChain still reports "anchor not found" — a stale or tampered sidecar must
// fail loudly, never silently degrade to "no anchor, verify everything".
func effectiveAnchorSHA(lines [][]byte, fileAnchorSHA string) string {
	anchor := fileAnchorSHA
	// Seals BEFORE the sidecar anchor are already inside the untrusted prefix,
	// so only seals at/after it may move the anchor forward.
	reached := fileAnchorSHA == ""
	prevLineSHA := ""
	for _, line := range lines {
		sha := sha256Hex(line)
		switch {
		case !reached && sha == fileAnchorSHA:
			reached = true
		case !reached:
			// still inside the untrusted prefix
		default:
			if hasPrev, e, err := decodeLedgerLine(line); err == nil &&
				isOperatorSeal(e) && sealChainsFromPrev(hasPrev, e, prevLineSHA) {
				anchor = sha
			}
		}
		prevLineSHA = sha
	}
	return anchor
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

// resolveAnchorLine picks the single line SHA an anchor may bind to.
//
// wantSHA == "": the seq must be carried by exactly ONE distinct line —
// zero is "not found", more than one is ErrAmbiguousAnchorSeq (never a silent
// first-match). wantSHA set: that line must exist AND carry the requested seq,
// so a mistyped seq cannot bind an arbitrary line — the flag disambiguates
// between siblings, it does not override the seq argument.
//
// Distinctness is by SHA, not by position: two byte-identical lines are one set
// of bytes to bind and stay unambiguous.
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

// Anchor records an epoch-anchor at the ledger line whose entry_seq == seq,
// binding it to that line's current SHA. After this, Verify/VerifyDeep trust the
// pre-anchor prefix (the preserved, accepted historical damage) and validate
// strictly from the anchor forward. Errors (leaving no anchor file) when no line
// carries that seq, or when several do — see AnchorLine.
func (l *FileLedger) Anchor(ctx context.Context, seq int, note string) error {
	return l.AnchorLine(ctx, seq, "", note)
}

// AnchorLine is Anchor with an explicit line SHA, the disambiguation an
// ambiguous seq (ErrAmbiguousAnchorSeq) requires: lineSHA names the exact bytes
// to trust when siblings share a seq. An empty lineSHA is plain Anchor.
// Atomic write (temp + rename); no anchor file is left behind on any error.
func (l *FileLedger) AnchorLine(_ context.Context, seq int, lineSHA, note string) error {
	// Search the FULL chain — sealed segments + live tail — not just
	// ledger.jsonl: the ledger-1740 damage is old enough that its post-damage
	// line has likely been sealed into a segment.
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
	// os.CreateTemp gives a unique name so concurrent invocations never collide
	// on the temp path; rename is atomic (POSIX). Clean up the temp on any
	// failure after creation so a failed anchor leaves no residue.
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
