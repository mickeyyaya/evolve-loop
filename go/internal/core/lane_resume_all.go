package core

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
)

type PausedScope struct {
	Cycle           int
	Scope           []string
	Phase           string
	StatePath       string
	AutoAttempts    int
	AutoMaxAttempts int
	QuotaResetAt    string
}

func PausedLaneScopes(projectRoot, evolveDir string, opts ResumeOptions) ([]PausedScope, error) {
	states, err := readPerRunStates(evolveDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	opts = opts.withDefaults().withMaxAge(evolveDir)
	ships := shipsOrWarn(evolveDir, opts)
	var paused []PausedScope
	for key, last := range newestPerScope(states) {
		if rp, ok := laneResumable(last, ships, projectRoot, opts); ok {
			paused = append(paused, PausedScope{Cycle: rp.CycleID, Scope: strings.Split(key, ","), Phase: rp.Phase, StatePath: rp.StatePath,
				AutoAttempts: rp.AutoAttempts, AutoMaxAttempts: rp.AutoMaxAttempts, QuotaResetAt: rp.QuotaResetAt})
		}
	}
	slices.SortFunc(paused, func(a, b PausedScope) int { return a.Cycle - b.Cycle })
	return paused, nil
}
