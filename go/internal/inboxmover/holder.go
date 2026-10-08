package inboxmover

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type HolderVerdict string

const (
	HolderLive          HolderVerdict = "live"
	HolderResumePending HolderVerdict = "resume-pending"
	HolderStale         HolderVerdict = "stale"
)

type HolderEvidence struct {
	Lease            string `json:"lease"`
	Phase            string `json:"phase,omitempty"`
	Checkpoint       string `json:"checkpoint,omitempty"`
	Closeout         string `json:"closeout,omitempty"`
	SupersededBy     int    `json:"superseded_by,omitempty"`
	SupersededByLoop bool   `json:"superseded_by_running_loop,omitempty"`
}

type Holder struct {
	Cycle    int            `json:"cycle"`
	Verdict  HolderVerdict  `json:"verdict"`
	Reason   string         `json:"reason"`
	Evidence HolderEvidence `json:"evidence"`
}

func (h Holder) Keeps() bool { return h.Verdict == HolderLive || h.Verdict == HolderResumePending }

func ClassifyHolder(opts Options, cycle int) Holder {
	opts.resolveOpts()
	return newHolderJudge(opts).classify(cycle)
}

type holderState struct {
	CycleID  int    `json:"cycle_id"`
	Phase    string `json:"phase"`
	GoalHash string `json:"goal_hash"`
	path     string
	perRun   bool
}

type holderJudge struct {
	evolveDir, projectRoot, currentGoal string
	now                                 time.Time
	goals                               map[int]string
}

func newHolderJudge(opts Options) holderJudge {
	evolveDir := filepath.Dir(opts.InboxDir)
	projectRoot := opts.ProjectRoot
	if projectRoot == "" {
		projectRoot = filepath.Dir(evolveDir)
	}
	j := holderJudge{evolveDir: evolveDir, projectRoot: projectRoot, currentGoal: opts.CurrentGoal, now: opts.Now()}
	j.goals = j.readGoals()
	return j
}

func (j holderJudge) runDir(cycle int) string {
	return filepath.Join(j.evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle))
}

func (j holderJudge) classify(cycle int) Holder {
	live, lease := j.lease(cycle)
	st := j.state(cycle)
	h := Holder{Cycle: cycle, Evidence: HolderEvidence{Lease: lease, Phase: st.Phase}}
	reason, resumeErr := j.resumable(st)
	h.Evidence.Checkpoint = reason
	h.Evidence.Closeout, _ = dossier.CloseoutPath(j.projectRoot, cycle)
	h.Evidence.SupersededBy = j.supersededBy(cycle, st.GoalHash)
	h.Evidence.SupersededByLoop = j.currentGoal != "" && j.currentGoal != st.GoalHash
	h.Verdict, h.Reason = verdict(h.Evidence, live, resumeErr)
	return h
}

func verdict(ev HolderEvidence, live bool, resumeErr error) (HolderVerdict, string) {
	switch {
	case live:
		return HolderLive, "lease " + ev.Lease
	case ev.Closeout != "":
		return HolderStale, "closed out: " + ev.Closeout
	case ev.Phase == "end":
		return HolderStale, "cycle-state phase end"
	case resumeErr != nil:
		return HolderStale, "no live lease and no resumable checkpoint: " + resumeErr.Error()
	case ev.SupersededByLoop:
		return HolderStale, fmt.Sprintf("paused (%s), but the running loop has a newer goal", ev.Checkpoint)
	case ev.SupersededBy > 0:
		return HolderStale, fmt.Sprintf("paused (%s), but superseded by cycle %d of a newer goal", ev.Checkpoint, ev.SupersededBy)
	}
	return HolderResumePending, fmt.Sprintf("paused (%s); evolve loop --resume can resume it", ev.Checkpoint)
}

func (j holderJudge) resumable(st holderState) (string, error) {
	if st.path == "" {
		return "", errors.New("no cycle state")
	}
	rp, err := core.CheckpointResumable(st.path, j.projectRoot, core.ResumeOptions{})
	switch {
	case err != nil:
		return "", err
	case st.perRun && !core.IsResumableReason(rp.Reason):
		return "", fmt.Errorf("%w: loop --resume skips a per-run checkpoint with reason %q", core.ErrNoCheckpoint, rp.Reason)
	}
	return rp.Reason, nil
}

func (j holderJudge) lease(cycle int) (bool, string) {
	l, ok, err := runlease.Read(j.runDir(cycle))
	switch {
	case err != nil:
		return false, "unreadable: " + err.Error()
	case !ok:
		return false, "none"
	case runlease.OwnerLive(l, j.now, runlease.DefaultTTL, runlease.PIDAlive):
		return true, fmt.Sprintf("live: pid %d, heartbeat %s", l.OwnerPID, l.HeartbeatAt)
	}
	return false, fmt.Sprintf("stale: pid %d, heartbeat %s", l.OwnerPID, l.HeartbeatAt)
}

func (j holderJudge) state(cycle int) holderState {
	perRun := filepath.Join(j.runDir(cycle), core.CycleStateFile)
	if st, ok := readHolderState(perRun); ok {
		st.path, st.perRun = perRun, true
		return st
	}
	primary := core.ResolveCycleStatePath(j.evolveDir)
	if st, ok := readHolderState(primary); ok && st.CycleID == cycle {
		st.path = primary
		return st
	}
	return holderState{}
}

func readHolderState(path string) (holderState, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return holderState{}, false
	}
	var st holderState
	if json.Unmarshal(raw, &st) != nil {
		return holderState{}, false
	}
	return st, true
}

func (j holderJudge) readGoals() map[int]string {
	entries, err := os.ReadDir(filepath.Join(j.evolveDir, "runs"))
	if err != nil {
		return nil
	}
	goals := map[int]string{}
	for _, e := range entries {
		cycle, err := strconv.Atoi(strings.TrimPrefix(e.Name(), "cycle-"))
		if err != nil || !e.IsDir() || !strings.HasPrefix(e.Name(), "cycle-") {
			continue
		}
		if st, ok := readHolderState(filepath.Join(j.runDir(cycle), core.CycleStateFile)); ok && st.GoalHash != "" {
			goals[cycle] = st.GoalHash
		}
	}
	return goals
}

func (j holderJudge) supersededBy(cycle int, goal string) int {
	first := 0
	for other, otherGoal := range j.goals {
		if other > cycle && otherGoal != goal && (first == 0 || other < first) {
			first = other
		}
	}
	return first
}
