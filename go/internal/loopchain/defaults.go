package loopchain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func GitAhead(projectRoot, runningCommit string) (ahead bool, err error) {
	if runningCommit == "" {
		return false, nil
	}
	runningOut, err := sysexec.Command(context.Background(), "git", "-C", projectRoot, "rev-parse", runningCommit).Output()
	if err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: resolve running commit %q: %w", runningCommit, err)
	}
	running := strings.TrimSpace(string(runningOut))
	headOut, err := sysexec.Command(context.Background(), "git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: resolve HEAD: %w", err)
	}
	if strings.TrimSpace(string(headOut)) == running {
		return false, nil
	}
	if err := sysexec.Command(context.Background(), "git", "-C", projectRoot, "merge-base", "--is-ancestor", running, "HEAD").Run(); err != nil {
		return false, fmt.Errorf("chain boundary ahead-check: %q is not a verifiable ancestor of HEAD: %w", runningCommit, err)
	}
	return true, nil
}

func GitProvenance(runningCommit func() string) func(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return func(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
		return runningCommit(), func(c string) bool {
			if c == "" {
				return false
			}
			return sysexec.Command(context.Background(), "git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
		}
	}
}

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

func FleetLaneActive(evolveDir string) (active bool, err error) {
	_, active, err = LiveSiblingRun(evolveDir)
	return active, err
}

type SiblingRun struct {
	Dir    string
	Reason string
}

func LiveSiblingRun(evolveDir string) (SiblingRun, bool, error) {
	dirs, err := gc.Discover(evolveDir, gc.DiscoverOptions{})
	if err != nil {
		return SiblingRun{}, false, fmt.Errorf("chain boundary fleet-lane discovery: %w", err)
	}
	for _, d := range dirs {
		if !d.Live {
			continue
		}
		if reason := siblingReason(d.Path); reason != "" {
			return SiblingRun{Dir: d.Path, Reason: reason}, true, nil
		}
	}
	return SiblingRun{}, false, nil
}

func siblingReason(runDir string) string {
	lease, ok, err := runlease.Read(runDir)
	switch {
	case err != nil:
		return fmt.Sprintf("no readable lease (%v)", err)
	case !ok:
		return "no readable lease"
	case lease.OwnerPID == os.Getpid(), !runlease.OwnerLive(lease, time.Now(), 0, runlease.PIDAlive):
		return ""
	case lease.OwnerPID == 0:
		return "a fresh lease with no owner pid"
	}
	return fmt.Sprintf("live pid %d", lease.OwnerPID)
}
