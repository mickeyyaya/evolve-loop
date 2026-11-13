package core

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

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
