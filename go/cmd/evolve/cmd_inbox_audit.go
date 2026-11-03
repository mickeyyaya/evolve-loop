package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	inboxAuditSeat        = "inbox-curator"
	inboxAuditSchema      = "evolve.inbox.audit-report/1"
	inboxAuditInputSchema = "evolve.inbox.audit-input/1"
	inboxAuditDir         = "inbox-audit"
	inboxAuditStampLayout = "2006-01-02T15-04-05Z"
)

type inboxAuditEnv struct {
	agent     agentSeat
	now       func() time.Time
	newCenter func(root, evolveDir string, console io.Writer) (*signalcenter.Center, func())
}

type inboxAuditInput struct {
	Schema     string                        `json:"schema"`
	Candidates []inboxcurator.AuditCandidate `json:"candidates"`
}

type inboxAuditReport struct {
	Schema     string                  `json:"schema"`
	Ran        bool                    `json:"ran"`
	Count      int                     `json:"count"`
	Threshold  int                     `json:"threshold"`
	Candidates []string                `json:"candidates"`
	RunLabel   string                  `json:"run_label,omitempty"`
	Proposals  []inboxcurator.Proposal `json:"proposals"`
	Refused    []string                `json:"refused"`
	Record     string                  `json:"record,omitempty"`
}

type inboxAuditFailure struct {
	rc  int
	err error
}

func runInboxAudit(root string, args []string, stdout, stderr io.Writer) int {
	return runInboxAuditWith(defaultInboxAuditEnv(), root, args, stdout, stderr)
}

func defaultInboxAuditEnv() inboxAuditEnv {
	return inboxAuditEnv{agent: runAgentCommand, now: time.Now, newCenter: newInboxAuditCenter}
}

func newInboxAuditCenter(root, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	return newRoleSignalCenter("inbox", root, evolveDir, console)
}

func runInboxAuditWith(env inboxAuditEnv, root string, args []string, stdout, stderr io.Writer) int {
	asJSON, args := takeInboxJSON(args)
	fs := flag.NewFlagSet("inbox audit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "run the seat below the policy threshold")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(stderr, inboxUsage("audit"))
		return cliExitUsage
	}
	run, stale, fail := selectInboxAudit(env, root, stderr)
	report := inboxAuditReport{Schema: inboxAuditSchema, Count: len(stale), Threshold: run.cfg.TriggerCount, Candidates: []string{}, Proposals: []inboxcurator.Proposal{}, Refused: []string{}}
	if fail == nil && shouldAudit(len(stale), run.cfg.TriggerCount, *force) {
		fail = run.judge(&report, stale[:min(len(stale), run.cfg.MaxItems)])
	}
	if fail != nil {
		fmt.Fprintf(stderr, "inbox audit: %v\n", fail.err)
		return fail.rc
	}
	for _, msg := range report.Refused {
		fmt.Fprintln(stderr, "inbox audit: WARN: "+msg)
	}
	if asJSON {
		return printVerbJSON("inbox audit", report, stdout, stderr)
	}
	printInboxAudit(stdout, report)
	return cliExitOK
}

func shouldAudit(count, threshold int, force bool) bool {
	return count > 0 && (force || count >= threshold)
}

type inboxAuditRun struct {
	env    inboxAuditEnv
	root   string
	pol    policy.Policy
	cfg    policy.InboxAuditConfig
	stderr io.Writer
}

func selectInboxAudit(env inboxAuditEnv, root string, stderr io.Writer) (inboxAuditRun, []inboxbatch.Item, *inboxAuditFailure) {
	run := inboxAuditRun{env: env, root: root, stderr: stderr}
	pol, err := policy.Load(filepath.Join(paths.EvolveDirOf(root), paths.PolicyFile))
	if err != nil {
		return run, nil, &inboxAuditFailure{cliExitIO, err}
	}
	run.pol, run.cfg = pol, pol.InboxAuditConfig()
	pending, err := loadPendingInbox(root, "audit", stderr)
	if err != nil {
		return run, nil, &inboxAuditFailure{cliExitIO, err}
	}
	_, console, _ := inboxbatch.PartitionConsole(pending.items, pending.isProtected)
	return run, inboxcurator.StaleProbeless(console, env.now().Add(-time.Duration(run.cfg.MinAgeDays)*24*time.Hour)), nil
}

func (r inboxAuditRun) judge(report *inboxAuditReport, items []inboxbatch.Item) *inboxAuditFailure {
	ctx := context.Background()
	git := gitexec.Default(r.root)
	src := inboxcurator.CandidateSource{Git: git, Limit: inboxcurator.AuditLandedLimit, Exists: func(p string) bool {
		_, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(p)))
		return err == nil
	}}
	var cands []inboxcurator.AuditCandidate
	for _, it := range items {
		c, err := src.Candidate(ctx, it)
		if err != nil {
			return &inboxAuditFailure{cliExitIO, err}
		}
		cands = append(cands, c)
		report.Candidates = append(report.Candidates, c.ID)
	}
	seat, err := runAgentSeat(r.env.agent, []string{inboxAuditSeat, "--json", "--project-root", r.root}, inboxAuditPrompt(cands), r.stderr)
	report.Ran, report.RunLabel = true, seat.RunLabel
	if err != nil {
		return &inboxAuditFailure{cliExitIO, fmt.Errorf("%w (run %q)", err, seat.RunLabel)}
	}
	res, err := inboxcurator.ParseAudit([]byte(seat.Report), cands, func(sha string) bool { return inboxcurator.IsLandedCommit(ctx, git, sha) })
	if err != nil {
		return &inboxAuditFailure{cliExitFailed, fmt.Errorf("%w (run %q)", err, seat.RunLabel)}
	}
	report.Proposals = append(report.Proposals, res.Proposals...)
	report.Refused = append(report.Refused, res.Refused...)
	if err := storeInboxAudit(report, r.root, r.env.now()); err != nil {
		return &inboxAuditFailure{cliExitIO, fmt.Errorf("store the proposals: %w", err)}
	}
	r.publish(report.Proposals)
	return nil
}

func inboxAuditPrompt(cands []inboxcurator.AuditCandidate) string {
	raw, _ := json.Marshal(inboxAuditInput{Schema: inboxAuditInputSchema, Candidates: cands})
	return "Judge each candidate below against the checked-out branch. Write inbox-audit.json. You propose only; the pipeline moves no item.\n\n" +
		inboxcurator.AuditContract() + "\nThe candidates:\n\n" + string(raw) + "\n"
}

func storeInboxAudit(report *inboxAuditReport, root string, now time.Time) error {
	report.Record = filepath.Join(paths.EvolveDirOf(root), inboxAuditDir, now.UTC().Format(inboxAuditStampLayout)+"-"+report.RunLabel+".json")
	raw, _ := json.MarshalIndent(report, "", "  ")
	if err := os.MkdirAll(filepath.Dir(report.Record), 0o755); err != nil {
		return err
	}
	return atomicwrite.Bytes(report.Record, append(raw, '\n'))
}

func (r inboxAuditRun) publish(proposals []inboxcurator.Proposal) {
	center, closeCenter := r.env.newCenter(r.root, paths.EvolveDirOf(r.root), r.stderr)
	defer closeCenter()
	stage := r.pol.InboxCuratorConfig().Stage
	for _, p := range proposals {
		if p.Kind != inboxcurator.ProposeKeep {
			center.Emit(p.Event(stage, false))
		}
	}
}

func printInboxAudit(stdout io.Writer, r inboxAuditReport) {
	fmt.Fprintf(stdout, "inbox audit: %d old console item(s) with no probe; the threshold is %d\n", r.Count, r.Threshold)
	if !r.Ran {
		fmt.Fprintln(stdout, "inbox audit: below the threshold; no seat ran (--force runs it)")
		return
	}
	fmt.Fprintf(stdout, "inbox audit: the %s seat judged %d item(s) (run %s): %d proposal(s), %d refused; nothing moved\n",
		inboxAuditSeat, len(r.Candidates), r.RunLabel, len(r.Proposals), len(r.Refused))
	for _, p := range r.Proposals {
		fmt.Fprintf(stdout, "  %s %s: %s [%s]\n", p.Kind, p.ID, p.Reason, strings.Join(p.Evidence, ", "))
	}
	fmt.Fprintf(stdout, "inbox audit: the proposals are in %s\n", r.Record)
}
