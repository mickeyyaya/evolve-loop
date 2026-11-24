package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	boundaryRatchetVerb   = "ratchet-tighten"
	boundaryRatchetDesc   = "propose the tightened size-ratchet allowances as a pull request"
	ratchetTightenTitle   = "chore: tighten the size-ratchet allowances"
	ratchetTightenMessage = ratchetTightenTitle + "\n\nCloses-Inbox: none"
	ratchetTightenBody    = "The boundary step ratchet-tighten wrote this list from origin/main. It lands at a wave boundary through the integration batch like every other pull request.\n\nCloses-Inbox: none\n\n"
	ratchetBranchPrefix   = "chore/ratchet-tighten-"
	ratchetPRBase         = "main"
	ratchetGitTimeout     = 2 * time.Minute
)

var (
	ratchetRun   sysexec.RunFunc = sysexec.DefaultRunner
	ratchetWrite                 = os.WriteFile
	ratchetRead                  = os.ReadFile
)

func runBoundaryRatchetTighten(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var projectRoot string
	if _, err := cmdutil.Flags().Values(map[string]*string{"--project-root": &projectRoot}).Parse(args); err != nil || projectRoot == "" {
		fmt.Fprintf(stderr, "%s%s needs --project-root\n", boundaryPrefix, boundaryRatchetVerb)
		return exitUsage
	}
	if err := landRatchetTighten(projectRoot, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "%sWARN: ratchet tighten: %v\n", boundaryPrefix, err)
	}
	return 0
}

func landRatchetTighten(root string, stdout, stderr io.Writer) (err error) {
	if err := refuseBoundaryWrite(root, os.Getenv, time.Now(), "the offender list"); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "evolve-ratchet-tighten-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	wt := filepath.Join(dir, "main")
	if err := runGitSteps(root, []string{"fetch", "--quiet", "origin", "main"}, []string{"worktree", "add", "--quiet", "--detach", wt, "origin/main"}); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, runGitSteps(root, []string{"worktree", "remove", "--force", wt})) }()
	return ratchetRound{root: root, wt: wt, stdout: stdout, stderr: stderr}.run()
}

type tightenedList struct {
	rel     string
	changes []string
	encoded []byte
}

type ratchetList struct {
	rel     string
	tighten func(string) ([]string, []byte, error)
}

var ratchetLists = []ratchetList{
	{sizeratchet.OffendersRelPath, sizeratchet.TightenModule},
	{rawgitratchet.BaselineRelPath, rawgitratchet.TightenModule},
}

func ratchetRepoPath(rel string) string { return "go/" + rel }

func regenerateLists(module string) ([]tightenedList, error) {
	var lists []tightenedList
	for _, t := range ratchetLists {
		changes, encoded, err := t.tighten(module)
		if err != nil {
			return nil, err
		}
		if len(changes) == 0 {
			if encoded, err = ratchetRead(filepath.Join(module, filepath.FromSlash(t.rel))); err != nil {
				return nil, err
			}
		}
		lists = append(lists, tightenedList{rel: t.rel, changes: changes, encoded: encoded})
	}
	return lists, nil
}

func changedLists(all []tightenedList) []tightenedList {
	var changed []tightenedList
	for _, l := range all {
		if len(l.changes) > 0 {
			changed = append(changed, l)
		}
	}
	return changed
}

func writeTightenedLists(module string, lists []tightenedList) (paths, changes []string, err error) {
	for _, l := range lists {
		if werr := ratchetWrite(filepath.Join(module, filepath.FromSlash(l.rel)), l.encoded, 0o644); werr != nil {
			return nil, nil, werr
		}
		paths = append(paths, ratchetRepoPath(l.rel))
		changes = append(changes, l.changes...)
	}
	return paths, changes, runRatchetScans(module)
}

type ratchetOutcome struct {
	tight      bool
	skipped    bool
	head       string
	pr         ratchetPR
	content    string
	scanReason string
	ambiguous  bool
}

func tightenAndPropose(wt string, stdout io.Writer, cand ratchetCandidate) (ratchetOutcome, error) {
	module := filepath.Join(wt, "go")
	all, err := regenerateLists(module)
	if err != nil {
		return ratchetOutcome{}, err
	}
	changed := changedLists(all)
	if len(changed) == 0 {
		fmt.Fprintln(stdout, "ratchet tighten: tight")
		return ratchetOutcome{tight: true}, nil
	}
	paths, changes, err := writeTightenedLists(module, changed)
	var scanErr *ratchetScanError
	if errors.As(err, &scanErr) {
		return ratchetOutcome{scanReason: scanErr.reason}, err
	}
	if err != nil {
		return ratchetOutcome{}, err
	}
	if cand.unchangedBy(wt, all) {
		fmt.Fprintf(stdout, "ratchet tighten: unchanged, kept #%d\n", cand.pr.Number)
		return ratchetOutcome{skipped: true, head: cand.pr.Head, pr: cand.pr, ambiguous: cand.ambiguous}, nil
	}
	return pushTighten(wt, stdout, cand.pr, tightenPush{paths: paths, changes: changes, all: all})
}

type tightenPush struct {
	paths, changes []string
	all            []tightenedList
}

func pushTighten(wt string, stdout io.Writer, pr ratchetPR, push tightenPush) (ratchetOutcome, error) {
	err := commitAndPushTightenBranch(wt, pr.HeadRefName, push.paths)
	if err == nil && pr.Number == 0 {
		_, err = ratchetCmd(wt, "gh", "pr", "create", "--base", ratchetPRBase, "--head", pr.HeadRefName,
			"--title", ratchetTightenTitle, "--body", ratchetTightenBody+strings.Join(push.changes, "\n"))
	}
	if err != nil {
		return ratchetOutcome{}, err
	}
	fmt.Fprintln(stdout, strings.Join(push.changes, "\n"))
	fmt.Fprintln(stdout, pr.report())
	head, err := ratchetCmd(wt, "git", "rev-parse", "HEAD")
	if err != nil {
		return ratchetOutcome{}, err
	}
	listed := findRatchetCandidate(wt)
	return ratchetOutcome{head: head, pr: listed.pr, ambiguous: listed.ambiguous, content: ratchetHeadMatches(wt, head, push.all)}, nil
}

type ratchetPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	Fork        bool   `json:"isCrossRepository"`
	Draft       bool   `json:"isDraft"`
	Head        string `json:"headRefOid"`
	headErr     string
}

func (p ratchetPR) report() string {
	if p.Number == 0 {
		return "ratchet tighten: opened a pull request from " + p.HeadRefName
	}
	return fmt.Sprintf("ratchet tighten: updated pull request #%d (%s)", p.Number, p.HeadRefName)
}

type ratchetCandidate struct {
	pr        ratchetPR
	ambiguous bool
	err       error
}

func findRatchetCandidate(wt string) ratchetCandidate {
	out, err := ratchetCmd(wt, "gh", "pr", "list", "--state", "open", "--base", ratchetPRBase, "--limit", "500", "--json", "number,headRefName,isCrossRepository,isDraft,headRefOid")
	if err != nil {
		return ratchetCandidate{err: err}
	}
	var prs []ratchetPR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return ratchetCandidate{err: fmt.Errorf("gh pr list: %w", err)}
	}
	var found []ratchetPR
	for _, p := range prs {
		if !p.Fork && strings.HasPrefix(p.HeadRefName, ratchetBranchPrefix) {
			found = append(found, p)
		}
	}
	if len(found) == 0 {
		return ratchetCandidate{pr: ratchetPR{HeadRefName: ratchetBranchPrefix + time.Now().UTC().Format("2006-01-02")}}
	}
	return ratchetCandidate{pr: fetchRatchetHead(wt, found[0]), ambiguous: len(found) > 1}
}

func fetchRatchetHead(wt string, pr ratchetPR) ratchetPR {
	if pr.Head == "" {
		pr.headErr = "head_unknown"
		return pr
	}
	if _, err := ratchetCmd(wt, "git", "fetch", "--quiet", "origin", "refs/heads/"+pr.HeadRefName); err != nil {
		pr.headErr = "head_unknown"
		return pr
	}
	if tip, err := ratchetCmd(wt, "git", "rev-parse", "FETCH_HEAD"); err != nil || tip != pr.Head {
		pr.headErr = "head_moved"
	}
	return pr
}

func (c ratchetCandidate) unchangedBy(wt string, all []tightenedList) bool {
	return c.pr.Number != 0 && c.pr.headErr == "" && !c.ambiguous && ratchetHeadMatches(wt, c.pr.Head, all) == ""
}

func commitAndPushTightenBranch(wt, branch string, paths []string) error {
	ref := "refs/heads/" + branch
	tip, err := ratchetCmd(wt, "git", "ls-remote", "origin", ref)
	if err == nil {
		lease, _, _ := strings.Cut(tip, "\t")
		err = runGitSteps(wt,
			append([]string{"commit", "--quiet", "-m", ratchetTightenMessage, "--"}, paths...),
			[]string{"push", "--quiet", "--force-with-lease=" + ref + ":" + lease, "origin", "HEAD:" + ref})
	}
	return err
}

func ratchetCmd(dir, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ratchetGitTimeout)
	defer cancel()
	return sysexec.Output(ctx, ratchetRun, dir, bin, args...)
}

func runGitSteps(dir string, steps ...[]string) error {
	for _, args := range steps {
		if _, err := ratchetCmd(dir, "git", args...); err != nil {
			return err
		}
	}
	return nil
}
