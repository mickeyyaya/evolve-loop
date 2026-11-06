package integration

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeDriverFailed signalcenter.Code = "SHIP_INTEGRATION_DRIVER_FAILED"

	driverOrigin  = "integration.Driver"
	fieldStep     = "step"
	fieldError    = "error"
	fieldTrigger  = "trigger"
	fieldPR       = "pr"
	stepStage     = "stage"
	stepEpoch     = "epoch"
	stepRead      = "read"
	stepResume    = "resume"
	stepFlush     = "flush"
	stepWaveEnd   = "wave end"
	stepWrite     = "write"
	stepBase      = "base"
	stepLast      = "last cycle"
	stepRecord    = "record"
	stepPanic     = "panic"
	maxFlushPass  = 1024
	driverFailDoc = "the integration driver failed a step; fields.step names the step and fields.error the error, and the driver also writes the failure into the wave record, so a lost signal cannot hide it"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeDriverFailed, driverFailDoc)
}

type WaveStore interface {
	Read(pool int) (WaveRecord, bool, error)
	Write(WaveRecord) error
	Pools() []int
}

type FileWaveStore struct{ Root string }

func (s FileWaveStore) Read(pool int) (WaveRecord, bool, error) {
	rec, err := ReadWaveRecord(WaveRecordPath(s.Root, pool))
	if errors.Is(err, fs.ErrNotExist) {
		return WaveRecord{}, false, nil
	}
	return rec, err == nil, err
}

func (s FileWaveStore) Pools() []int {
	paths, err := filepath.Glob(WaveRecordPath(s.Root, 0)[:len(WaveRecordPath(s.Root, 0))-len("0.json")] + "*.json")
	if err != nil {
		return nil
	}
	var pools []int
	for _, p := range paths {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), wavePrefix), ".json")
		if n, err := strconv.Atoi(name); err == nil && n > 0 {
			pools = append(pools, n)
		}
	}
	slices.Sort(pools)
	return pools
}

func (s FileWaveStore) Write(rec WaveRecord) error {
	_, err := WriteWaveRecord(s.Root, rec)
	return err
}

type Driver struct {
	Pool       int
	Holder     uint64
	ShadowBase string
	Startup    []DriverFailure
	Stage      func() (string, error)
	Actors     map[string]Actor
	Epoch      EpochReader
	Store      WaveStore
	Records    func() RecordSet
	Flush      FlushPolicy
	Last       func(ctx context.Context) (int, error)
	Now        func() time.Time
	Publish    Publisher

	rec     *WaveRecord
	written WaveRecord
	resumed bool
	stopped error
	settled map[int]map[string]MemberStatus
}

type heldAt struct {
	pool    int
	pending bool
}

type stepError struct {
	step string
	err  error
}

func (e *stepError) Error() string { return e.step + ": " + e.err.Error() }
func (e *stepError) Unwrap() error { return e.err }

func (d *Driver) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func (d *Driver) Check() error {
	_, err := d.actor()
	return err
}

func (d *Driver) actor() (Actor, error) {
	stage, err := d.Stage()
	if err != nil {
		return nil, &stepError{stepStage, err}
	}
	a, ok := d.Actors[stage]
	if !ok {
		return nil, &stepError{stepStage, fmt.Errorf("the stage %q has no actor, so the driver does not start", stage)}
	}
	return a, nil
}

func (d *Driver) Tick(ctx context.Context) error { return d.run(ctx, d.flushOnce) }

func (d *Driver) EndWave(ctx context.Context) error { return d.run(ctx, d.drainAndEnd) }

func (d *Driver) run(ctx context.Context, work func(context.Context, Actor) error) error {
	if d.stopped != nil {
		return d.stopped
	}
	err := d.step(ctx, work)
	if err != nil {
		d.fail(err)
	}
	return err
}

func (d *Driver) step(ctx context.Context, work func(context.Context, Actor) error) error {
	actor, err := d.actor()
	if err != nil {
		return err
	}
	if err := d.checkEpoch(); err != nil {
		return err
	}
	if err := d.load(); err != nil {
		return err
	}
	if err := d.resume(ctx, actor); err != nil {
		return err
	}
	d.admit()
	return errors.Join(work(ctx, actor), d.persist())
}

func (d *Driver) drainAndEnd(ctx context.Context, actor Actor) error {
	if err := d.drain(ctx, actor); err != nil {
		return err
	}
	return d.endWave(ctx, actor)
}

func (d *Driver) checkEpoch() error {
	cur, err := d.Epoch.Current()
	if err != nil {
		return &stepError{stepEpoch, err}
	}
	if cur != d.Holder {
		d.stopped = &stepError{stepEpoch, fmt.Errorf("%w: holder %d, file %d", ErrStaleEpoch, d.Holder, cur)}
		return d.stopped
	}
	return nil
}

func (d *Driver) load() error {
	if d.rec != nil {
		return nil
	}
	rec, found, err := d.Store.Read(d.Pool)
	if err != nil {
		return &stepError{stepRead, err}
	}
	if !found {
		if d.ShadowBase == "" {
			return &stepError{stepBase, errors.New("the pool has no shadow base, so the driver writes no record")}
		}
		rec = WaveRecord{Pool: d.Pool, ShadowBase: d.ShadowBase, Members: map[int]MemberState{}}
	}
	if rec.Epoch > d.Holder {
		d.stopped = &stepError{stepEpoch, fmt.Errorf("%w: holder %d, record %d", ErrStaleEpoch, d.Holder, rec.Epoch)}
		return d.stopped
	}
	d.written = WaveRecord{}
	if found {
		d.written = cloneRecord(rec)
	}
	rec.Epoch = d.Holder
	if rec.Members == nil {
		rec.Members = map[int]MemberState{}
	}
	rec.Failures = appendFailures(rec.Failures, d.Startup...)
	d.rec = &rec
	d.settled = map[int]map[string]MemberStatus{}
	return nil
}

func (d *Driver) resume(ctx context.Context, actor Actor) error {
	if d.resumed {
		return nil
	}
	rec, err := actor.Resume(ctx, *d.rec)
	if err != nil {
		return &stepError{stepResume, err}
	}
	d.rec, d.resumed = &rec, true
	return nil
}

func appendFailures(into []DriverFailure, add ...DriverFailure) []DriverFailure {
	for _, f := range add {
		if !slices.ContainsFunc(into, func(g DriverFailure) bool { return g.Step == f.Step && g.Error == f.Error }) {
			into = append(into, f)
		}
	}
	return into
}

func (d *Driver) recordSet() RecordSet {
	if d.Records == nil {
		return RecordSet{}
	}
	return d.Records()
}

func (d *Driver) admit() {
	set := d.recordSet()
	for _, f := range set.Failures {
		if f.Concerns(d.Pool) {
			d.rec.Failures = appendFailures(d.rec.Failures, DriverFailure{Step: stepRecord, Error: f.Path + ": " + f.Err.Error(), At: d.now()})
		}
	}
	d.admitMembers(set)
}

func (d *Driver) admitMembers(set RecordSet) {
	var held map[string]heldAt
	for _, l := range set.Owned() {
		d.noteRedelivered(l)
		if !d.admits(l) {
			continue
		}
		if l.Pool < d.Pool && held == nil {
			held = d.heldElsewhere()
		}
		if d.mine(l, held) {
			d.rec.Members[l.Record.Cycle] = MemberState{Status: MemberPending, PatchIDs: patchIDsOf(l.Record)}
		}
	}
}

func (d *Driver) noteRedelivered(l ListedRecord) {
	m, known := d.rec.Members[l.Record.Cycle]
	if known && l.Pool == d.Pool && m.Status != MemberPending && memberKey(l.Record.Cycle, m.PatchIDs) != MemberKey(l.Record) {
		d.rec.Failures = appendFailures(d.rec.Failures, DriverFailure{Step: stepRecord, Error: fmt.Sprintf("cycle %d was delivered again under new patches after it was %s", l.Record.Cycle, m.Status), At: d.now()})
	}
}

func (d *Driver) admits(l ListedRecord) bool {
	m, known := d.rec.Members[l.Record.Cycle]
	if !known {
		return true
	}
	return l.Pool == d.Pool && m.Status == MemberPending && memberKey(l.Record.Cycle, m.PatchIDs) != MemberKey(l.Record)
}

func (d *Driver) mine(l ListedRecord, held map[string]heldAt) bool {
	switch {
	case l.Pool == d.Pool:
		return true
	case l.Pool > d.Pool:
		return false
	}
	h, ok := held[MemberKey(l.Record)]
	return ok && h.pool >= l.Pool && h.pending
}

func (d *Driver) heldElsewhere() map[string]heldAt {
	held := map[string]heldAt{}
	for _, p := range d.olderPools() {
		for key, status := range d.statusesOf(p) {
			held[key] = heldAt{pool: p, pending: status == MemberPending}
		}
	}
	return held
}

func (d *Driver) statusesOf(pool int) map[string]MemberStatus {
	if cached, ok := d.settled[pool]; ok {
		return cached
	}
	rec, found, err := d.Store.Read(pool)
	if err != nil || !found {
		return nil
	}
	out := make(map[string]MemberStatus, len(rec.Members))
	settled := rec.Ended
	for cycle, m := range rec.Members {
		out[memberKey(cycle, m.PatchIDs)] = m.Status
		settled = settled && m.Status != MemberPending
	}
	if settled {
		d.settled[pool] = out
	}
	return out
}

func (d *Driver) olderPools() []int {
	var pools []int
	for _, p := range d.Store.Pools() {
		if p < d.Pool {
			pools = append(pools, p)
		}
	}
	slices.Sort(pools)
	return pools
}

func (d *Driver) pending(set RecordSet) []DeliverableRecord {
	var out []DeliverableRecord
	for _, l := range set.Owned() {
		m, ok := d.rec.Members[l.Record.Cycle]
		if ok && m.Status == MemberPending && memberKey(l.Record.Cycle, m.PatchIDs) == MemberKey(l.Record) {
			out = append(out, l.Record)
		}
	}
	return out
}

func idle(decision FlushDecision) bool {
	return decision.Trigger == TriggerNone && len(decision.Flush) == 0
}

func deliverablesOf(records []DeliverableRecord) []Deliverable {
	out := make([]Deliverable, len(records))
	for i, r := range records {
		out[i] = r.Deliverable()
	}
	return out
}

func (d *Driver) flushOnce(ctx context.Context, actor Actor) error {
	pending := d.pending(d.recordSet())
	decision := DecideFlush(deliverablesOf(pending), false, d.Flush, d.now())
	if idle(decision) {
		return nil
	}
	return d.flush(ctx, actor, decision, pending)
}

func (d *Driver) drain(ctx context.Context, actor Actor) error {
	for range maxFlushPass {
		pending := d.pending(d.recordSet())
		decision := DecideFlush(deliverablesOf(pending), true, d.Flush, d.now())
		if idle(decision) {
			return nil
		}
		if err := d.flush(ctx, actor, decision, pending); err != nil {
			return err
		}
		if len(decision.Hold) == 0 {
			return nil
		}
	}
	return &stepError{stepFlush, fmt.Errorf("the wave end flush did not empty the hold in %d passes", maxFlushPass)}
}

func (d *Driver) flush(ctx context.Context, actor Actor, decision FlushDecision, pending []DeliverableRecord) error {
	byID := map[string]DeliverableRecord{}
	for _, r := range pending {
		byID[r.Deliverable().ID] = r
	}
	var members []DeliverableRecord
	for _, dv := range OrderDeliverables(decision.Flush) {
		members = append(members, byID[dv.ID])
	}
	base := d.rec.ShadowBase
	if n := len(d.rec.Batches); n > 0 {
		base = d.rec.Batches[n-1].Tip
	}
	index := len(d.rec.Batches) + 1
	res, err := actor.Flush(ctx, PendingBatch{Pool: d.Pool, Index: index, Base: base, Trigger: decision.Trigger, Members: members})
	if err != nil {
		return &stepError{stepFlush, err}
	}
	d.rec.Batches = append(d.rec.Batches, res.Batch)
	for cycle, r := range res.Members {
		m := d.rec.Members[cycle]
		m.Status, m.Landed = r.Status, r.Landed
		d.rec.Members[cycle] = m
	}
	d.rec.Failures = appendFailures(d.rec.Failures, res.Failures...)
	d.rec.Decisions = append(d.rec.Decisions, flushRecord(decision, d.now()))
	d.publishFlush(decision.Trigger, res.Batch)
	return nil
}

func flushRecord(decision FlushDecision, at time.Time) FlushRecord {
	rec := FlushRecord{Trigger: decision.Trigger, At: at}
	for _, dv := range decision.Flush {
		rec.Flush = append(rec.Flush, cycleOfID(dv.ID))
	}
	for _, dv := range decision.Hold {
		rec.Hold = append(rec.Hold, cycleOfID(dv.ID))
	}
	return rec
}

func cycleOfID(id string) int {
	n, _ := strconv.Atoi(id[len("cycle-"):])
	return n
}

func (d *Driver) publishFlush(trigger FlushTrigger, b IntegrationBatch) {
	d.emit(signalcenter.Event{
		Kind: signalcenter.KindShipDelivered, Code: CodeFlushed, Severity: signalcenter.SeverityInfo,
		Reason: fmt.Sprintf("shadow: pool %d batch %d flushed on %s with no PR", d.Pool, b.Index, trigger),
		Fields: map[string]string{fieldPool: strconv.Itoa(d.Pool), fieldBatch: b.Branch, fieldTrigger: string(trigger), fieldPR: "0", fieldShadow: shadowTrue},
	})
}

func (d *Driver) endWave(ctx context.Context, actor Actor) error {
	if !slices.ContainsFunc(d.rec.Decisions, func(f FlushRecord) bool { return f.Trigger == TriggerWaveEnd }) {
		d.rec.Decisions = append(d.rec.Decisions, FlushRecord{Trigger: TriggerWaveEnd, At: d.now()})
	}
	return d.finishWave(ctx, actor)
}

func (d *Driver) finishWave(ctx context.Context, actor Actor) error {
	rec, err := actor.WaveEnd(ctx, *d.rec)
	if err != nil {
		return &stepError{stepWaveEnd, err}
	}
	last, lerr := d.last(ctx)
	rec.LastCycle, rec.Ended = last, true
	d.rec = &rec
	if lerr != nil {
		return &stepError{stepLast, lerr}
	}
	return nil
}

func (d *Driver) last(ctx context.Context) (int, error) {
	if d.Last == nil {
		return 0, nil
	}
	return d.Last(ctx)
}

func (d *Driver) persist() error {
	if reflect.DeepEqual(d.written, *d.rec) {
		return nil
	}
	if err := d.Store.Write(*d.rec); err != nil {
		d.rec.Failures = appendFailures(d.rec.Failures, DriverFailure{Step: stepWrite, Error: err.Error(), At: d.now()})
		return &stepError{stepWrite, err}
	}
	d.written = cloneRecord(*d.rec)
	return nil
}

func cloneRecord(r WaveRecord) WaveRecord {
	out := r
	out.Batches = slices.Clone(r.Batches)
	out.Decisions = slices.Clone(r.Decisions)
	out.Failures = slices.Clone(r.Failures)
	out.Members = make(map[int]MemberState, len(r.Members))
	for k, v := range r.Members {
		out.Members[k] = v
	}
	return out
}

func (d *Driver) fail(err error) {
	step := "driver"
	var se *stepError
	if errors.As(err, &se) {
		step = se.step
	}
	d.emit(signalcenter.Event{
		Kind: signalcenter.KindShipWarning, Code: CodeDriverFailed, Severity: signalcenter.SeverityWarn,
		Reason: fmt.Sprintf("shadow: the integration driver of pool %d failed the step %s: %v", d.Pool, step, err),
		Fields: map[string]string{fieldStep: step, fieldError: err.Error(), fieldPool: strconv.Itoa(d.Pool), fieldShadow: shadowTrue},
	})
	if d.rec != nil && d.stopped == nil {
		d.rec.Failures = appendFailures(d.rec.Failures, DriverFailure{Step: step, Error: err.Error(), At: d.now()})
		_ = d.persist()
	}
}

func (d *Driver) emit(ev signalcenter.Event) { emitEvent(d.Publish, driverOrigin, ev) }

func StageActors(shadow Actor) map[string]Actor {
	return map[string]Actor{policy.FleetIntegrationShadow: shadow}
}
