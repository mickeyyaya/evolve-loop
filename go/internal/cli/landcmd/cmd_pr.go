package landcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/inboxcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/mergewindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	prUsage       = "usage: evolve pr merge <n>... [--update-branch] [--wait D] [--json] [--project-root P]"
	prMergePrefix = "evolve pr merge: "
)

var (
	prNumber   = regexp.MustCompile(`^[1-9][0-9]*$`)
	fullCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type prMergeArgs struct {
	prs          []string
	updateBranch bool
	wait         time.Duration
	projectRoot  string
	json         bool
}

func parsePRMergeArgs(args []string) (prMergeArgs, error) {
	var a prMergeArgs
	var wait string
	operands, err := cmdutil.Flags().
		Bools(map[string]*bool{"--update-branch": &a.updateBranch, "--json": &a.json}).
		Values(map[string]*string{"--wait": &wait, "--project-root": &a.projectRoot}).
		Parse(args)
	if err != nil {
		return a, err
	}
	if wait != "" {
		if a.wait, err = time.ParseDuration(wait); err != nil || a.wait < 0 {
			return a, fmt.Errorf("--wait %q must be a non-negative duration", wait)
		}
	}
	if len(operands) == 0 {
		return a, errors.New("name at least one PR number")
	}
	seen := map[string]bool{}
	for _, n := range operands {
		if !prNumber.MatchString(n) || seen[n] {
			return a, fmt.Errorf("PR operand %q must be a distinct positive number", n)
		}
		seen[n] = true
	}
	a.prs = operands
	return a, nil
}

func RunPR(args []string, _ io.Reader, stdout, stderr io.Writer, deps Deps) int {
	current.Store(&deps)
	return dispatchPR(args, stdout, stderr)
}

func dispatchPR(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "window" {
		return runPRWindow(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "boundary" {
		return runPRBoundary(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "refresh" {
		return runPRRefresh(args[1:], stdout, stderr)
	}
	if len(args) == 0 || args[0] != "merge" {
		fmt.Fprintln(stderr, prUsage+"\n"+usagePRWindow+"\n"+usagePRBoundary+"\n"+usagePRRefresh)
		return exitUsage
	}
	a, err := parsePRMergeArgs(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", prMergePrefix, err, prUsage)
		return exitUsage
	}
	root, err := resolveRoot(a.projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prMergePrefix, err)
		return exitIO
	}
	m, err := newPRMerger(root, a, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prMergePrefix, err)
		return exitIO
	}
	return mergePRs(m, a, stdout, stderr)
}

func mergePRs(m prMerger, a prMergeArgs, stdout, stderr io.Writer) int {
	if !a.json {
		return m.mergeAll(context.Background(), a.prs)
	}
	m.stdout = io.Discard
	result, rc := m.merge(context.Background(), a.prs)
	return max(printVerbJSON("pr merge", result, stdout, stderr), rc)
}

type prMerged struct {
	PR  string `json:"pr"`
	SHA string `json:"sha"`
}

type prMergeResult struct {
	Merged       []prMerged `json:"merged"`
	Refused      string     `json:"refused,omitempty"`
	NotAttempted []string   `json:"not_attempted"`
}

type prMerger struct {
	root         string
	windowOpen   bool
	method       string
	trailerStage string
	poll, wait   time.Duration
	updateBranch bool
	gh           func(ctx context.Context, args ...string) ([]byte, error)
	fetch        ciwatch.Fetcher
	liveRuns     func() ([]runlease.LiveRun, error)
	now          func() time.Time
	sleep        func(time.Duration)
	stdout       io.Writer
	stderr       io.Writer
}

func newPRMerger(root string, a prMergeArgs, stdout, stderr io.Writer) (prMerger, error) {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err != nil {
		return prMerger{}, err
	}
	method, err := pol.PRMergeMethod()
	if err != nil {
		return prMerger{}, err
	}
	cw, err := pol.CIWatchConfig()
	if err != nil {
		return prMerger{}, err
	}
	gate, err := windowGateOf(pol)
	if err != nil {
		return prMerger{}, err
	}
	return prMerger{
		root: root, windowOpen: gate.open(), method: method, trailerStage: pol.InboxTrailerStage(),
		poll: time.Duration(*cw.PollS) * time.Second, wait: a.wait, updateBranch: a.updateBranch,
		gh: cmdutil.GHIn(root), fetch: ciwatch.NewGHFetcher(root), liveRuns: func() ([]runlease.LiveRun, error) { return cmdutil.PlaneLiveRuns(root, time.Now()) },
		now: time.Now, sleep: time.Sleep, stdout: stdout, stderr: stderr,
	}, nil
}

type prRefusal string

func (r prRefusal) Error() string { return string(r) }

func refusef(format string, args ...any) error { return prRefusal(fmt.Sprintf(format, args...)) }

func (m prMerger) mergeAll(ctx context.Context, prs []string) int {
	_, rc := m.merge(ctx, prs)
	return rc
}

func (m prMerger) merge(ctx context.Context, prs []string) (prMergeResult, int) {
	result := prMergeResult{Merged: []prMerged{}, NotAttempted: []string{}}
	if err := m.closedWindowIdle(); err != nil {
		result.Refused, result.NotAttempted = err.Error(), append(result.NotAttempted, prs...)
		code := m.report("", err)
		m.notAttempted(prs)
		return result, code
	}
	for i, n := range prs {
		sha, err := m.mergeOne(ctx, n)
		if err != nil {
			result.Refused, result.NotAttempted = "#"+n+" "+err.Error(), append(result.NotAttempted, prs[i+1:]...)
			code := m.report("#"+n+" ", err)
			m.notAttempted(prs[i+1:])
			return result, code
		}
		result.Merged = append(result.Merged, prMerged{PR: n, SHA: sha})
		fmt.Fprintf(m.stdout, "merged #%s %s\n", n, sha)
	}
	return result, exitOK
}

func (m prMerger) report(subject string, err error) int {
	var refusal prRefusal
	if errors.As(err, &refusal) {
		fmt.Fprintf(m.stderr, "%s%srefused: %s\n", prMergePrefix, subject, refusal)
		return exitRefused
	}
	fmt.Fprintf(m.stderr, "%s%s%v\n", prMergePrefix, subject, err)
	return exitIO
}

func (m prMerger) notAttempted(prs []string) {
	if len(prs) == 0 {
		return
	}
	fmt.Fprintf(m.stderr, "%snot attempted: #%s\n", prMergePrefix, strings.Join(prs, " #"))
}

func (m prMerger) planeIdle() error {
	live, err := m.liveRuns()
	if err != nil {
		return err
	}
	if len(live) == 0 {
		return nil
	}
	names := make([]string, 0, len(live))
	for _, r := range live {
		names = append(names, fmt.Sprintf("run %s (%s)", r.Lease.RunID, r.Dir))
	}
	return refusef("a loop is running: %s is live", strings.Join(names, ", "))
}

type prCommit struct {
	MessageHeadline string `json:"messageHeadline"`
	MessageBody     string `json:"messageBody"`
}

type prView struct {
	State       string     `json:"state"`
	IsDraft     bool       `json:"isDraft"`
	Mergeable   string     `json:"mergeable"`
	HeadRefOid  string     `json:"headRefOid"`
	BaseRefName string     `json:"baseRefName"`
	Body        string     `json:"body"`
	Commits     []prCommit `json:"commits"`
}

func (p prView) refusal() error {
	switch {
	case p.State != "OPEN":
		return refusef("state is %s, not OPEN", p.State)
	case p.IsDraft:
		return refusef("it is a draft")
	case p.Mergeable == "CONFLICTING":
		return refusef("it conflicts with %s", p.BaseRefName)
	}
	return nil
}

func (m prMerger) ghJSON(ctx context.Context, into any, args ...string) error {
	return decodeGH(ctx, m.gh, into, args...)
}

func decodeGH(ctx context.Context, gh func(context.Context, ...string) ([]byte, error), into any, args ...string) error {
	out, err := gh(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, into); err != nil {
		return fmt.Errorf("gh %s: decode: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (m prMerger) view(ctx context.Context, n string) (prView, error) {
	var pr prView
	if err := m.ghJSON(ctx, &pr, "pr", "view", n, "--json", "number,state,isDraft,mergeable,headRefOid,baseRefName,body,commits"); err != nil {
		return pr, err
	}
	return pr, pr.refusal()
}

func (m prMerger) behindBy(ctx context.Context, base, head string) (int, error) {
	var cmp struct {
		BehindBy int `json:"behind_by"`
	}
	err := m.ghJSON(ctx, &cmp, "api", "repos/{owner}/{repo}/compare/"+base+"..."+head)
	return cmp.BehindBy, err
}

func (m prMerger) closedWindowIdle() error {
	if m.windowOpen {
		return nil
	}
	return m.planeIdle()
}

func (m prMerger) admit(ctx context.Context, n string) error {
	err := m.planeIdle()
	var refusal prRefusal
	if !m.windowOpen || !errors.As(err, &refusal) {
		return err
	}
	d, err := decidePRWindow(ctx, m.root, prWindowArgs{pr: n}, m.gh)
	if err != nil {
		return err
	}
	if d.Verdict == mergewindow.Window {
		return nil
	}
	return refusef("%s; #%s waits for a boundary: %s", refusal, n, d.Reason)
}

func prTrailerText(pr prView) []byte {
	var b strings.Builder
	b.WriteString(pr.Body)
	for _, c := range pr.Commits {
		b.WriteString("\n\n")
		b.WriteString(c.MessageHeadline)
		b.WriteByte('\n')
		b.WriteString(c.MessageBody)
	}
	return []byte(b.String())
}

func (m prMerger) checkTrailers(n string, pr prView) error {
	_, _, violations := inboxcmd.CheckInboxTrailerBytes(m.root, prTrailerText(pr), "PR #"+n)
	if len(violations) == 0 {
		return nil
	}
	msg := strings.Join(violations, "; ")
	if m.trailerStage == "enforce" {
		return refusef("%s", msg)
	}
	fmt.Fprintf(m.stderr, "%s#%s shadow: would refuse: %s\n", prMergePrefix, n, msg)
	return nil
}

func (m prMerger) mergeOne(ctx context.Context, n string) (string, error) {
	if err := m.admit(ctx, n); err != nil {
		return "", err
	}
	pr, err := m.view(ctx, n)
	if err != nil {
		return "", err
	}
	if err := m.checkTrailers(n, pr); err != nil {
		return "", err
	}
	if pr, err = m.upToDate(ctx, n, pr); err != nil {
		return "", err
	}
	if err := m.awaitGreen(ctx, n, pr.HeadRefOid); err != nil {
		return "", err
	}
	behind, err := m.behindBy(ctx, pr.BaseRefName, pr.HeadRefOid)
	if err != nil {
		return "", err
	}
	if behind > 0 {
		return "", refusef("base moved during the wait: %s is behind %s by %d", shortSHA(pr.HeadRefOid), pr.BaseRefName, behind)
	}
	if err := m.admit(ctx, n); err != nil {
		return "", err
	}
	if _, err := m.gh(ctx, "pr", "merge", n, "--"+m.method, "--match-head-commit", pr.HeadRefOid); err != nil {
		return "", err
	}
	return m.mergedSHA(ctx, n)
}

func (m prMerger) upToDate(ctx context.Context, n string, pr prView) (prView, error) {
	behind, err := m.behindBy(ctx, pr.BaseRefName, pr.HeadRefOid)
	if err != nil || behind == 0 {
		return pr, err
	}
	if !m.updateBranch {
		return pr, refusef("behind %s by %d; rerun with --update-branch", pr.BaseRefName, behind)
	}
	if _, err := m.gh(ctx, "pr", "update-branch", n); err != nil {
		return pr, err
	}
	fmt.Fprintf(m.stderr, "%s#%s updated with %s\n", prMergePrefix, n, pr.BaseRefName)
	return m.awaitNewHead(ctx, n, pr.HeadRefOid)
}

const prHeadMoveBound = 60 * time.Second

func (m prMerger) headWaitBound() time.Duration { return max(m.wait, prHeadMoveBound) }

func (m prMerger) awaitNewHead(ctx context.Context, n, oldHead string) (prView, error) {
	bound := m.headWaitBound()
	deadline := m.now().Add(bound)
	for {
		pr, err := m.view(ctx, n)
		if err != nil {
			return pr, err
		}
		if pr.HeadRefOid != "" && pr.HeadRefOid != oldHead {
			return pr, nil
		}
		if m.now().Add(m.poll).After(deadline) {
			return pr, refusef("head is still %s after update-branch; waited %s", shortSHA(oldHead), bound)
		}
		m.sleep(m.poll)
	}
}

func (m prMerger) awaitGreen(ctx context.Context, n, head string) error {
	deadline := m.now().Add(m.wait)
	for {
		st, err := m.fetch(ctx, head)
		if err != nil {
			return err
		}
		if st.Status == ciwatch.StatusCompleted {
			if st.Conclusion == ciwatch.ConclusionSuccess {
				return nil
			}
			return refusef("required CI %s on %s: %s", st.Conclusion, shortSHA(head), failingWhat(st))
		}
		if m.now().Add(m.poll).After(deadline) {
			return refusef("required CI %s on %s; pending", st.Status, shortSHA(head))
		}
		fmt.Fprintf(m.stderr, "%s#%s waiting on required CI (%s) for %s\n", prMergePrefix, n, st.Status, shortSHA(head))
		m.sleep(m.poll)
	}
}

func failingWhat(st ciwatch.RunStatus) string {
	switch {
	case len(st.FailingJobs) > 0:
		return strings.Join(st.FailingJobs, ", ")
	case st.FailingTest != "":
		return st.FailingTest
	}
	return st.RunURL
}

func (m prMerger) mergedSHA(ctx context.Context, n string) (string, error) {
	var pr struct {
		State       string `json:"state"`
		MergeCommit *struct {
			OID string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err := m.ghJSON(ctx, &pr, "pr", "view", n, "--json", "state,mergeCommit"); err != nil {
		return "", err
	}
	if pr.State != "MERGED" || pr.MergeCommit == nil || !fullCommit.MatchString(pr.MergeCommit.OID) {
		return "", fmt.Errorf("gh reported the merge but #%s is %s with no merge commit", n, pr.State)
	}
	return pr.MergeCommit.OID, nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
