package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	prUsage       = "usage: evolve pr merge <n>... [--update-branch] [--wait D] [--project-root P]"
	exitUsage     = 10
	exitIO        = 2
	exitRefused   = 1
	prMergePrefix = "evolve pr merge: "
)

var (
	prNumber   = regexp.MustCompile(`^[1-9][0-9]*$`)
	fullCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type cliFlags struct {
	bools  map[string]*bool
	values map[string]*string
}

func (f cliFlags) parse(args []string) ([]string, error) {
	var operands []string
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		if b, ok := f.bools[name]; ok && !inline {
			*b = true
			continue
		}
		if v, ok := f.values[name]; ok {
			if !inline {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("%s needs a value", name)
				}
				i++
				value = args[i]
			}
			*v = value
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			return nil, fmt.Errorf("unknown flag %q", args[i])
		}
		operands = append(operands, args[i])
	}
	return operands, nil
}

type prMergeArgs struct {
	prs          []string
	updateBranch bool
	wait         time.Duration
	projectRoot  string
}

func parsePRMergeArgs(args []string) (prMergeArgs, error) {
	var a prMergeArgs
	var wait string
	operands, err := cliFlags{
		bools:  map[string]*bool{"--update-branch": &a.updateBranch},
		values: map[string]*string{"--wait": &wait, "--project-root": &a.projectRoot},
	}.parse(args)
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

func runPR(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "merge" {
		fmt.Fprintln(stderr, prUsage)
		return exitUsage
	}
	a, err := parsePRMergeArgs(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", prMergePrefix, err, prUsage)
		return exitUsage
	}
	root, err := loopStopRoot(a.projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prMergePrefix, err)
		return exitIO
	}
	m, err := newPRMerger(root, a, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prMergePrefix, err)
		return exitIO
	}
	return m.mergeAll(context.Background(), a.prs)
}

type prMerger struct {
	method       string
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
	return prMerger{
		method: method, poll: time.Duration(*cw.PollS) * time.Second, wait: a.wait, updateBranch: a.updateBranch,
		gh: ghIn(root), fetch: ciwatch.NewGHFetcher(root), liveRuns: func() ([]runlease.LiveRun, error) { return planeLiveRuns(root, time.Now()) },
		now: time.Now, sleep: time.Sleep, stdout: stdout, stderr: stderr,
	}, nil
}

func ghIn(root string) func(ctx context.Context, args ...string) ([]byte, error) {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "gh", args...)
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
}

func planeLiveRuns(root string, now time.Time) ([]runlease.LiveRun, error) {
	out, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list in %s: %w", root, err)
	}
	trees := []string{root}
	for _, line := range strings.Split(string(out), "\n") {
		if tree, ok := strings.CutPrefix(line, "worktree "); ok {
			trees = append(trees, tree)
		}
	}
	var live []runlease.LiveRun
	seen := map[string]bool{}
	for _, tree := range trees {
		runs := filepath.Join(paths.EvolveDirOf(tree), "runs")
		if real, err := filepath.EvalSymlinks(runs); err == nil {
			runs = real
		}
		if seen[runs] {
			continue
		}
		seen[runs] = true
		live = append(live, runlease.LiveRuns(runs, now)...)
	}
	return live, nil
}

type prRefusal string

func (r prRefusal) Error() string { return string(r) }

func refusef(format string, args ...any) error { return prRefusal(fmt.Sprintf(format, args...)) }

func (m prMerger) mergeAll(ctx context.Context, prs []string) int {
	if err := m.planeIdle(); err != nil {
		code := m.report("", err)
		m.notAttempted(prs)
		return code
	}
	for i, n := range prs {
		sha, err := m.mergeOne(ctx, n)
		if err != nil {
			code := m.report("#"+n+" ", err)
			m.notAttempted(prs[i+1:])
			return code
		}
		fmt.Fprintf(m.stdout, "merged #%s %s\n", n, sha)
	}
	return 0
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

type prView struct {
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	Mergeable   string `json:"mergeable"`
	HeadRefOid  string `json:"headRefOid"`
	BaseRefName string `json:"baseRefName"`
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
	out, err := m.gh(ctx, args...)
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
	if err := m.ghJSON(ctx, &pr, "pr", "view", n, "--json", "number,state,isDraft,mergeable,headRefOid,baseRefName"); err != nil {
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

func (m prMerger) mergeOne(ctx context.Context, n string) (string, error) {
	pr, err := m.view(ctx, n)
	if err != nil {
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
	if err := m.planeIdle(); err != nil {
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
	return m.view(ctx, n)
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
