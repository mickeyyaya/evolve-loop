package integration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	PrivateMainRef = "refs/evolve/integration-main"

	fieldPool   = "pool"
	fieldShadow = "shadow"
	fieldBatch  = "batch"
	fieldCycle  = "cycle"
	shadowTrue  = "true"
	actorOrigin = "integration.ShadowActor"
)

type Publisher interface {
	Emit(signalcenter.Event)
}

type PendingBatch struct {
	Pool    int
	Index   int
	Base    string
	Trigger FlushTrigger
	Members []DeliverableRecord
}

type MemberResult struct {
	Status MemberStatus
	Landed bool
}

type FlushResult struct {
	Batch    IntegrationBatch
	Members  map[int]MemberResult
	Failures []DriverFailure
}

type Actor interface {
	Resume(ctx context.Context, rec WaveRecord) (WaveRecord, error)
	Flush(ctx context.Context, b PendingBatch) (FlushResult, error)
	WaveEnd(ctx context.Context, rec WaveRecord) (WaveRecord, error)
}

type ShadowActor struct {
	Composer Composer
	Records  func() RecordSet
	Refresh  func(ctx context.Context) error
	Publish  Publisher
	Now      func() time.Time
}

func (a ShadowActor) Resume(ctx context.Context, rec WaveRecord) (WaveRecord, error) {
	return rec, a.dropStale(ctx, rec.Pool)
}

func (a ShadowActor) dropStale(ctx context.Context, pool int) error {
	out, err := a.Composer.Repo.Git(ctx, a.Composer.Repo.Dir, "", "for-each-ref", "--format=%(refname)", branchRef("integration/p"+strconv.Itoa(pool)+"-*"))
	if err != nil {
		return fmt.Errorf("integration: list the stale refs of pool %d: %w", pool, err)
	}
	var errs []error
	for _, ref := range strings.Fields(out) {
		errs = append(errs, a.deleteRef(ctx, ref))
	}
	return errors.Join(errs...)
}

func (a ShadowActor) deleteRef(ctx context.Context, ref string) error {
	if _, err := a.Composer.Repo.Git(ctx, a.Composer.Repo.Dir, "", "update-ref", "-d", "--end-of-options", ref); err != nil {
		return fmt.Errorf("integration: delete %s: %w", ref, err)
	}
	return nil
}

func (a ShadowActor) Flush(ctx context.Context, b PendingBatch) (FlushResult, error) {
	var res FlushResult
	res.Failures = a.refresh(ctx)
	cur, err := a.Composer.Repo.StartBatch(ctx, Batch{Wave: strconv.Itoa(b.Pool), Pool: b.Pool, Index: b.Index, Base: b.Base})
	if err != nil {
		return FlushResult{}, err
	}
	res.Members = make(map[int]MemberResult, len(b.Members))
	var cycles []int
	for _, rec := range b.Members {
		landed, lerr := a.landed(ctx, rec)
		res.Failures = a.noteFailure(res.Failures, "landed", lerr)
		next, status := a.compose(ctx, cur, b, rec)
		if status.err != nil {
			res.Failures = a.noteFailure(res.Failures, "compose", status.err)
		}
		cur = next
		res.Members[rec.Cycle] = MemberResult{Status: status.status, Landed: landed}
		cycles = append(cycles, rec.Cycle)
	}
	res.Batch = IntegrationBatch{Index: b.Index, Base: b.Base, Branch: cur.Branch, Tip: cur.Tip, Members: cycles}
	return res, nil
}

type composeStatus struct {
	status MemberStatus
	err    error
}

func (a ShadowActor) compose(ctx context.Context, cur Batch, b PendingBatch, rec DeliverableRecord) (Batch, composeStatus) {
	st, err := a.Composer.Compose(ctx, cur, rec)
	var ej *Ejection
	switch {
	case errors.As(err, &ej):
		status := a.conflictStatus(ctx, b.Base, rec)
		a.emit(signalcenter.Event{
			Cycle: rec.Cycle, Kind: signalcenter.KindShipWarning, Code: CodeEjected, Severity: signalcenter.SeverityWarn,
			Reason: fmt.Sprintf("shadow: %s would be ejected on a %s; no brief is written and no lane is told", ej.Label(), ej.Cause),
			Fields: map[string]string{FieldCause: ej.Cause, FieldPaths: strings.Join(ej.Paths, ","), fieldShadow: shadowTrue, fieldCycle: strconv.Itoa(rec.Cycle), "status": string(status)},
		})
		return cur, composeStatus{status: status}
	case err != nil:
		return cur, composeStatus{status: MemberFailed, err: fmt.Errorf("compose cycle %d: %w", rec.Cycle, err)}
	}
	a.emit(signalcenter.Event{
		Cycle: rec.Cycle, Kind: signalcenter.KindShipDelivered, Code: CodeComposed, Severity: signalcenter.SeverityInfo,
		Reason: fmt.Sprintf("shadow: cycle %d composed onto %s", rec.Cycle, st.Batch.Branch),
		Fields: map[string]string{fieldBatch: st.Batch.Branch, fieldCycle: strconv.Itoa(rec.Cycle), fieldShadow: shadowTrue},
	})
	return st.Batch, composeStatus{status: MemberComposed}
}

func (a ShadowActor) conflictStatus(ctx context.Context, chainBase string, rec DeliverableRecord) MemberStatus {
	ok, err := a.Composer.Repo.IsAncestor(ctx, rec.Commits[0].BaseSHA, chainBase)
	if err == nil && !ok {
		return MemberConflictedBaseAhead
	}
	return MemberConflicted
}

func (a ShadowActor) landed(ctx context.Context, rec DeliverableRecord) (bool, error) {
	if len(rec.Commits) == 0 {
		return false, nil
	}
	for _, c := range rec.Commits {
		ok, err := a.Composer.Repo.IsAncestor(ctx, c.SHA, PrivateMainRef)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func (a ShadowActor) refresh(ctx context.Context) []DriverFailure {
	if a.Refresh == nil {
		return nil
	}
	return a.noteFailure(nil, "refresh", a.Refresh(ctx))
}

func (a ShadowActor) noteFailure(into []DriverFailure, step string, err error) []DriverFailure {
	if err == nil {
		return into
	}
	return append(into, DriverFailure{Step: step, Error: err.Error(), At: a.now()})
}

func (a ShadowActor) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a ShadowActor) emit(ev signalcenter.Event) { emitEvent(a.Publish, actorOrigin, ev) }

func emitEvent(p Publisher, origin string, ev signalcenter.Event) {
	if p == nil {
		return
	}
	ev.Module, ev.Origin = signalcenter.ModuleShip, origin
	p.Emit(ev)
}

func (a ShadowActor) WaveEnd(ctx context.Context, rec WaveRecord) (WaveRecord, error) {
	out := rec
	out.Members = make(map[int]MemberState, len(rec.Members))
	for cycle, m := range rec.Members {
		out.Members[cycle] = m
	}
	failures := a.refresh(ctx)
	set := a.listed()
	for cycle, m := range out.Members {
		listed, found := set.first(func(r DeliverableRecord) bool { return !r.Console() && MemberKey(r) == memberKey(cycle, m.PatchIDs) })
		if !found {
			continue
		}
		landed, err := a.landed(ctx, listed)
		failures = a.noteFailure(failures, "landed", err)
		m.Landed = landed
		out.Members[cycle] = m
	}
	var dropErrs []error
	for _, b := range rec.Batches {
		dropErrs = append(dropErrs, a.deleteRef(ctx, branchRef(b.Branch)))
	}
	out.Failures = append(slices.Clone(out.Failures), failures...)
	return out, errors.Join(dropErrs...)
}

func (a ShadowActor) listed() RecordSet {
	if a.Records == nil {
		return RecordSet{}
	}
	return a.Records()
}

func ShadowComposer(repo Repo) Composer {
	return Composer{Repo: repo, Stage: policy.FleetIntegrationShadow}
}
