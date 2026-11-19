package integration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const enforceOrigin = "integration.EnforceActor"

type PlaneSync interface {
	Sync(ctx context.Context) (SyncOutcome, error)
}

type LandingFeed interface {
	NoteEjected(ctx context.Context, cycle int, briefPath string) error
}

type Requeuer interface {
	Requeue(ctx context.Context, rec DeliverableRecord, briefPath string) error
}

type BriefStore interface {
	Write(ctx context.Context, ej *Ejection) (string, error)
}

type Promoter interface {
	Promote(ctx context.Context, rec DeliverableRecord) error
}

type EnforceActor struct {
	Repo    Repo
	Merger  Merger
	Plane   PlaneSync
	Landing LandingFeed
	Requeue Requeuer
	Briefs  BriefStore
	Promote Promoter
	Publish Publisher
	Now     func() time.Time
}

func (a EnforceActor) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a EnforceActor) Resume(ctx context.Context, rec WaveRecord) (WaveRecord, error) {
	if a.Repo.Git == nil {
		return rec, nil
	}
	return rec, dropPoolRefs(ctx, a.Repo, rec.Pool)
}

func (a EnforceActor) Flush(ctx context.Context, b PendingBatch) (FlushResult, error) {
	res := FlushResult{Members: make(map[int]MemberResult, len(b.Members))}
	byCycle := make(map[int]DeliverableRecord, len(b.Members))
	cycles := make([]int, 0, len(b.Members))
	for _, rec := range b.Members {
		byCycle[rec.Cycle] = rec
		cycles = append(cycles, rec.Cycle)
		res.Members[rec.Cycle] = MemberResult{Status: MemberPending}
	}

	var lastBatch Batch
	m := a.Merger
	origCompose := m.Compose
	m.Compose = func(ctx context.Context, recs []DeliverableRecord) (Composition, error) {
		comp, err := origCompose(ctx, recs)
		if err == nil && len(comp.Kept) > 0 {
			lastBatch = comp.Batch
		}
		return comp, err
	}
	ejected := map[int]bool{}
	if m.Eject == nil {
		m.Eject = func(ctx context.Context, rec DeliverableRecord, cause error) error {
			ejected[rec.Cycle] = true
			res.Members[rec.Cycle] = MemberResult{Status: MemberEjected}
			return a.ejectMember(ctx, rec, cause, &res)
		}
	}

	out, landErr := m.Land(ctx, b.Members)
	for _, ref := range out.Ejected {
		if _, ok := byCycle[ref]; ok && !ejected[ref] {
			res.Members[ref] = MemberResult{Status: MemberEjected}
		}
	}
	prNum, promoteErr := a.recordLanded(ctx, out, byCycle, &res)
	res.Batch = IntegrationBatch{Index: b.Index, Base: b.Base, Branch: lastBatch.Branch, Tip: lastBatch.Tip, PR: prNum, Members: cycles}
	return res, errors.Join(landErr, promoteErr)
}

func (a EnforceActor) recordLanded(ctx context.Context, out Outcome, byCycle map[int]DeliverableRecord, res *FlushResult) (int, error) {
	var promoteErr error
	prNum := 0
	for _, ref := range slices.Concat(out.Landed, out.OnMain) {
		rec, ok := byCycle[ref]
		if !ok {
			continue
		}
		if prNum == 0 && out.Via != nil {
			prNum = out.Via[ref]
		}
		res.Members[rec.Cycle] = MemberResult{Status: MemberMerged, Landed: true}
		if a.Promote != nil {
			if err := a.Promote.Promote(ctx, rec); err != nil {
				res.Failures = append(res.Failures, DriverFailure{Step: "promote", Error: err.Error(), At: a.now()})
				promoteErr = errors.Join(promoteErr, err)
			}
		}
	}
	return prNum, promoteErr
}

func (a EnforceActor) ejectMember(ctx context.Context, rec DeliverableRecord, cause error, res *FlushResult) error {
	var ej *Ejection
	if !errors.As(cause, &ej) {
		ej = &Ejection{Cycle: rec.Cycle, PR: rec.PR, Cause: CauseGate, Err: cause}
	}
	var briefPath string
	if a.Briefs != nil {
		path, err := a.Briefs.Write(ctx, ej)
		if err != nil {
			res.Failures = append(res.Failures, DriverFailure{Step: "brief", Error: err.Error(), At: a.now()})
			return err
		}
		briefPath = path
	}
	emitEvent(a.Publish, enforceOrigin, signalcenter.Event{
		Cycle: rec.Cycle, Kind: signalcenter.KindShipWarning, Code: CodeEjected, Severity: signalcenter.SeverityWarn,
		Reason: fmt.Sprintf("%s is ejected on a %s", ej.Label(), ej.Cause),
		Fields: ej.Fields(briefPath),
	})
	if a.Requeue != nil {
		if err := a.Requeue.Requeue(ctx, rec, briefPath); err != nil {
			res.Failures = append(res.Failures, DriverFailure{Step: "requeue", Error: err.Error(), At: a.now()})
			return err
		}
	}
	if a.Landing != nil {
		if err := a.Landing.NoteEjected(ctx, rec.Cycle, briefPath); err != nil {
			res.Failures = append(res.Failures, DriverFailure{Step: "landing", Error: err.Error(), At: a.now()})
			return err
		}
	}
	return nil
}

func (a EnforceActor) WaveEnd(ctx context.Context, rec WaveRecord) (WaveRecord, error) {
	out := cloneRecord(rec)
	var errs []error
	if a.Plane != nil {
		outcome, err := a.Plane.Sync(ctx)
		if err != nil {
			out.Failures = appendFailures(out.Failures, DriverFailure{Step: "plane sync", Error: err.Error(), At: a.now()})
			errs = append(errs, err)
		} else if outcome == SyncBlocked || outcome == SyncDiverged {
			out.Failures = appendFailures(out.Failures, DriverFailure{Step: "plane sync", Error: fmt.Sprintf("plane sync %s", outcome), At: a.now()})
		}
	}
	for cycle, m := range out.Members {
		if m.Status == MemberPending {
			err := fmt.Errorf("integration: member %d still pending at wave end", cycle)
			out.Failures = appendFailures(out.Failures, DriverFailure{Step: stepWaveEnd, Error: err.Error(), At: a.now()})
			errs = append(errs, err)
		}
	}
	if a.Repo.Git != nil {
		for _, b := range rec.Batches {
			errs = append(errs, deleteBranchRef(ctx, a.Repo, branchRef(b.Branch)))
		}
	}
	return out, errors.Join(errs...)
}

func dropPoolRefs(ctx context.Context, repo Repo, pool int) error {
	out, err := repo.Git(ctx, repo.Dir, "", "for-each-ref", "--format=%(refname)", branchRef("integration/p"+strconv.Itoa(pool)+"-*"))
	if err != nil {
		return fmt.Errorf("integration: list the stale refs of pool %d: %w", pool, err)
	}
	var errs []error
	for _, ref := range strings.Fields(out) {
		errs = append(errs, deleteBranchRef(ctx, repo, ref))
	}
	return errors.Join(errs...)
}

func deleteBranchRef(ctx context.Context, repo Repo, ref string) error {
	if _, err := repo.Git(ctx, repo.Dir, "", "update-ref", "-d", "--end-of-options", ref); err != nil {
		return fmt.Errorf("integration: delete %s: %w", ref, err)
	}
	return nil
}
