package core

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const laneQuotaReason = "quota-likely"

func ScopeIDs(scope string) []string {
	return normalizedIDs(strings.Split(scope, ","))
}

func FindLaneResume(projectRoot, evolveDir string, scope []string, opts ResumeOptions) (*ResumePoint, bool) {
	want := normalizedIDs(scope)
	if len(want) == 0 {
		return nil, false
	}
	states, err := readPerRunStates(evolveDir)
	if err != nil {
		return nil, false
	}
	last, found := latestAttempt(states, want)
	if !found || !laneQuotaPaused(last) {
		return nil, false
	}
	opts = opts.withDefaults().withMaxAge(evolveDir)
	rp, err := loadResumeStateFrom(last.path, projectRoot, opts)
	if err != nil {
		fmt.Fprintf(opts.Log, "[resume] lane %s: cycle %d checkpoint is not resumable: %v\n", strings.Join(want, ","), last.cycle, err)
		return nil, false
	}
	return rp, true
}

func latestAttempt(states []perRunState, scope []string) (perRunState, bool) {
	var last perRunState
	found := false
	for _, st := range states {
		if !slices.Equal(normalizedIDs(LaneScopeIDs(filepath.Dir(st.path))), scope) {
			continue
		}
		if !found || st.cycle > last.cycle {
			last, found = st, true
		}
	}
	return last, found
}

func laneQuotaPaused(st perRunState) bool {
	return st.enabled && st.reason == laneQuotaReason && !st.leased && st.phase != string(PhaseEnd) && st.phase != "aborted"
}

const resumeAdmissionRunID = "lane-resume-admission"

type ResumeHeldError struct {
	Workspace string
	Holder    runlease.Lease
	Cause     error
}

func (e *ResumeHeldError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("resume: %s is held by an unreadable lease until its file ages past %s: %v", e.Workspace, runlease.DefaultTTL, e.Cause)
	}
	return fmt.Sprintf("resume: %s is held by run %q, pid %d (heartbeat %s)", e.Workspace, e.Holder.RunID, e.Holder.OwnerPID, e.Holder.HeartbeatAt)
}

func resumeHolder(workspace string, now time.Time) (runlease.Lease, bool, error) {
	l, ok, err := runlease.Read(workspace)
	if err != nil {
		info, serr := os.Stat(runlease.PathIn(workspace))
		return runlease.Lease{}, serr == nil && now.Sub(info.ModTime()) < runlease.DefaultTTL, err
	}
	return l, ok && runlease.Fresh(l, now, 0), nil
}

func TakeResumeLease(workspace string, now time.Time) error {
	holder, held, cause := resumeHolder(workspace, now)
	if held {
		return &ResumeHeldError{Workspace: workspace, Holder: holder, Cause: cause}
	}
	return runlease.Write(workspace, runlease.Lease{RunID: resumeAdmissionRunID, OwnerPID: os.Getpid()}, now)
}

func ReleaseResumeLease(workspace string) error {
	l, ok, err := runlease.Read(workspace)
	if err != nil || !ok || l.RunID != resumeAdmissionRunID || l.OwnerPID != os.Getpid() {
		return err
	}
	return os.Remove(runlease.PathIn(workspace))
}

func scopedFleetLane(env map[string]string) bool {
	return fleetMode(env) && len(ScopeIDs(env[ipcenv.FleetScopeKey])) > 0
}

func normalizedIDs(raw []string) []string {
	var ids []string
	for _, id := range raw {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
