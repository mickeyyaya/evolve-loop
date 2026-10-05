package ciwatch

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

const (
	classifyRunFields = "attempt,conclusion,createdAt,databaseId,headSha,status,url"
	mainBranch        = "main"
	compareFileCap    = 300
)

type ghRunDoc struct {
	Attempt    int       `json:"attempt"`
	Conclusion string    `json:"conclusion"`
	CreatedAt  time.Time `json:"createdAt"`
	DatabaseID int64     `json:"databaseId"`
	HeadSHA    string    `json:"headSha"`
	Status     string    `json:"status"`
	URL        string    `json:"url"`
}

func (d ghRunDoc) status() RunStatus {
	return RunStatus{Status: d.Status, Conclusion: d.Conclusion, RunURL: d.URL, RunID: d.DatabaseID,
		HeadSHA: d.HeadSHA, Attempt: d.Attempt, CreatedAt: d.CreatedAt}
}

type ghCompareDoc struct {
	Status    string `json:"status"`
	MergeBase struct {
		SHA string `json:"sha"`
	} `json:"merge_base_commit"`
	Files []struct {
		Filename string `json:"filename"`
	} `json:"files"`
}

type ghClassifySource struct {
	root string
}

func NewGHClassifySource(repoRoot string) ClassifySource {
	return ghClassifySource{root: repoRoot}
}

func (s ghClassifySource) gh(ctx context.Context, args ...string) ([]byte, error) {
	out, err := execCapture(ctx, s.root, "gh", args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (s ghClassifySource) ghJSON(ctx context.Context, into any, args ...string) error {
	out, err := s.gh(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, into); err != nil {
		return fmt.Errorf("gh %s: decode: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (s ghClassifySource) Resolve(ctx context.Context, t Target) (Change, error) {
	ch := Change{Target: t}
	var err error
	switch t.Kind {
	case TargetRun:
		ch.Run, err = s.runView(ctx, t.Value)
		ch.HeadSHA = ch.Run.HeadSHA
	case TargetPR:
		return s.resolvePR(ctx, t)
	case TargetSHA:
		if ch.HeadSHA, err = s.commitSHA(ctx, t.Value); err == nil {
			ch.Run, err = s.LatestRunOn(ctx, ch.HeadSHA)
		}
	default:
		return ch, fmt.Errorf("%w: kind %q", ErrBadTarget, t.Kind)
	}
	if err != nil || ch.HeadSHA == "" {
		return ch, err
	}
	ch.BaseSHA, ch.Files, ch.Truncated, err = s.mainBase(ctx, ch.HeadSHA)
	return ch, err
}

func (s ghClassifySource) resolvePR(ctx context.Context, t Target) (Change, error) {
	var pr struct {
		HeadRefOid  string `json:"headRefOid"`
		BaseRefName string `json:"baseRefName"`
	}
	ch := Change{Target: t}
	if err := s.ghJSON(ctx, &pr, "pr", "view", t.Value, "--json", "headRefOid,baseRefName"); err != nil {
		return ch, err
	}
	ch.HeadSHA = pr.HeadRefOid
	diff, err := s.compare(ctx, pr.BaseRefName, ch.HeadSHA)
	if err != nil {
		return ch, err
	}
	ch.BaseSHA, ch.Files, ch.Truncated = diff.MergeBase.SHA, diff.filenames(), len(diff.Files) >= compareFileCap
	ch.Run, err = s.LatestRunOn(ctx, ch.HeadSHA)
	return ch, err
}

func (s ghClassifySource) mainBase(ctx context.Context, head string) (string, []string, bool, error) {
	onMain, err := s.compare(ctx, mainBranch, head)
	if err != nil {
		return "", nil, false, err
	}
	base := onMain.MergeBase.SHA
	if onMain.Status == "identical" || onMain.Status == "behind" {
		if base, err = s.firstParent(ctx, head); err != nil || base == "" {
			return base, nil, false, err
		}
		if onMain, err = s.compare(ctx, base, head); err != nil {
			return "", nil, false, err
		}
	}
	return base, onMain.filenames(), len(onMain.Files) >= compareFileCap, nil
}

func (d ghCompareDoc) filenames() []string {
	names := make([]string, 0, len(d.Files))
	for _, f := range d.Files {
		names = append(names, f.Filename)
	}
	return names
}

func (s ghClassifySource) compare(ctx context.Context, base, head string) (ghCompareDoc, error) {
	var doc ghCompareDoc
	err := s.ghJSON(ctx, &doc, "api", "repos/{owner}/{repo}/compare/"+base+"..."+head)
	return doc, err
}

type ghCommitDoc struct {
	SHA     string `json:"sha"`
	Parents []struct {
		SHA string `json:"sha"`
	} `json:"parents"`
}

func (s ghClassifySource) commit(ctx context.Context, ref string) (ghCommitDoc, error) {
	var doc ghCommitDoc
	err := s.ghJSON(ctx, &doc, "api", "repos/{owner}/{repo}/commits/"+ref)
	return doc, err
}

func (s ghClassifySource) commitSHA(ctx context.Context, ref string) (string, error) {
	doc, err := s.commit(ctx, ref)
	return doc.SHA, err
}

func (s ghClassifySource) firstParent(ctx context.Context, sha string) (string, error) {
	doc, err := s.commit(ctx, sha)
	if err != nil || len(doc.Parents) == 0 {
		return "", err
	}
	return doc.Parents[0].SHA, nil
}

func (s ghClassifySource) runView(ctx context.Context, id string) (RunStatus, error) {
	var doc ghRunDoc
	if err := s.ghJSON(ctx, &doc, "run", "view", id, "--json", classifyRunFields); err != nil {
		return RunStatus{}, err
	}
	return s.withFailures(ctx, doc.status())
}

func (s ghClassifySource) LatestRunOn(ctx context.Context, sha string) (RunStatus, error) {
	runs, err := s.listRuns(ctx, "--commit", sha, "--limit", "1")
	if err != nil || len(runs) == 0 {
		return RunStatus{Status: "queued"}, err
	}
	return runs[0], nil
}

func (s ghClassifySource) MainRunsBefore(ctx context.Context, before time.Time, limit int) ([]RunStatus, error) {
	return s.listRuns(ctx, "--branch", mainBranch, "--status", StatusCompleted,
		"--created", "<"+before.UTC().Format(time.RFC3339), "--limit", strconv.Itoa(limit))
}

func (s ghClassifySource) listRuns(ctx context.Context, filters ...string) ([]RunStatus, error) {
	var docs []ghRunDoc
	args := slices.Concat([]string{"run", "list", "--workflow", ciparity.RequiredWorkflow}, filters, []string{"--json", classifyRunFields})
	if err := s.ghJSON(ctx, &docs, args...); err != nil {
		return nil, err
	}
	runs := make([]RunStatus, 0, len(docs))
	for _, d := range docs {
		st, err := s.withFailures(ctx, d.status())
		if err != nil {
			return nil, err
		}
		runs = append(runs, st)
	}
	return runs, nil
}

func (s ghClassifySource) RerunFailed(ctx context.Context, runID int64, afterAttempt int, timeout, poll time.Duration) (RunStatus, error) {
	id := strconv.FormatInt(runID, 10)
	if _, err := s.gh(ctx, "run", "rerun", id, "--failed"); err != nil {
		return RunStatus{}, err
	}
	deadline := time.Now().Add(timeout)
	for {
		var doc ghRunDoc
		if err := s.ghJSON(ctx, &doc, "run", "view", id, "--json", classifyRunFields); err != nil {
			return RunStatus{}, err
		}
		st := doc.status()
		if st.Attempt > afterAttempt && st.Status == StatusCompleted {
			return s.withFailures(ctx, st)
		}
		if time.Now().Add(poll).After(deadline) {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return RunStatus{}, ctx.Err()
		case <-time.After(poll):
		}
	}
}

func (s ghClassifySource) withFailures(ctx context.Context, st RunStatus) (RunStatus, error) {
	if st.Status != StatusCompleted || st.Conclusion == ConclusionSuccess {
		return st, nil
	}
	args := []string{"run", "view", strconv.FormatInt(st.RunID, 10), "--log-failed"}
	if st.Attempt > 0 {
		args = append(args, "--attempt", strconv.Itoa(st.Attempt))
	}
	out, err := s.gh(ctx, args...)
	if err != nil {
		return RunStatus{}, err
	}
	st.FailingTests, st.FailingJobs = parseFailingTests(string(out)), failedJobNames(string(out))
	return st, nil
}

func logText(line string) (job, text string, ok bool) {
	job, rest, ok := strings.Cut(line, "\t")
	if !ok || job == "" {
		return "", "", false
	}
	if _, after, stepped := strings.Cut(rest, "\t"); stepped {
		rest = after
	}
	if stamp, after, spaced := strings.Cut(rest, " "); spaced {
		if _, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			rest = after
		}
	}
	return job, strings.TrimSpace(rest), true
}

type failureParse struct {
	found   map[string]*FailingTest
	pending map[string][]string
	named   map[string]bool
	jobs    []string
}

func (p *failureParse) add(pkg, test, job string) {
	p.named[job] = true
	f := FailingTest{Package: pkg, Test: test, Jobs: []string{job}}
	key := failureKey(f)
	if prev, ok := p.found[key]; ok {
		if !slices.Contains(prev.Jobs, job) {
			prev.Jobs = append(prev.Jobs, job)
		}
		return
	}
	p.found[key] = &f
}

func (p *failureParse) line(job, text string) {
	if !slices.Contains(p.jobs, job) {
		p.jobs = append(p.jobs, job)
	}
	if name, isFail := strings.CutPrefix(text, "--- FAIL: "); isFail {
		if fields := strings.Fields(name); len(fields) > 0 {
			top, _, _ := strings.Cut(fields[0], "/")
			if !slices.Contains(p.pending[job], top) {
				p.pending[job] = append(p.pending[job], top)
			}
		}
		return
	}
	fields := strings.Fields(text)
	if len(fields) < 2 || !strings.HasPrefix(text, "FAIL\t") {
		return
	}
	tests := p.pending[job]
	delete(p.pending, job)
	if len(tests) == 0 {
		p.add(fields[1], "", job)
	}
	for _, test := range tests {
		p.add(fields[1], test, job)
	}
}

func parseFailingTests(log string) []FailingTest {
	p := failureParse{found: map[string]*FailingTest{}, pending: map[string][]string{}, named: map[string]bool{}}
	for _, line := range strings.Split(log, "\n") {
		if job, text, ok := logText(line); ok {
			p.line(job, text)
		}
	}
	for _, job := range p.jobs {
		for _, test := range p.pending[job] {
			p.add("", test, job)
		}
		if !p.named[job] {
			p.add("", "", job)
		}
	}
	return sortedFailures(p.found)
}

func sortedFailures(found map[string]*FailingTest) []FailingTest {
	out := make([]FailingTest, 0, len(found))
	for _, f := range found {
		slices.Sort(f.Jobs)
		out = append(out, *f)
	}
	slices.SortFunc(out, func(a, b FailingTest) int {
		return cmp.Or(cmp.Compare(a.Package, b.Package), cmp.Compare(a.Test, b.Test), cmp.Compare(a.Jobs[0], b.Jobs[0]))
	})
	return out
}
