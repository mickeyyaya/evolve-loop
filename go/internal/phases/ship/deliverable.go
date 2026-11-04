package ship

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	codeIntegrationDelivered signalcenter.Code = "SHIP_INTEGRATION_DELIVERED"
	codeIntegrationComposed  signalcenter.Code = "SHIP_INTEGRATION_COMPOSED"
	codeIntegrationEjected   signalcenter.Code = "SHIP_INTEGRATION_EJECTED"
	codeIntegrationFlushed   signalcenter.Code = "SHIP_INTEGRATION_FLUSHED"
	codeIntegrationMerged    signalcenter.Code = "SHIP_INTEGRATION_MERGED"
	codeIntegrationReverted  signalcenter.Code = "SHIP_INTEGRATION_REVERTED"
)

func init() {
	for c, doc := range map[signalcenter.Code]string{
		codeIntegrationDelivered: "a fleet lane delivered its record to the integration of the wave (kind ship.delivered); fields.wave, fields.cycle, fields.commits and fields.record name it, and at enforce the lane's ship stopped before main",
		codeIntegrationComposed:  "a deliverable composed onto the integration branch and passed its L1 gates; fields.wave, fields.cycle and fields.batch name it",
		codeIntegrationEjected:   "a deliverable went back to its lane; fields.cause names why (conflict, gate or proof) and fields.layer names the layer that found it",
		codeIntegrationFlushed:   "a batch PR opened; fields.trigger names the flush trigger and fields.pr the PR",
		codeIntegrationMerged:    "a batch PR merged into main by a fast-forward to the tested tip; fields.pr names the PR and fields.tip the tip",
		codeIntegrationReverted:  "the check after a merge reverted one cycle commit alone; fields.cycle names it and fields.commit the reverted commit",
	} {
		signalcenter.RegisterCode(signalcenter.ModuleShip, c, doc)
	}
}

type Integration struct {
	Stage     string
	Wave      string
	Risk      string
	InboxRank int
	Scope     []string
	Deps      []string
}

func (o *Options) integrationDir() (string, error) {
	return integration.WaveDir(o.ProjectRoot, o.Integration.Wave)
}

func (s *worktreeShip) deliver(commit, pending string) (bool, error) {
	if s.opts.Integration == nil {
		return false, nil
	}
	enforce := s.opts.Integration.Stage == policy.FleetIntegrationEnforce
	rec, path, err := s.writeDeliverable(commit)
	if err != nil {
		if !enforce {
			s.logf("[ship] WARN: deliverable record not written: %v - the ship lands as before", err)
			return false, nil
		}
		return false, shipErr(core.CodeStateIO, deliverableErrorClass(err), core.StageAtomicShip,
			"ship: deliverable record not written, so ship delivered nothing: "+err.Error(), "commit", commit)
	}
	s.emitDelivered(rec, path)
	if !enforce {
		return false, nil
	}
	if err := s.moveLaneRef(pending); err != nil {
		return false, err
	}
	s.result.CommitSHA = commit
	s.logf("[ship] DELIVERED: %s holds the record of cycle %d; ship stops before %s", path, rec.Cycle, s.branch)
	return true, nil
}

func deliverableErrorClass(err error) core.ShipErrorClass {
	if errors.Is(err, integration.ErrNoPatch) || errors.Is(err, integration.ErrBadWaveID) {
		return core.ShipClassPrecondition
	}
	return core.ShipClassTransient
}

func (s *worktreeShip) writeDeliverable(commit string) (integration.DeliverableRecord, string, error) {
	dir, err := s.opts.integrationDir()
	if err != nil {
		return integration.DeliverableRecord{}, "", err
	}
	rec, err := s.buildDeliverable(commit)
	if err != nil {
		return rec, "", err
	}
	path, err := integration.WriteRecord(dir, rec)
	return rec, path, err
}

func (s *worktreeShip) buildDeliverable(commit string) (integration.DeliverableRecord, error) {
	o, in := s.opts, s.opts.Integration
	rec := integration.DeliverableRecord{
		Wave:        in.Wave,
		Cycle:       o.CycleID,
		Trailer:     integration.CycleTrailer(o.CycleID),
		Audit:       integration.AuditBinding{TreeSHA: o.internalAuditBoundTreeSHA, ArtifactSHA: o.internalAuditArtifactSHA},
		Scope:       in.Scope,
		Deps:        in.Deps,
		InboxRank:   in.InboxRank,
		Risk:        in.Risk,
		DeliveredAt: s.deliveredAt(),
	}
	if cid, ok, err := cycleIDForShip(o); err == nil && ok {
		rec.Cycle, rec.Trailer = cid, integration.CycleTrailer(cid)
	}
	list, err := captureGitOutputAtDir(s.ctx, o, s.worktree, "rev-list", "--reverse", o.WorktreeBaseSHA+".."+commit)
	if err != nil {
		return rec, err
	}
	repo := integration.Repo{Dir: s.worktree, Git: shipGit(o)}
	for _, sha := range strings.Fields(list) {
		patch, err := repo.PatchID(s.ctx, sha)
		if err != nil {
			return rec, err
		}
		rec.Commits = append(rec.Commits, integration.CommitIdentity{SHA: sha, BaseSHA: o.WorktreeBaseSHA, PatchID: patch})
	}
	return rec, nil
}

func (s *worktreeShip) deliveredAt() time.Time {
	now := defaultNow
	if s.opts.NowFn != nil {
		now = s.opts.NowFn
	}
	return time.Unix(now().Unix, 0).UTC()
}

func shipGit(opts *Options) integration.GitFunc {
	return func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if stdin == "" {
			return captureGitOutputAtDir(ctx, opts, dir, args...)
		}
		var out strings.Builder
		exit, err := opts.runStdin(ctx, "git", args, strings.NewReader(stdin), &out, io.Discard)
		if err != nil || exit != 0 {
			return "", errors.Join(fmt.Errorf("git %s exited %d", strings.Join(args, " "), exit), err)
		}
		return out.String(), nil
	}
}

func (s *worktreeShip) emitDelivered(rec integration.DeliverableRecord, path string) {
	s.opts.Signals.Emit(signalcenter.Event{
		Cycle: rec.Cycle, RunID: s.opts.RunID, Phase: "ship", Module: signalcenter.ModuleShip, Origin: "deliver",
		Kind: signalcenter.KindShipDelivered, Code: codeIntegrationDelivered, Severity: signalcenter.SeverityInfo,
		Reason: "cycle " + strconv.Itoa(rec.Cycle) + " delivered its record to wave " + rec.Wave,
		Fields: map[string]string{"wave": rec.Wave, "cycle": strconv.Itoa(rec.Cycle), "commits": strconv.Itoa(len(rec.Commits)), "record": path},
	})
}

func landedByIdentity(ctx context.Context, opts *Options, sha string, refs ...string) bool {
	if opts.Integration == nil {
		return false
	}
	cid, ok, err := cycleIDForShip(opts)
	if err != nil || !ok {
		return false
	}
	dir, err := opts.integrationDir()
	if err != nil {
		return false
	}
	rec, err := integration.ReadRecord(integration.RecordPath(dir, cid))
	if err != nil || !rec.HoldsCommit(sha) {
		return false
	}
	repo := integration.Repo{Dir: opts.ProjectRoot, Git: shipGit(opts)}
	for _, ref := range refs {
		if repo.LandedOnRef(ctx, rec, ref) {
			return true
		}
	}
	return false
}
