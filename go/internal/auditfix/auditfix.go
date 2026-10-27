package auditfix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const (
	RecordFile = "audit-fix.json"

	ReasonOff          = "off"
	ReasonShadow       = "shadow"
	ReasonNotChosen    = "not_chosen"
	ReasonNoReviewSeat = "no_review_seat"
	ReasonPassed       = "passed"
	ReasonHeld         = "held_proposal"
	ReasonEscalate     = "escalate"
	ReasonExhausted    = "audit_fix_exhausted"

	RefusalEmpty   = "empty_patch"
	RefusalUncited = "outside_cited_files"
	RefusalFence   = "fence_path"
	RefusalReview  = "review_failed"
	RefusalGate    = "gate_red"
)

type Seat struct {
	Family  string `json:"family"`
	Tier    string `json:"tier"`
	Session string `json:"session"`
}

type Prescription struct {
	Finding string `json:"finding"`
	Defect  string `json:"defect"`
	Text    string `json:"prescription"`
}

type Change struct {
	Path   string `json:"path"`
	Lines  int    `json:"lines"`
	Reason string `json:"reason,omitempty"`
}

type Patch struct {
	Changes []Change `json:"changes,omitempty"`
	Diff    string   `json:"diff,omitempty"`
}

type Verdict struct {
	Pass    bool
	Reasons []string
}

type FixRequest struct {
	Cycle         int
	Worktree      string
	Prescriptions []Prescription
	Cited         []string
	Bounds        retryladder.AuditFixBounds
}

type ReviewRequest struct {
	Seat          Seat
	Patch         Patch
	Prescriptions []Prescription
}

type Auditor interface {
	Seat() Seat
	ApplyFix(ctx context.Context, req FixRequest) (map[string]string, error)
}

type Reviewer interface {
	ReviewSeat(ctx context.Context, avoid Seat) (Seat, error)
	Review(ctx context.Context, req ReviewRequest) (Verdict, error)
}

type Gates interface {
	Run(ctx context.Context, worktree string) (Verdict, error)
}

type Worktree interface {
	Snapshot(ctx context.Context, dir string) (string, error)
	Diff(ctx context.Context, dir, base string) (Patch, error)
	Restore(ctx context.Context, dir, base string) error
}

type Ports struct {
	Auditor  Auditor
	Reviewer Reviewer
	Gates    Gates
	Worktree Worktree
}

type Config struct {
	Stage     string
	HoldFence bool
}

type Record struct {
	Cycle         int            `json:"cycle"`
	Stage         string         `json:"stage"`
	Outcome       string         `json:"outcome"`
	Refusal       string         `json:"refusal,omitempty"`
	Failed        []string       `json:"failed,omitempty"`
	Auditor       Seat           `json:"auditor"`
	Reviewer      Seat           `json:"reviewer"`
	Prescriptions []Prescription `json:"prescriptions"`
	Cited         []string       `json:"cited"`
	Patch         Patch          `json:"patch"`
	Held          []string       `json:"held,omitempty"`
}

type Handler struct {
	cfg   Config
	ports Ports
}

var _ core.RungHandler = (*Handler)(nil)

func New(cfg Config, ports Ports) *Handler {
	return &Handler{cfg: cfg, ports: ports}
}

func (h *Handler) Handle(ctx context.Context, in core.RungInput) (core.RungResult, error) {
	switch {
	case in.Rung != retryladder.RungAuditFix || !in.Choice.Choose:
		return core.RungResult{Reason: ReasonNotChosen}, nil
	case h.cfg.Stage == policy.AuditFixOff:
		return core.RungResult{Reason: ReasonOff}, nil
	}
	prior, found, err := ReadRecord(in.Workspace)
	if err != nil {
		return core.RungResult{}, err
	}
	rec := newRecord(in, h.cfg.Stage)
	switch {
	case found && prior.Outcome == ReasonEscalate:
		return finish(in.Workspace, rec, core.RungResult{Handled: true, Stop: true, Reason: ReasonExhausted})
	case h.cfg.Stage != policy.RetryLadderEnforce:
		return finish(in.Workspace, rec, core.RungResult{Reason: ReasonShadow})
	}
	return h.apply(ctx, in, rec)
}

func (h *Handler) apply(ctx context.Context, in core.RungInput, rec Record) (core.RungResult, error) {
	rec.Auditor = h.ports.Auditor.Seat()
	seat, err := h.ports.Reviewer.ReviewSeat(ctx, rec.Auditor)
	if err != nil {
		return core.RungResult{}, fmt.Errorf("auditfix: review seat: %w", err)
	}
	if seat.Session == "" || seat.Session == rec.Auditor.Session {
		return finish(in.Workspace, rec, core.RungResult{Reason: ReasonNoReviewSeat})
	}
	rec.Reviewer = seat
	base, err := h.ports.Worktree.Snapshot(ctx, in.Worktree)
	if err != nil {
		return core.RungResult{}, fmt.Errorf("auditfix: snapshot: %w", err)
	}
	rec.Patch, err = h.patch(ctx, in, rec, base)
	if err != nil {
		return core.RungResult{}, errors.Join(err, h.ports.Worktree.Restore(ctx, in.Worktree, base))
	}
	if rec.Refusal, rec.Held = h.refusal(rec, in.Choice.Input.Bounds); rec.Refusal != "" {
		return h.escalate(ctx, in, rec, base, nil)
	}
	return h.judge(ctx, in, rec, base)
}

func (h *Handler) patch(ctx context.Context, in core.RungInput, rec Record, base string) (Patch, error) {
	reasons, err := h.ports.Auditor.ApplyFix(ctx, FixRequest{
		Cycle: in.Cycle, Worktree: in.Worktree, Prescriptions: rec.Prescriptions, Cited: rec.Cited, Bounds: in.Choice.Input.Bounds,
	})
	if err != nil {
		return Patch{}, fmt.Errorf("auditfix: apply: %w", err)
	}
	patch, err := h.ports.Worktree.Diff(ctx, in.Worktree, base)
	if err != nil {
		return Patch{}, fmt.Errorf("auditfix: diff: %w", err)
	}
	changes := make([]Change, len(patch.Changes))
	for i, c := range patch.Changes {
		changes[i] = Change{Path: c.Path, Lines: c.Lines, Reason: reasons[c.Path]}
	}
	return Patch{Changes: changes, Diff: patch.Diff}, nil
}

func (h *Handler) judge(ctx context.Context, in core.RungInput, rec Record, base string) (core.RungResult, error) {
	review, err := h.ports.Reviewer.Review(ctx, ReviewRequest{Seat: rec.Reviewer, Patch: rec.Patch, Prescriptions: rec.Prescriptions})
	if err != nil {
		return core.RungResult{}, errors.Join(fmt.Errorf("auditfix: review: %w", err), h.ports.Worktree.Restore(ctx, in.Worktree, base))
	}
	if !review.Pass {
		rec.Refusal = RefusalReview
		return h.escalate(ctx, in, rec, base, review.Reasons)
	}
	gates, err := h.ports.Gates.Run(ctx, in.Worktree)
	if err != nil {
		return core.RungResult{}, errors.Join(fmt.Errorf("auditfix: gates: %w", err), h.ports.Worktree.Restore(ctx, in.Worktree, base))
	}
	if !gates.Pass {
		rec.Refusal = RefusalGate
		return h.escalate(ctx, in, rec, base, gates.Reasons)
	}
	reason := ReasonPassed
	if len(rec.Held) > 0 {
		reason = ReasonHeld
	}
	return finish(in.Workspace, rec, core.RungResult{Handled: true, Reason: reason})
}

func (h *Handler) escalate(ctx context.Context, in core.RungInput, rec Record, base string, failed []string) (core.RungResult, error) {
	if err := h.ports.Worktree.Restore(ctx, in.Worktree, base); err != nil {
		return core.RungResult{}, fmt.Errorf("auditfix: restore: %w", err)
	}
	rec.Failed = failed
	return finish(in.Workspace, rec, core.RungResult{Handled: true, Escalate: true, Reason: ReasonEscalate})
}

func (h *Handler) refusal(rec Record, b retryladder.AuditFixBounds) (string, []string) {
	if len(rec.Patch.Changes) == 0 {
		return RefusalEmpty, nil
	}
	lines := map[string]int{}
	var held []string
	for _, c := range rec.Patch.Changes {
		switch {
		case !slices.Contains(rec.Cited, c.Path):
			return RefusalUncited, nil
		case guards.IsProtectedSurface(c.Path) && !h.cfg.HoldFence:
			return RefusalFence, nil
		case guards.IsProtectedSurface(c.Path):
			held = append(held, c.Path)
		}
		lines[c.Path] += c.Lines
	}
	return retryladder.AuditFixOverBounds(retryladder.EstimatePatch(lines), b), held
}

func newRecord(in core.RungInput, stage string) Record {
	fb := in.Failure
	prescriptions := make([]Prescription, len(fb.Prescription))
	for i, text := range fb.Prescription {
		p := Prescription{Text: text}
		if i < len(fb.Defects) {
			p.Defect, p.Finding = fb.Defects[i], findingID(fb.Defects[i])
		}
		prescriptions[i] = p
	}
	return Record{
		Cycle: in.Cycle, Stage: stage, Prescriptions: prescriptions,
		Cited: retryladder.CitedPaths(append(slices.Clone(fb.Defects), fb.Prescription...)...),
	}
}

func findingID(defect string) string {
	id, _, ok := strings.Cut(defect, ":")
	if !ok || strings.ContainsAny(id, " \t") {
		return ""
	}
	return id
}

func finish(workspace string, rec Record, res core.RungResult) (core.RungResult, error) {
	rec.Outcome = res.Reason
	if err := atomicwrite.JSON(filepath.Join(workspace, RecordFile), rec); err != nil {
		return core.RungResult{}, fmt.Errorf("auditfix: write %s: %w", RecordFile, err)
	}
	return res, nil
}

func ReadRecord(workspace string) (Record, bool, error) {
	var rec Record
	raw, err := os.ReadFile(filepath.Join(workspace, RecordFile))
	if errors.Is(err, fs.ErrNotExist) {
		return rec, false, nil
	}
	if err == nil {
		err = json.Unmarshal(raw, &rec)
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("auditfix: read %s: %w", RecordFile, err)
	}
	return rec, true, nil
}
