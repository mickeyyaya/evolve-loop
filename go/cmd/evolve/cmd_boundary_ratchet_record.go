package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	roleRatchet   = "ratchet"
	gatePass      = "pass"
	gateSkipped   = "skipped"
	decisionNone  = "none"
	decisionNo    = "no_record"
	decisionWould = "would_record"
	decisionDone  = "recorded"
	decisionAgain = "already_recorded"
)

type ratchetRecordRequest struct {
	root string
	pr   int
	head string
	bind integration.Binder
	env  integrationEnv
	warn io.Writer
}

var ratchetRecord = recordRatchetPR

func recordRatchetPR(req ratchetRecordRequest) (string, error) {
	train, _ := req.env.train(req.root, req.warn)
	return train.Record(context.Background(), req.pr, req.bind)
}

func ratchetListPaths() []string {
	listed := make([]string, 0, len(ratchetLists))
	for _, l := range ratchetLists {
		listed = append(listed, ratchetRepoPath(l.rel))
	}
	return listed
}

func onlyRatchetLists(changed []string) bool {
	listed := ratchetListPaths()
	return len(changed) > 0 && !slices.ContainsFunc(changed, func(p string) bool { return !slices.Contains(listed, p) })
}

func ratchetListsBinder(ev integration.Evidence) (integration.AuditBinding, error) {
	return integration.ClassBinding(ev, integration.RatchetListsClass, onlyRatchetLists)
}

func ratchetRaw(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ratchetGitTimeout)
	defer cancel()
	out, errOut, code, err := sysexec.Capture(ctx, ratchetRun, dir, "git", args...)
	if err != nil || code != 0 {
		return "", fmt.Errorf("git %v exit=%d: %w %s", args, code, err, strings.TrimSpace(errOut))
	}
	return out, nil
}

func ratchetHeadMatches(wt, head string, lists []tightenedList) string {
	shape, err := ratchetCmd(wt, "git", "rev-list", "--parents", "origin/main.."+head)
	if err != nil {
		return "content_unknown"
	}
	if rows := strings.Split(shape, "\n"); shape == "" || len(rows) != 1 || len(strings.Fields(rows[0])) != 2 {
		return "not_single_commit"
	}
	files, err := ratchetCmd(wt, "git", "diff-tree", "--no-commit-id", "--name-only", "-r", "--end-of-options", head)
	if err != nil {
		return "content_unknown"
	}
	if !onlyRatchetLists(strings.Fields(files)) {
		return "unexpected_files"
	}
	for _, l := range lists {
		raw, err := ratchetRaw(wt, "show", head+":"+ratchetRepoPath(l.rel))
		if err != nil {
			return "content_unknown"
		}
		if raw != string(l.encoded) {
			return "content_differs"
		}
	}
	return ""
}

func ratchetStage(root string) (policy.RatchetMergeStage, error) {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err != nil {
		return policy.RatchetMergeShadow, err
	}
	return pol.RatchetMergeStage(), nil
}

type ratchetDecision struct {
	stage, decision, reason, detail         string
	pr                                      int
	head                                    string
	headGate, contentGate, scans, candidate string
	skipped                                 bool
}

func (d ratchetDecision) refuse(reason string) ratchetDecision {
	d.decision, d.reason = decisionNo, reason
	return d
}

type ratchetRound struct {
	root, wt string
	stdout   io.Writer
	stderr   io.Writer
}

type roundResult struct {
	stage      policy.RatchetMergeStage
	stageErr   error
	cand       ratchetCandidate
	out        ratchetOutcome
	proposeErr error
}

func (r ratchetRound) run() error {
	res := roundResult{cand: findRatchetCandidate(r.wt)}
	res.stage, res.stageErr = ratchetStage(r.root)
	res.proposeErr = res.cand.err
	if res.proposeErr == nil {
		res.out, res.proposeErr = tightenAndPropose(r.wt, r.stdout, res.cand)
	}
	if res.stage != policy.RatchetMergeOff {
		r.emit(r.settle(res))
	}
	return res.proposeErr
}

func (r ratchetRound) settle(res roundResult) ratchetDecision {
	out := res.out
	d := ratchetDecision{stage: string(res.stage), decision: decisionNone, headGate: gateSkipped, contentGate: gateSkipped, scans: gateSkipped, candidate: gateSkipped, skipped: out.skipped}
	switch {
	case res.stageErr != nil:
		return d.refuse("policy_unreadable")
	case out.scanReason != "":
		d.scans = out.scanReason
		return d.refuse(out.scanReason)
	case res.proposeErr != nil:
		return d.refuse("propose_failed")
	case out.tight:
		return r.settleTight(d, res.cand)
	}
	d.scans = gatePass
	return r.settleGates(d, res.stage, out)
}

func (r ratchetRound) settleTight(d ratchetDecision, cand ratchetCandidate) ratchetDecision {
	if cand.pr.Number == 0 {
		return d
	}
	d.pr, d.reason = cand.pr.Number, "stale_open_pr"
	fmt.Fprintf(r.stderr, "%sWARN: ratchet record: #%d is open and origin/main is tight; close it\n", boundaryPrefix, cand.pr.Number)
	return d
}

func (r ratchetRound) settleGates(d ratchetDecision, stage policy.RatchetMergeStage, out ratchetOutcome) ratchetDecision {
	d.pr, d.head = out.pr.Number, out.head
	switch {
	case out.ambiguous:
		d.candidate = "ambiguous_pr"
	case out.pr.Number == 0:
		d.candidate = "pr_unknown"
	case out.pr.Draft:
		d.candidate = "draft"
	default:
		d.candidate = gatePass
	}
	d.headGate = headGateOf(out)
	d.contentGate = gatePass
	if out.content != "" {
		d.contentGate = out.content
	}
	for _, gate := range []string{d.candidate, d.headGate, d.contentGate} {
		if gate != gatePass {
			return d.refuse(gate)
		}
	}
	if stage != policy.RatchetMergeEnforce {
		fmt.Fprintf(r.stdout, "ratchet record: would record #%d\n", d.pr)
		d.decision = decisionWould
		return d
	}
	return r.record(d)
}

func headGateOf(out ratchetOutcome) string {
	switch {
	case out.pr.headErr != "":
		return out.pr.headErr
	case out.pr.Head == "":
		return "head_unknown"
	case out.pr.Head != out.head:
		return "head_moved"
	}
	return gatePass
}

func (r ratchetRound) record(d ratchetDecision) ratchetDecision {
	if rec, ok := integration.FindPRRecord(r.root, d.pr); ok && rec.Commits[len(rec.Commits)-1].SHA == d.head {
		d.decision = decisionAgain
		return d
	}
	_, err := ratchetRecord(ratchetRecordRequest{root: r.root, pr: d.pr, head: d.head, bind: ratchetListsBinder, env: productionIntegrationEnv(r.root, time.Now()), warn: r.stderr})
	if err == nil {
		fmt.Fprintf(r.stdout, "ratchet record: recorded #%d\n", d.pr)
		d.decision = decisionDone
		return d
	}
	d.detail = err.Error()
	fmt.Fprintf(r.stderr, "%sWARN: ratchet record: #%d: %v\n", boundaryPrefix, d.pr, err)
	var refusal *integration.Refusal
	if errors.As(err, &refusal) {
		return d.refuse("record_refused")
	}
	return d.refuse("record_error")
}

func (r ratchetRound) emit(d ratchetDecision) {
	emitRatchetRecordDecision(r.root, r.stderr, d)
}

func emitRatchetRecordDecision(root string, stderr io.Writer, d ratchetDecision) {
	signals, closeSignals := newRoleSignalCenter(roleRatchet, root, paths.EvolveDirOf(root), stderr)
	defer closeSignals()
	kind, severity := signalcenter.KindGatePassed, signalcenter.SeverityInfo
	if d.decision == decisionNo && d.stage == string(policy.RatchetMergeEnforce) {
		kind, severity = signalcenter.KindGateRejected, signalcenter.SeverityWarn
	}
	signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleCLI, Origin: "evolve.emitRatchetRecordDecision", Kind: kind, Severity: severity,
		Code: signalcenter.CodeCLIRatchetMergeDecision, Reason: strings.TrimSpace(fmt.Sprintf("%s %s %s", d.decision, d.reason, d.detail)),
		Fields: map[string]string{
			"stage": d.stage, "pr": fmt.Sprint(d.pr), "head": d.head, "decision": d.decision, "reason": d.reason,
			"head_gate": d.headGate, "content_gate": d.contentGate, "scans": d.scans,
			"skipped_push": fmt.Sprint(d.skipped), "candidate": d.candidate,
		},
	})
}
