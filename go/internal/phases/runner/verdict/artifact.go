package verdict

// artifact.go — the on-disk helpers of the verdict engine, moved verbatim from
// the host runner: the single-read decision, the forensic renderers, the
// (size, mtime) identity of the canonical artifact, and the ACS deterministic
// floor's predicate (its challenge-token read goes through
// phasecontract.ChallengeToken, the reader the host's prompt preparation
// shares).

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Snapshot is the (size, mtime) identity of the canonical artifact at one
// instant — the same key the bridge's stability window and baseline use, so
// the runner's and the bridge's notion of "unchanged" cannot drift. Only
// StatSnapshot mints one: the host takes it pre-dispatch and the engine judges
// it post-dispatch (the cycle-1550 stale-leftover gate).
type Snapshot struct {
	size    int64
	modTime time.Time
}

// StatSnapshot snapshots path if it is a non-empty regular file.
func StatSnapshot(path string) (Snapshot, bool) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
		return Snapshot{}, false
	}
	return Snapshot{size: fi.Size(), modTime: fi.ModTime()}, true
}

// unchangedSince reports whether path is still byte-identical (by the
// size+mtime key) to the given pre-dispatch snapshot. Any error reads as
// changed — fail-open toward the pre-existing reconcile behavior.
func unchangedSince(path string, snap Snapshot) bool {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	return fi.Size() == snap.size && fi.ModTime().Equal(snap.modTime)
}

// classifiedArtifact returns the content Classify must judge for a CONTRACTED
// phase, given the deliverable's Verify result, the artifact path THIS run
// dispatched, and the terminal pane as the last resort.
//
// SINGLE READ (deliverable-verified-bytes-single-read): when the Verify result
// describes the same file the bridge was told to write, its Content IS the
// classified content — verdict and content come from ONE read, so a writer
// racing the just-finished launch cannot slip bytes past the gate that judged
// them. The snapshot is authoritative only when it is BOTH of the same file
// AND non-empty: an infra read fault returns an empty Result, and a
// deliverable that materialises after the ladder's last probe verifies
// absent — in either case the snapshot would classify "" for a file that is
// on disk right now, so the fallback costs one read. The fallback reads the
// dispatched artifact exactly as the pre-single-read code did, including the
// "absent + !OK ⇒ empty artifact" rule that makes an unwritten deliverable a
// coherent FAIL instead of a pane-scraped one; the pane survives only for a
// path-less Result (a NoArtifact contract) or a fake that verified another
// path.
func classifiedArtifact(res deliverable.Result, artifactPath, pane string) string {
	if res.ArtifactPath == artifactPath && res.Content != "" {
		return res.Content
	}
	if data, err := os.ReadFile(artifactPath); err == nil {
		return string(data)
	}
	if !res.OK {
		return "" // contracted file genuinely absent → Classify sees no sentinel → FAIL
	}
	return pane
}

// forensicSnapshot reports a file's existence, byte size and last tailN bytes
// (where the audit-report.md verdict sentinel lives) as one log-safe token for
// the teardown-FAIL event. The read error is deliberately discarded (a
// directory renders size=N tail="") — the token's bytes are a golden.
func forensicSnapshot(path string, tailN int) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "absent"
	}
	data, _ := os.ReadFile(path)
	tail := string(data)
	if len(tail) > tailN {
		tail = tail[len(tail)-tailN:]
	}
	return fmt.Sprintf("size=%d tail=%q", fi.Size(), tail)
}

// forensicCodes renders a violation set as its comma-joined codes.
func forensicCodes(vs []deliverable.Violation) string {
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		parts = append(parts, string(v.Code))
	}
	return strings.Join(parts, ",")
}

// acsFloorRescues reports whether a teardown-time deliverable.Verify not-OK
// should be OVERRIDDEN by the deterministic ACS ground truth
// (verdict-incoherence family: cycles 603/921/924/931/3). report is the
// VERIFIED deliverable content (the bytes Verify read — Result.Content),
// never a fresh read: the rescue decision and the subsequent Classify must
// judge the SAME snapshot. True iff ALL hold:
//   - the phase is audit (the acs-verdict.json + coherence floor are audit-scoped);
//   - the acssuite verdict is PASS — a NON-LLM signal a session stall cannot corrupt;
//   - the report declares a PASS-class verdict sentinel (via the canonical
//     ParseVerdictSentinel, with its placeholder-echo guard — read by ReadCycleVerdicts);
//   - the report echoes THIS cycle's minted challenge token, read through
//     phasecontract.ChallengeToken (anti-gaming: a stale, forged, or cross-cycle
//     report cannot be laundered to PASS by the ACS verdict alone).
//
// This is precisely the (audit==PASS && acs==PASS) condition the ADR-0072
// coherence floor flags as incoherent — reusing coherence.ReadCycleVerdicts
// keeps a single definition of "both verdicts agree on PASS". It never
// manufactures a PASS: a malformed/verdict-less/token-missing report, or a
// non-ship-eligible suite, declines.
func acsFloorRescues(phase, workspace, report string) bool {
	if phase != string(core.PhaseAudit) {
		return false
	}
	audit, acs, auditRan := coherence.ReadCycleVerdicts(workspace)
	if !auditRan || audit != "PASS" || acs != "PASS" {
		return false
	}
	tok, ok := phasecontract.ChallengeToken(workspace)
	if !ok {
		return false
	}
	return strings.Contains(report, tok)
}
