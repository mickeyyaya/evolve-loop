package ciwatch

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
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
	root   string
	module string
}

func NewGHClassifySource(repoRoot, modulePath string) ClassifySource {
	return ghClassifySource{root: repoRoot, module: modulePath}
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

func ResolveCommit(ctx context.Context, repoRoot, ref string) (string, error) {
	sha, err := ghClassifySource{root: repoRoot}.commitSHA(ctx, ref)
	return checkedSHA("commit "+ref, sha, err)
}

func ResolvePRHead(ctx context.Context, repoRoot, pr string) (string, error) {
	var doc struct {
		HeadRefOid string `json:"headRefOid"`
	}
	err := ghClassifySource{root: repoRoot}.ghJSON(ctx, &doc, "pr", "view", pr, "--json", "headRefOid")
	return checkedSHA("PR "+pr, doc.HeadRefOid, err)
}

func checkedSHA(what, sha string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if !isHex(sha, 40, 40) {
		return "", fmt.Errorf("ciwatch: %s resolved to %q, not a commit SHA", what, sha)
	}
	return strings.ToLower(sha), nil
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
	truth, withJSON, err := s.testJSONFailures(ctx, st)
	if err != nil {
		return RunStatus{}, err
	}
	log := string(out)
	st.FailingJobs = failedJobNames(log)
	if len(withJSON) > 0 {
		st.FailingTests = attributeFailures(log, truth, withJSON)
	} else {
		st.FailingTests = parseFailingTests(log)
	}
	return st, nil
}

const unknownStep = "UNKNOWN STEP"

var makeErrorLine = regexp.MustCompile(`^make(?:\[\d+\])?: \*\*\* \[(?:[^\]]*:\d+: )?([^\]]+)\] Error \d+`)

var uploadedArtifactLine = regexp.MustCompile(`^Artifact (\S+) has been successfully uploaded!`)

func logText(line string) (job, step, text string, ok bool) {
	job, rest, ok := strings.Cut(line, "\t")
	if !ok || job == "" {
		return "", "", "", false
	}
	if name, after, stepped := strings.Cut(rest, "\t"); stepped {
		step, rest = name, after
	}
	if stamp, after, spaced := strings.Cut(rest, " "); spaced {
		if _, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
			rest = after
		}
	}
	return job, step, strings.TrimSpace(rest), true
}

type failureParse struct {
	found   map[string]*FailingTest
	pending map[string][]string
	named   map[string]bool
	jobs    []string
	makes   map[string]string
	steps   map[string]string
	uploads map[string]string
}

func scanLog(log string) *failureParse {
	p := &failureParse{found: map[string]*FailingTest{}, pending: map[string][]string{}, named: map[string]bool{},
		makes: map[string]string{}, steps: map[string]string{}, uploads: map[string]string{}}
	for _, line := range strings.Split(log, "\n") {
		if job, step, text, ok := logText(line); ok {
			p.line(job, step, text)
		}
	}
	return p
}

func (p *failureParse) add(row FailingTest, job string) {
	p.named[job] = true
	row.Jobs = []string{job}
	key := failureKey(row)
	if prev, ok := p.found[key]; ok {
		if !slices.Contains(prev.Jobs, job) {
			prev.Jobs = append(prev.Jobs, job)
		}
		return
	}
	p.found[key] = &row
}

func (p *failureParse) note(job, step, text string) {
	if !slices.Contains(p.jobs, job) {
		p.jobs = append(p.jobs, job)
	}
	if m := makeErrorLine.FindStringSubmatch(text); m != nil && p.makes[job] == "" {
		p.makes[job] = "make " + m[1]
	}
	if m := uploadedArtifactLine.FindStringSubmatch(text); m != nil {
		p.uploads[job] = m[1]
	}
	if step != "" && step != unknownStep && p.steps[job] == "" {
		p.steps[job] = step
	}
}

func (p *failureParse) line(job, step, text string) {
	p.note(job, step, text)
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
		p.add(FailingTest{Package: fields[1]}, job)
	}
	for _, test := range tests {
		p.add(FailingTest{Package: fields[1], Test: test}, job)
	}
}

func (p *failureParse) addUnnamedJobs() {
	for _, job := range p.jobs {
		if !p.named[job] {
			p.add(FailingTest{Step: cmp.Or(p.makes[job], p.steps[job])}, job)
		}
	}
}

func (p *failureParse) addPending() {
	for _, job := range p.jobs {
		for _, test := range p.pending[job] {
			p.add(FailingTest{Test: test}, job)
		}
	}
}

func parseFailingTests(log string) []FailingTest {
	p := scanLog(log)
	p.addPending()
	p.addUnnamedJobs()
	return sortedFailures(p.found)
}

func attributeFailures(log string, truth []FailingTest, withJSON map[string]bool) []FailingTest {
	p := scanLog(log)
	p.addPending()
	text := sortedFailures(p.found)
	p.found, p.named = map[string]*FailingTest{}, map[string]bool{}
	for _, f := range truth {
		for _, job := range p.uploaders(f.Jobs[0]) {
			p.add(FailingTest{Package: f.Package, Test: f.Test}, job)
		}
	}
	for _, row := range text {
		for _, job := range row.Jobs {
			if !withJSON[p.uploads[job]] {
				p.add(FailingTest{Package: row.Package, Test: row.Test}, job)
			}
		}
	}
	p.addUnnamedJobs()
	return sortedFailures(p.found)
}

func (p *failureParse) uploaders(artifact string) []string {
	var jobs []string
	for _, job := range p.jobs {
		if p.uploads[job] == artifact {
			jobs = append(jobs, job)
		}
	}
	if len(jobs) == 0 {
		return []string{artifact}
	}
	return jobs
}

func sortedFailures(found map[string]*FailingTest) []FailingTest {
	out := make([]FailingTest, 0, len(found))
	for _, f := range found {
		slices.Sort(f.Jobs)
		out = append(out, *f)
	}
	slices.SortFunc(out, func(a, b FailingTest) int {
		return cmp.Or(cmp.Compare(a.Package, b.Package), cmp.Compare(a.Test, b.Test), cmp.Compare(a.Step, b.Step), cmp.Compare(a.Jobs[0], b.Jobs[0]))
	})
	return out
}
