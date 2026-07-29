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
// checked-out `main` onto origin/main. Returns true only when the tree
// actually moved. Every skip path is deliberate: not-on-main (sequential /
// console launches), no origin remote (offline dev), fetch failure
// (transient network — WARN), already current (quiet), blocked by local
// tracked changes (WARN, names the cause), diverged history (WARN — FF-only,
// the loop never merges).
func syncMainFromOriginAtWaveBoundary(ctx context.Context, projectRoot string, warn io.Writer) bool {
	if info, err := plane.Classify(projectRoot); err != nil || info.Branch != "main" {
		return false
	}
	sctx, cancel := context.WithTimeout(ctx, waveSyncTimeout)
	defer cancel()
	g := gitexec.Git{Dir: projectRoot, Exec: sysexec.DefaultRunner}
	if _, _, code, err := g.Capture(sctx, "remote", "get-url", "origin"); err != nil || code != 0 {
		return false
	}
	if _, stderr, code, err := g.Capture(sctx, "fetch", "-q", "origin", "main"); err != nil || code != 0 {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: fetch origin failed (rc=%d %v: %s) — planning against the local main\n", code, err, strings.TrimSpace(stderr))
		return false
	}
	local, _, lcode, lerr := g.Capture(sctx, "rev-parse", "HEAD")
	remote, _, rcode, rerr := g.Capture(sctx, "rev-parse", "origin/main")
	if lerr != nil || rerr != nil || lcode != 0 || rcode != 0 {
		return false
	}
	if strings.TrimSpace(local) == strings.TrimSpace(remote) {
		return false // already current — the steady state stays silent
	}
	// Probe ancestry BEFORE merging (review HIGH): --ff-only also refuses
	// when LOCAL TRACKED CHANGES would be overwritten — a normal runtime
	// state (rebuilt-binary churn) that must not be misdiagnosed as history
	// divergence, whose "next ship reconciles" remedy is the stowaway class.
	_, _, acode, aerr := g.Capture(sctx, "merge-base", "--is-ancestor", "HEAD", "origin/main")
	behindOnly := aerr == nil && acode == 0
	if _, stderr, code, err := g.Capture(sctx, "merge", "--ff-only", "origin/main"); err != nil || code != 0 {
		if behindOnly {
			fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: local tracked changes block the fast-forward (rc=%d: %s) — resolve the dirt (or console-lease it) rather than shipping it\n", code, strings.TrimSpace(stderr))
		} else {
			fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: local main diverged from origin/main (rc=%d: %s) — the loop never merges; the next lane ship's push reconciles\n", code, strings.TrimSpace(stderr))
		}
		return false
	}
	fmt.Fprintf(warn, "[loop] wave-boundary sync: fast-forwarded main to origin/main (%.12s)\n", strings.TrimSpace(remote))
	return true
}
