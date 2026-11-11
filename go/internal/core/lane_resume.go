package core

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const laneQuotaReason = "quota-likely"

const laneStoppedReason = "operator-requested"

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
	last, found := newestPerScope(states)[strings.Join(want, ",")]
	if !found {
		return nil, false
	}
	opts = opts.withDefaults().withMaxAge(evolveDir)
	return laneResumable(last, shipsOrWarn(evolveDir, opts), projectRoot, opts)
}

func newestPerScope(states []perRunState) map[string]perRunState {
	newest := map[string]perRunState{}
	for _, st := range states {
		ids := normalizedIDs(LaneScopeIDs(filepath.Dir(st.path)))
		if len(ids) == 0 {
			continue
		}
		key := strings.Join(ids, ",")
		if prev, ok := newest[key]; !ok || st.cycle > prev.cycle {
			newest[key] = st
		}
	}
	return newest
}

func shipsOrWarn(evolveDir string, opts ResumeOptions) map[int]time.Time {
	ships, err := landedShips(evolveDir)
	if err != nil {
		fmt.Fprintf(opts.Log, "[resume] WARN: the ledger scan stopped early (%v); a later ship can be missed\n", err)
	}
	return ships
}

func laneResumable(last perRunState, ships map[int]time.Time, projectRoot string, opts ResumeOptions) (*ResumePoint, bool) {
	if _, shipped := ships[last.cycle]; shipped || !laneQuotaPaused(last) {
		return nil, false
	}
	rp, err := loadResumeStateFrom(last.path, projectRoot, opts)
	if err != nil {
		fmt.Fprintf(opts.Log, "[resume] lane %s: cycle %d checkpoint is not resumable: %v\n", strings.Join(normalizedIDs(LaneScopeIDs(filepath.Dir(last.path))), ","), last.cycle, err)
		return nil, false
	}
	return rp, true
}

func laneQuotaPaused(st perRunState) bool {
	return st.enabled && (st.reason == laneQuotaReason || st.reason == laneStoppedReason) && !st.leased && !IsTerminalPhase(st.phase)
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
