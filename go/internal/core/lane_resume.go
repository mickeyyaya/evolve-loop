package core

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
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
