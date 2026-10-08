package inboxmover

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
)

var (
	ErrClaimHeld     = errors.New("inboxmover: the holding cycle can still use its claim")
	ErrClaimConflict = lifecycle.ErrClaimConflict
)

type HeldClaim struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Duplicate bool   `json:"duplicate"`
	Holder    Holder `json:"holder"`
}

type EmptyClaimDir struct {
	Path   string `json:"path"`
	Holder Holder `json:"holder"`
}

type ClaimSurvey struct {
	Claims    []HeldClaim     `json:"claims"`
	EmptyDirs []EmptyClaimDir `json:"empty_dirs"`
}

type ClaimOutcome string

const (
	ClaimReleased         ClaimOutcome = "released"
	ClaimDuplicateRemoved ClaimOutcome = "duplicate-removed"
	ClaimNotHeld          ClaimOutcome = "not-claimed"
)

type ClaimRelease struct {
	ID      string       `json:"id"`
	Cycle   int          `json:"cycle"`
	Outcome ClaimOutcome `json:"outcome"`
	Path    string       `json:"path"`
	Holder  Holder       `json:"holder"`
}

func SurveyClaims(opts Options) (ClaimSurvey, error) {
	opts.resolveOpts()
	list, err := lifecycle.ListClaims(opts.InboxDir)
	if err != nil {
		return ClaimSurvey{}, err
	}
	judge := newHolderJudge(opts)
	survey := ClaimSurvey{Claims: []HeldClaim{}, EmptyDirs: []EmptyClaimDir{}}
	for _, it := range list.Items {
		survey.Claims = append(survey.Claims, HeldClaim{ID: it.ID, Path: it.Path, Duplicate: it.Duplicate, Holder: judge.classify(it.Cycle)})
	}
	for _, dir := range list.EmptyDirs {
		survey.EmptyDirs = append(survey.EmptyDirs, EmptyClaimDir{Path: dir.Path, Holder: judge.classify(dir.Cycle)})
	}
	return survey, nil
}

func ReleaseClaim(opts Options, taskID, reason string) (ClaimRelease, error) {
	opts.resolveOpts()
	taskID, reason = strings.TrimSpace(taskID), strings.TrimSpace(reason)
	if taskID == "" || reason == "" {
		return ClaimRelease{}, fmt.Errorf("%w: release requires a task id and a reason", ErrBadArgs)
	}
	loc, err := Locate(opts.InboxDir, taskID)
	if err != nil {
		return ClaimRelease{}, err
	}
	if loc.Cycle == 0 {
		return ClaimRelease{ID: taskID, Outcome: ClaimNotHeld, Path: loc.Path}, nil
	}
	holder := newHolderJudge(opts).classify(loc.Cycle)
	if holder.Keeps() {
		return ClaimRelease{}, fmt.Errorf("%w: %s is held by cycle %d (%s: %s)", ErrClaimHeld, taskID, loc.Cycle, holder.Verdict, holder.Reason)
	}
	return releaseHeld(opts, taskID, loc, holder, reason)
}

func ReleaseStaleClaims(opts Options, reason string) ([]ClaimRelease, error) {
	opts.resolveOpts()
	list, err := lifecycle.ListClaims(opts.InboxDir)
	if err != nil {
		return nil, err
	}
	judge := newHolderJudge(opts)
	released := []ClaimRelease{}
	var errs []error
	for _, it := range list.Items {
		holder := judge.classify(it.Cycle)
		if holder.Keeps() {
			continue
		}
		rel, err := releaseHeld(opts, it.ID, it.Location, holder, reason+": "+holder.Reason)
		if err != nil {
			errs = append(errs, fmt.Errorf("release %s from cycle %d: %w", it.ID, it.Cycle, err))
			continue
		}
		released = append(released, rel)
	}
	return released, errors.Join(errs...)
}

func releaseHeld(opts Options, taskID string, loc Location, holder Holder, reason string) (ClaimRelease, error) {
	res, err := opts.mover().ReleaseClaim(taskID, loc, reason)
	if err != nil {
		return ClaimRelease{}, err
	}
	outcome := ClaimReleased
	if res.Duplicate {
		outcome = ClaimDuplicateRemoved
	}
	return ClaimRelease{ID: taskID, Cycle: loc.Cycle, Outcome: outcome, Path: res.Path, Holder: holder}, nil
}

type Absorbed = lifecycle.Absorbed

const (
	AbsorbDropped = lifecycle.AbsorbDropped
	AbsorbParked  = lifecycle.AbsorbParked
)

func AbsorbRootCopies(opts Options) ([]Absorbed, error) {
	return opts.mover().AbsorbRootCopies()
}
