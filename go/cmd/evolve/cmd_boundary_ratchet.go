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

	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	boundaryRatchetVerb   = "ratchet-tighten"
	boundaryRatchetDesc   = "propose the tightened size-ratchet allowances as a pull request"
	ratchetTightenMessage = "chore: tighten the size-ratchet allowances"
	ratchetTightenBody    = "The boundary step ratchet-tighten wrote this list from origin/main. The step merges it when required CI is green and both ratchet scans are clean on main.\n\n"
	ratchetBranchPrefix   = "chore/ratchet-tighten-"
	ratchetPRBase         = "main"
	ratchetGitTimeout     = 2 * time.Minute
)

var (
	ratchetRun   sysexec.RunFunc = sysexec.DefaultRunner
	ratchetWrite                 = os.WriteFile
)

func runBoundaryRatchetTighten(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var projectRoot string
	if _, err := (cliFlags{values: map[string]*string{"--project-root": &projectRoot}}).parse(args); err != nil || projectRoot == "" {
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
	return ratchetStep{root: root, wt: wt, stdout: stdout, stderr: stderr}.tightenAndPropose()
}

type boundaryRatchet struct {
	rel     string
	tighten func(string) ([]string, []byte, error)
	scan    func(string) error
}

var boundaryRatchets = []boundaryRatchet{
	{sizeratchet.OffendersRelPath, sizeratchet.TightenModule, sizeratchet.Scan},
	{rawgitratchet.BaselineRelPath, rawgitratchet.TightenModule, scanRawGit},
}

func scanRawGit(module string) error {
	_, err := rawgitratchet.Scan(module)
	return err
}

func (r boundaryRatchet) repoPath() string { return "go/" + r.rel }

func ratchetListPaths() []string {
	paths := make([]string, 0, len(boundaryRatchets))
	for _, r := range boundaryRatchets {
		paths = append(paths, r.repoPath())
	}
	return paths
}

func scanRatchets(module string) error {
	var errs []error
	for _, r := range boundaryRatchets {
		errs = append(errs, r.scan(module))
	}
	return errors.Join(errs...)
}

type tightenedList struct {
	boundaryRatchet
	changes []string
	encoded []byte
}

func tightenLists(module string) ([]tightenedList, error) {
	var lists []tightenedList
	for _, r := range boundaryRatchets {
		changes, encoded, err := r.tighten(module)
		if err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			lists = append(lists, tightenedList{boundaryRatchet: r, changes: changes, encoded: encoded})
		}
	}
	return lists, nil
}

type tightened struct {
	paths   []string
	changes []string
}

func writeTightenedLists(module string, lists []tightenedList) (tightened, error) {
	var t tightened
	for _, l := range lists {
		if err := ratchetWrite(filepath.Join(module, filepath.FromSlash(l.rel)), l.encoded, 0o644); err != nil {
			return tightened{}, err
		}
		t.paths = append(t.paths, l.repoPath())
		t.changes = append(t.changes, l.changes...)
	}
	return t, scanRatchets(module)
}

func tightenWorktree(wt string) (tightened, error) {
	module := filepath.Join(wt, "go")
	lists, err := tightenLists(module)
	if err != nil {
		return tightened{}, err
	}
	return writeTightenedLists(module, lists)
}

type ratchetStep struct {
	root, wt       string
	stdout, stderr io.Writer
}

func (s ratchetStep) tightenAndPropose() error {
	pr, err := openTightenPR(s.wt)
	if err != nil {
		return err
	}
	lists, err := tightenWorktree(s.wt)
	if err == nil && pr.Number != 0 && s.landTightenPR(pr) {
		err = runGitSteps(s.wt, []string{"fetch", "--quiet", "origin", "main"}, []string{"checkout", "--quiet", "--force", "--detach", "origin/main"})
		if err == nil {
			pr, err = openTightenPR(s.wt)
		}
		if err == nil {
			lists, err = tightenWorktree(s.wt)
		}
	}
	if err != nil {
		return err
	}
	return s.proposeTightened(pr, lists)
}

func (s ratchetStep) proposeTightened(pr ratchetPR, lists tightened) error {
	if len(lists.paths) == 0 {
		fmt.Fprintln(s.stdout, "ratchet tighten: tight")
		return nil
	}
	pushed, err := commitAndPushTightenBranch(s.wt, pr.HeadRefName, lists.paths)
	if err == nil && pr.Number == 0 {
		_, err = ratchetCmd(s.wt, "gh", "pr", "create", "--base", ratchetPRBase, "--head", pr.HeadRefName,
			"--title", ratchetTightenMessage, "--body", ratchetTightenBody+strings.Join(lists.changes, "\n"))
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(s.stdout, strings.Join(lists.changes, "\n"))
	if !pushed && pr.Number != 0 {
		fmt.Fprintf(s.stdout, "ratchet tighten: #%d already holds this list (%s)\n", pr.Number, pr.HeadRefName)
		return nil
	}
	fmt.Fprintln(s.stdout, pr.report())
	return nil
}

type ratchetPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
	Fork        bool   `json:"isCrossRepository"`
}

func (p ratchetPR) report() string {
	if p.Number == 0 {
		return "ratchet tighten: opened a pull request from " + p.HeadRefName
	}
	return fmt.Sprintf("ratchet tighten: updated pull request #%d (%s)", p.Number, p.HeadRefName)
}

func openTightenPR(wt string) (ratchetPR, error) {
	out, err := ratchetCmd(wt, "gh", "pr", "list", "--state", "open", "--base", ratchetPRBase, "--limit", "500", "--json", "number,headRefName,headRefOid,isCrossRepository")
	if err != nil {
		return ratchetPR{}, err
	}
	var prs []ratchetPR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return ratchetPR{}, fmt.Errorf("gh pr list: %w", err)
	}
	for _, p := range prs {
		if !p.Fork && strings.HasPrefix(p.HeadRefName, ratchetBranchPrefix) {
			return p, nil
		}
	}
	return ratchetPR{HeadRefName: ratchetBranchPrefix + time.Now().UTC().Format("2006-01-02")}, nil
}

func commitAndPushTightenBranch(wt, branch string, paths []string) (bool, error) {
	ref := "refs/heads/" + branch
	tip, err := ratchetCmd(wt, "git", "ls-remote", "origin", ref)
	if err != nil {
		return false, err
	}
	lease, _, _ := strings.Cut(tip, "\t")
	if lease != "" {
		if _, err := fetchBranchTip(wt, ref); err != nil {
			return false, err
		}
	}
	if same, err := leaseHoldsTheStepLists(wt, lease); same || err != nil {
		return false, err
	}
	return true, runGitSteps(wt,
		append([]string{"commit", "--quiet", "-m", ratchetTightenMessage, "--"}, paths...),
		[]string{"push", "--quiet", "--force-with-lease=" + ref + ":" + lease, "origin", "HEAD:" + ref})
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
