package integration

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

type PRState string

const stateOpen PRState = "OPEN"

const (
	sourcePRKey   = "Source-PR: #"
	batchSettled  = "Every PR in this batch is already on main or closed. This batch PR is closed. Its branch is unchanged."
	settleLanded  = "is on main: closed, record spent"
	settleClosed  = "is closed on the host: record spent"
	settleBatchPR = "holds only PRs that are on main or closed: closed"
)

type Settlement struct {
	PR    int
	Moved []string
	kind  string
}

func (s Settlement) Line() string {
	switch s.kind {
	case settleBatchPR:
		return fmt.Sprintf("settle: batch PR #%d %s", s.PR, s.kind)
	}
	return fmt.Sprintf("settle: PR #%d %s", s.PR, s.kind)
}

func (t Train) Settle(ctx context.Context) ([]Settlement, error) {
	var done []Settlement
	var errs []error
	gone := map[int]bool{}
	for _, rec := range t.consoleRecords() {
		s, err := t.settleRecord(ctx, rec)
		errs = append(errs, err)
		if s != nil {
			done = append(done, *s)
			gone[s.PR] = true
		}
	}
	batches, err := t.settleBatches(ctx, gone)
	return append(done, batches...), errors.Join(append(errs, err)...)
}

func (t Train) consoleRecords() []DeliverableRecord {
	var recs []DeliverableRecord
	for _, l := range ListRecords(t.Root).Listed {
		if l.Record.Console() {
			recs = append(recs, l.Record)
		}
	}
	slices.SortFunc(recs, func(a, b DeliverableRecord) int { return cmp.Compare(a.PR, b.PR) })
	return recs
}

func (t Train) settleRecord(ctx context.Context, rec DeliverableRecord) (*Settlement, error) {
	state, err := t.Host.State(ctx, rec.PR)
	if err != nil {
		return nil, err
	}
	if state != stateOpen {
		if err := t.spend(rec); err != nil {
			return nil, err
		}
		return &Settlement{PR: rec.PR, kind: settleClosed}, nil
	}
	landed, err := t.landedOnMain(ctx, rec)
	if err != nil || !landed {
		return nil, err
	}
	moved, err := t.complete(ctx, rec, func() error { return t.Host.Close(ctx, rec.PR, alreadyLanded) })
	if err != nil {
		return nil, err
	}
	return &Settlement{PR: rec.PR, Moved: moved, kind: settleLanded}, nil
}

func (t Train) landedOnMain(ctx context.Context, rec DeliverableRecord) (bool, error) {
	if err := t.sync(ctx, []int{rec.PR}); err != nil {
		return false, err
	}
	moved, err := t.headEjection(ctx, rec, false)
	if err != nil || moved != nil {
		return false, err
	}
	return t.Composer.Repo.CheckLanded(ctx, rec, refMain)
}

func (t Train) spend(rec DeliverableRecord) error {
	if err := os.Remove(PRRecordPath(WaveDir(t.Root, ConsoleWave), rec.PR)); err != nil {
		return fmt.Errorf("integration: spend the record of PR #%d: %w", rec.PR, err)
	}
	return nil
}

func (t Train) settleBatches(ctx context.Context, gone map[int]bool) ([]Settlement, error) {
	batches, err := t.Host.OpenBatches(ctx)
	if err != nil {
		return nil, err
	}
	var done []Settlement
	var errs []error
	for _, b := range batches {
		closed, err := t.settleBatch(ctx, b, gone)
		errs = append(errs, err)
		if closed {
			done = append(done, Settlement{PR: b, kind: settleBatchPR})
		}
	}
	return done, errors.Join(errs...)
}

func (t Train) settleBatch(ctx context.Context, batch int, gone map[int]bool) (bool, error) {
	members, err := t.batchMembers(ctx, batch)
	if err != nil || len(members) == 0 {
		return false, err
	}
	for _, m := range members {
		if gone[m] {
			continue
		}
		state, err := t.Host.State(ctx, m)
		if err != nil || state == stateOpen {
			return false, err
		}
	}
	if err := t.Host.Close(ctx, batch, batchSettled); err != nil {
		return false, err
	}
	return true, nil
}

func (t Train) batchMembers(ctx context.Context, batch int) ([]int, error) {
	if err := t.sync(ctx, []int{batch}); err != nil {
		return nil, err
	}
	git := t.Composer.Repo
	log, err := git.Git(ctx, git.Dir, "", "log", "-z", "--format=%B", "--end-of-options", refMain+"..refs/evolve/pr/"+strconv.Itoa(batch))
	if err != nil {
		return nil, fmt.Errorf("integration: list the commits of batch PR #%d: %w", batch, err)
	}
	var members []int
	for _, message := range strings.Split(log, logEntrySep) {
		if strings.TrimSpace(message) == "" {
			continue
		}
		prs := sourcePRs(message)
		if len(prs) == 0 {
			return nil, nil
		}
		members = append(members, prs...)
	}
	return members, nil
}

func sourcePRs(message string) []int {
	var prs []int
	for _, l := range strings.Split(message, "\n") {
		num, ok := strings.CutPrefix(strings.TrimSpace(l), sourcePRKey)
		if n, err := strconv.Atoi(num); ok && err == nil && n > 0 {
			prs = append(prs, n)
		}
	}
	return prs
}
