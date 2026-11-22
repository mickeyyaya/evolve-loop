package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type GhFunc func(ctx context.Context, dir string, args ...string) (string, int, error)

const (
	ghBin       = "gh"
	mainBranch  = "main"
	prTitleStem = "evolve: integrate "
	redExitCode = 1

	registerTries   = 5
	runsOfASHA      = 20
	watchTries      = 720
	readyLimit      = 100
	statusCompleted = "completed"
	conclusionGreen = "success"
	conclusionRed   = "failure"
	batchHeadPrefix = "integration/"
)

var (
	ErrRunTimeout     = errors.New("integration: CI run timed out")
	ErrRunPending     = errors.New("integration: the CI run is not complete")
	ErrNoRun          = errors.New("integration: no CI run")
	errMissingHeadOID = errors.New("integration: gh pr view returned no headRefOid")
	prURL             = regexp.MustCompile(`/pull/([0-9]+)\s*$`)
)

func SysGh() GhFunc {
	return func(ctx context.Context, dir string, args ...string) (string, int, error) {
		var out strings.Builder
		code, err := sysexec.DefaultRunner(ctx, ghBin, dir, args, nil, nil, &out, nil)
		return out.String(), code, err
	}
}

type GhCI struct {
	Repo   Repo
	Gh     GhFunc
	Settle func(ctx context.Context) error
	Runs   *RunLog
}

func (c GhCI) gh(ctx context.Context, args ...string) (string, error) {
	out, code, err := c.Gh(ctx, c.Repo.Dir, args...)
	if err != nil {
		return "", fmt.Errorf("integration: gh %s: %w", strings.Join(args, " "), err)
	}
	if code != 0 {
		return "", fmt.Errorf("integration: gh %s exit=%d", strings.Join(args, " "), code)
	}
	return out, nil
}

func (c GhCI) decode(ctx context.Context, into any, args ...string) error {
	out, err := c.gh(ctx, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(out), into); err != nil {
		return fmt.Errorf("integration: gh %s printed no JSON of the expected shape: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (c GhCI) Open(ctx context.Context, b Batch) (PR, error) {
	if err := b.check(); err != nil {
		return PR{}, err
	}
	lease := []string{"push", "--force-with-lease", "origin", b.Tip + ":" + branchRef(b.Branch)}
	if _, err := c.Repo.Git(ctx, c.Repo.Dir, "", lease...); err != nil {
		return PR{}, fmt.Errorf("integration: push %s: %w", b.Branch, err)
	}
	var open []struct {
		Number int `json:"number"`
	}
	if err := c.decode(ctx, &open, "pr", "list", "--head", b.Branch, "--state", "open", "--json", "number"); err != nil {
		return PR{}, err
	}
	if len(open) > 0 {
		return PR{Number: open[0].Number, Tip: b.Tip}, nil
	}
	return c.create(ctx, b)
}

func (c GhCI) create(ctx context.Context, b Batch) (PR, error) {
	body := "Batch " + b.Branch + " of wave " + b.Wave + " at " + b.Tip
	out, err := c.gh(ctx, "pr", "create", "--base", mainBranch, "--head", b.Branch, "--title", prTitleStem+b.Branch, "--body", body)
	if err != nil {
		return PR{}, err
	}
	m := prURL.FindStringSubmatch(out)
	if m == nil {
		return PR{}, fmt.Errorf("integration: gh pr create printed no PR url: %q", out)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return PR{}, fmt.Errorf("integration: %q is not a PR number", m[1])
	}
	return PR{Number: n, Tip: b.Tip}, nil
}

func (c GhCI) runIDs(ctx context.Context, tip string) ([]string, error) {
	for attempt := 1; ; attempt++ {
		var runs []struct {
			DatabaseID int `json:"databaseId"`
		}
		if err := c.decode(ctx, &runs, "run", "list", "--workflow", ciparity.RequiredWorkflow, "--commit", tip, "--limit", strconv.Itoa(runsOfASHA), "--json", "databaseId"); err != nil {
			return nil, err
		}
		if len(runs) > 0 {
			ids := make([]string, 0, len(runs))
			for _, r := range runs {
				ids = append(ids, strconv.Itoa(r.DatabaseID))
			}
			return ids, nil
		}
		if c.Settle == nil || attempt >= registerTries {
			return nil, fmt.Errorf("%w of %s for the tip %s", ErrNoRun, ciparity.RequiredWorkflow, tip)
		}
		if err := c.Settle(ctx); err != nil {
			return nil, err
		}
	}
}

func (c GhCI) runID(ctx context.Context, tip string) (string, error) {
	ids, err := c.runIDs(ctx, tip)
	if err != nil {
		return "", err
	}
	return ids[0], nil
}

func (c GhCI) tipVerdict(ctx context.Context, ids []string) (Verdict, error) {
	newest, err := c.verdict(ctx, ids[0])
	var ended *RunConclusionError
	if err != nil && !errors.As(err, &ended) {
		return "", err
	}
	if newest == Red {
		return Red, nil
	}
	settled := c
	settled.Settle = nil
	green := newest == Green
	for _, id := range ids[1:] {
		v, e := settled.verdict(ctx, id)
		switch {
		case v == Red:
			return Red, nil
		case v == Green:
			green = true
		case e != nil && !errors.Is(e, ErrRunPending) && !errors.As(e, &ended):
			return "", e
		}
	}
	if green {
		return Green, nil
	}
	return "", err
}

func (c GhCI) Watch(ctx context.Context, pr PR) (Verdict, error) {
	ids, err := c.runIDs(ctx, pr.Tip)
	if err != nil {
		return "", err
	}
	id := ids[0]
	c.Runs.note(pr.Tip, id)
	_, code, err := c.Gh(ctx, c.Repo.Dir, "run", "watch", id, "--exit-status")
	switch {
	case err != nil:
		return "", fmt.Errorf("integration: gh run watch %s: %w", id, err)
	case code != 0 && code != redExitCode:
		return "", fmt.Errorf("integration: gh run watch %s exit=%d", id, code)
	}
	return c.tipVerdict(ctx, ids)
}

func (c GhCI) verdict(ctx context.Context, id string) (Verdict, error) {
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return "", fmt.Errorf("integration: run %s wait canceled: %w", id, errors.Join(ctx.Err(), ErrRunTimeout))
		}
		var run struct {
			Conclusion string `json:"conclusion"`
			Status     string `json:"status"`
		}
		if err := c.decode(ctx, &run, "run", "view", id, "--json", "conclusion,status"); err != nil {
			return "", err
		}
		if run.Status != "" && run.Status != statusCompleted {
			if c.Settle == nil {
				return "", fmt.Errorf("integration: run %s is %s: %w", id, run.Status, ErrRunPending)
			}
			if attempt >= watchTries {
				return "", fmt.Errorf("integration: run %s is %s: %w", id, run.Status, ErrRunTimeout)
			}
			if err := c.Settle(ctx); err != nil {
				return "", fmt.Errorf("integration: run %s wait: %w", id, errors.Join(err, ErrRunTimeout))
			}
			continue
		}
		switch run.Conclusion {
		case conclusionGreen:
			return Green, nil
		case conclusionRed:
			return Red, nil
		}
		return "", &RunConclusionError{Run: id, Conclusion: run.Conclusion}
	}
}

type RunConclusionError struct {
	Run        string
	Conclusion string
}

func (e *RunConclusionError) Error() string {
	return fmt.Sprintf("integration: run %s ended with the conclusion %q, which is neither green nor red", e.Run, e.Conclusion)
}

type PeekCI struct {
	CI GhCI
}

func (p PeekCI) Watch(ctx context.Context, pr PR) (Verdict, error) {
	c := p.CI
	c.Settle = nil
	ids, err := c.runIDs(ctx, pr.Tip)
	if err != nil {
		return "", err
	}
	return c.tipVerdict(ctx, ids)
}

func (c GhCI) Rerun(ctx context.Context, pr PR) error {
	id, err := c.runID(ctx, pr.Tip)
	if err != nil {
		return err
	}
	_, err = c.gh(ctx, "run", "rerun", id, "--failed")
	return err
}

type GhHost struct {
	CI GhCI
}

func (h GhHost) Ready(ctx context.Context) ([]int, error) {
	var open []struct {
		Number      int    `json:"number"`
		HeadRefName string `json:"headRefName"`
	}
	if err := h.CI.decode(ctx, &open, "pr", "list", "--state", "open", "--base", mainBranch, "--search", "draft:false", "--limit", strconv.Itoa(readyLimit), "--json", "number,headRefName"); err != nil {
		return nil, err
	}
	if len(open) >= readyLimit {
		return nil, fmt.Errorf("integration: gh listed %d open PRs, the most it lists; name the PRs to land", readyLimit)
	}
	var prs []int
	for _, p := range open {
		if !strings.HasPrefix(p.HeadRefName, batchHeadPrefix) {
			prs = append(prs, p.Number)
		}
	}
	return prs, nil
}

func (h GhHost) Comment(ctx context.Context, pr int, body string) error {
	_, err := h.CI.gh(ctx, "pr", "comment", strconv.Itoa(pr), "--body", body)
	return err
}

func (h GhHost) Close(ctx context.Context, pr int, body string) error {
	_, err := h.CI.gh(ctx, "pr", "close", strconv.Itoa(pr), "--comment", body)
	return err
}

func (h GhHost) State(ctx context.Context, pr int) (PRState, error) {
	var view struct {
		State PRState `json:"state"`
	}
	if err := h.CI.decode(ctx, &view, "pr", "view", strconv.Itoa(pr), "--json", "state"); err != nil {
		return "", err
	}
	return view.State, nil
}

func (h GhHost) OpenBatches(ctx context.Context) ([]int, error) {
	var open []struct {
		Number      int    `json:"number"`
		HeadRefName string `json:"headRefName"`
	}
	if err := h.CI.decode(ctx, &open, "pr", "list", "--state", "open", "--base", mainBranch, "--limit", strconv.Itoa(readyLimit), "--json", "number,headRefName"); err != nil {
		return nil, err
	}
	if len(open) >= readyLimit {
		return nil, fmt.Errorf("integration: gh listed %d open PRs, the most it lists; close the batch PRs that are done", readyLimit)
	}
	var prs []int
	for _, p := range open {
		if strings.HasPrefix(p.HeadRefName, batchHeadPrefix) {
			prs = append(prs, p.Number)
		}
	}
	return prs, nil
}

func (h GhHost) HeadOID(ctx context.Context, pr int) (string, error) {
	var view struct {
		HeadRefOid string `json:"headRefOid"`
	}
	if err := h.CI.decode(ctx, &view, "pr", "view", strconv.Itoa(pr), "--json", "headRefOid"); err != nil {
		return "", err
	}
	if view.HeadRefOid == "" {
		return "", fmt.Errorf("%w: PR #%d", errMissingHeadOID, pr)
	}
	return view.HeadRefOid, nil
}
