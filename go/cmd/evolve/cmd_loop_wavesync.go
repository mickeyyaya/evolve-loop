package main

// cmd_loop_wavesync.go — ADR-0080 S3: refresh the runtime plane from origin
// at the wave boundary, fast-forward ONLY. Post-cutover, origin/main is the
// single integration channel (console PRs merge there; lane ships push
// there), so a wave that plans against a stale local main bases its lanes on
// work origin has already superseded. FF-only covers HISTORY safety; the one
// review-HIGH hazard it does not cover is handled explicitly below: a merge
// refusal distinguishes "local tracked changes block FF" (expected: binary
// rebuild churn) from real history divergence — the wrong diagnosis
// prescribed the stowaway-adopting remedy.
//
// Self-SHA note (review round 2, investigated to ground truth): an FF CANNOT
// drift the ship self-SHA pin — verifySelfSHA hashes os.Executable()
// (gitignored bin/evolve, an untracked build output no git operation
// rewrites). The hazard is confined to operator rebuilds, already covered by
// the boot and post-build re-pins. An earlier draft shipped a repin seam
// here; it was provably inert and was DELETED rather than left as a false
// protection claim (ADR-0080 implementation notes).

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// waveSyncTimeout bounds the fetch — a wedged network must delay one wave by
// at most this, never hang the boundary.
const waveSyncTimeout = 60 * time.Second

// syncMainFromOriginAtWaveBoundary fetches origin and fast-forwards a
// checked-out `main` onto origin/main. synced is true only when the tree
// actually moved. Every skip path is deliberate: not-on-main (sequential /
// console launches), no origin remote (offline dev), fetch failure
// (transient network — WARN), already current (quiet), local main AHEAD
// (WARN — the local integration HEAD is the lane base; nothing moved),
// blocked by local tracked changes (WARN, names the cause). DIVERGED history
// returns halt: the ONE main-relation resolver (gitexec.RelationToRemote)
// that laneStartRef also reads would refuse every lane, so the batch stops
// here, before any lane spends a phase, with the same sentence — the two
// consumers can no longer disagree (2026-09-09 token-waste root cause #3).
func syncMainFromOriginAtWaveBoundary(ctx context.Context, projectRoot string, warn io.Writer) (synced bool, halt error) {
	if info, err := plane.Classify(projectRoot); err != nil || info.Branch != "main" {
		return false, nil
	}
	sctx, cancel := context.WithTimeout(ctx, waveSyncTimeout)
	defer cancel()
	g := gitexec.Git{Dir: projectRoot, Exec: sysexec.DefaultRunner}
	if _, _, code, err := g.Capture(sctx, "remote", "get-url", "origin"); err != nil || code != 0 {
		return false, nil
	}
	if _, stderr, code, err := g.Capture(sctx, "fetch", "-q", "origin", "main"); err != nil || code != 0 {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: fetch origin failed (rc=%d %v: %s) — planning against the local main\n", code, err, strings.TrimSpace(stderr))
		return false, nil
	}
	rel, err := g.RelationToRemote(sctx, "origin/main")
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %v — planning against the local main\n", err)
		return false, nil
	}
	switch rel.Kind {
	case gitexec.RelationCurrent:
		return false, nil // the steady state stays silent
	case gitexec.RelationAhead:
		// --ff-only would "succeed" without moving HEAD here; say what is true.
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %s; the next lane ship's push publishes them\n", rel)
		return false, nil
	case gitexec.RelationDiverged:
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %s\n", rel)
		return false, fmt.Errorf("wave-boundary sync: %s", rel)
	}
	// Behind: fast-forward. --ff-only can still refuse when LOCAL TRACKED
	// CHANGES would be overwritten — a normal runtime state (rebuilt-binary
	// churn) that must be named as such, never as divergence (whose "next
	// ship reconciles" remedy is the stowaway class).
	if _, stderr, code, err := g.Capture(sctx, "merge", "--ff-only", "origin/main"); err != nil || code != 0 {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: local tracked changes block the fast-forward (rc=%d: %s) — resolve the dirt (or console-lease it) rather than shipping it\n", code, strings.TrimSpace(stderr))
		return false, nil
	}
	fmt.Fprintf(warn, "[loop] wave-boundary sync: fast-forwarded main to origin/main (%.12s)\n", rel.Remote)
	return true, nil
}
