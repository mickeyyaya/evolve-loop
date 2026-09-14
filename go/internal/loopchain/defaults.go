package loopchain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// GitAhead reports whether HEAD carries commits beyond runningCommit — the
// ancestor-check idiom `evolve reset-sha` uses, inverted: ahead=true means
// runningCommit is a STRICT ancestor of HEAD. An empty runningCommit (an
// unstamped dev binary) is a quiet no-op. Any git/repo failure returns an
// error the caller treats as "skip this boundary's refresh", never a halt.
// The Makefile stamps a 12-char SHORT commit while `git rev-parse HEAD`
// returns the full SHA, and `merge-base --is-ancestor` treats equal commits
// as ancestors too — so the stamp is resolved to its full SHA FIRST (Q-C1,
// cycle 1320) or an up-to-date binary would always look ahead.
func GitAhead(projectRoot, runningCommit string) (ahead bool, err error) {
	if runningCommit == "" {
		return false, nil
	}
	runningOut, err := exec.Command("git", "-C", projectRoot, "rev-parse", runningCommit).Output()
	if err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: resolve running commit %q: %w", runningCommit, err)
	}
	running := strings.TrimSpace(string(runningOut))
	headOut, err := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: resolve HEAD: %w", err)
	}
	if strings.TrimSpace(string(headOut)) == running {
		return false, nil
	}
	if err := exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", running, "HEAD").Run(); err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: %q is not a verifiable ancestor of HEAD: %w", runningCommit, err)
	}
	return true, nil
}

// GitProvenance is the boundary re-pin's provenance: the running binary's
// build commit (read from runningCommit at every call, never snapshotted)
// plus a closure asserting a commit is an ancestor of HEAD. An empty commit
// is unverifiable and returns false — a stripped or tampered binary can never
// self-authorize a re-pin, and there is no sentinel substitute (cycle 1320:
// "the rebuild just succeeded, so it must be HEAD" is not a verification).
func GitProvenance(runningCommit func() string) func(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return func(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
		return runningCommit(), func(c string) bool {
			if c == "" {
				return false
			}
			return exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
		}
	}
}

// RebuiltBinary returns the REBUILT <projectRoot>/go/bin/evolve — the artifact
// the rebuild just wrote — never os.Args[0], which is the STALE running image
// the refresh exists to escape. An absent or non-executable target is an
// error, so the boundary degrades to "no refresh" rather than exec'ing an
// unknown path.
func RebuiltBinary(projectRoot string) (string, error) {
	target := filepath.Join(projectRoot, "go", "bin", "evolve")
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("rebuilt binary %s: %w", target, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("rebuilt binary %s is not executable (mode %s)", target, info.Mode())
	}
	return target, nil
}

// FleetLaneActive reports whether a SIBLING fleet lane holds a live run under
// evolveDir. Discovery is gc.Discover, the lease-aware run-dir scan the
// retention engine owns; the lease of each Live dir is then re-read here for
// the two questions retention never asks — whose is it, and is the owner
// alive? A run dir is a sibling when its lease is not this process's own
// (runlease.Lease.OwnerPID == os.Getpid(): the whole chain runs in one process
// and every lease it writes carries its pid, while a different lane is by
// construction a different process — cycle 1364) AND its owner is live
// (runlease.OwnerLive: a sealed lane's lease outlives its process and stays
// fresh for a TTL — 2026-09-15, twice, with no lane running). Retention keeps
// its TTL-only Live on purpose (it errs toward keeping dirs). A dir Live
// through gc's other liveness source (the current workspace, no fresh lease)
// has no pid to compare and is NOT excluded — the fail-safe posture. A
// discovery error is unverifiable safety state.
func FleetLaneActive(evolveDir string) (active bool, err error) {
	dirs, err := gc.Discover(evolveDir, gc.DiscoverOptions{})
	if err != nil {
		return false, fmt.Errorf("chain boundary fleet-lane discovery: %w", err)
	}
	selfPID := os.Getpid()
	for _, d := range dirs {
		if !d.Live {
			continue
		}
		if lease, ok, lerr := runlease.Read(d.Path); lerr == nil && ok {
			if lease.OwnerPID == selfPID {
				continue
			}
			// A sealed lane's lease outlives its process (the writer stops
			// heartbeating at exit; the file stays fresh for a TTL): the
			// owner's liveness decides, not the timestamp. An ownerless
			// lease cannot be probed and stays a sibling.
			if !runlease.OwnerLive(lease, time.Now(), 0, runlease.PIDAlive) {
				continue
			}
		}
		return true, nil
	}
	return false, nil
}
