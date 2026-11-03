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
	return tightenAndPropose(root, wt, stdout, stderr)
}

type tightenedList struct {
	rel     string
	changes []string
	encoded []byte
}

func tightenLists(module string) ([]tightenedList, error) {
	var lists []tightenedList
	for _, t := range []struct {
		rel     string
		tighten func(string) ([]string, []byte, error)
	}{{sizeratchet.OffendersRelPath, sizeratchet.TightenModule}, {rawgitratchet.BaselineRelPath, rawgitratchet.TightenModule}} {
		changes, encoded, err := t.tighten(module)
		if err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			lists = append(lists, tightenedList{rel: t.rel, changes: changes, encoded: encoded})
		}
	}
	return lists, nil
}

func writeTightenedLists(module string, lists []tightenedList) (paths, changes []string, err error) {
	for _, l := range lists {
		if werr := ratchetWrite(filepath.Join(module, filepath.FromSlash(l.rel)), l.encoded, 0o644); werr != nil {
			return nil, nil, werr
		}
		paths = append(paths, "go/"+l.rel)
		changes = append(changes, l.changes...)
	}
	if serr := sizeratchet.Scan(module); serr != nil {
		return nil, nil, serr
	}
	_, err = rawgitratchet.Scan(module)
	return paths, changes, err
}

func tightenAndPropose(root, wt string, stdout, stderr io.Writer) error {
	pr, err := openTightenPR(wt)
	if err == nil && pr.Number != 0 && landTightenPR(root, wt, pr, stdout, stderr) {
		err = runGitSteps(wt, []string{"fetch", "--quiet", "origin", "main"}, []string{"checkout", "--quiet", "--detach", "origin/main"})
		if err == nil {
			pr, err = openTightenPR(wt)
		}
	}
	if err != nil {
		return err
	}
	return proposeTightened(wt, pr, stdout)
}

func proposeTightened(wt string, pr ratchetPR, stdout io.Writer) error {
	module := filepath.Join(wt, "go")
	lists, err := tightenLists(module)
	if err != nil {
		return err
	}
	if len(lists) == 0 {
		fmt.Fprintln(stdout, "ratchet tighten: tight")
		return nil
	}
	paths, changes, err := writeTightenedLists(module, lists)
	pushed := false
	if err == nil {
		pushed, err = commitAndPushTightenBranch(wt, pr.HeadRefName, paths)
	}
	if err == nil && pr.Number == 0 {
		_, err = ratchetCmd(wt, "gh", "pr", "create", "--base", ratchetPRBase, "--head", pr.HeadRefName,
			"--title", ratchetTightenMessage, "--body", ratchetTightenBody+strings.Join(changes, "\n"))
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, strings.Join(changes, "\n"))
	fmt.Fprintln(stdout, pr.report(pushed))
	return nil
}

type ratchetPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
	Fork        bool   `json:"isCrossRepository"`
}

func (p ratchetPR) report(pushed bool) string {
	switch {
	case p.Number == 0:
		return "ratchet tighten: opened a pull request from " + p.HeadRefName
	case !pushed:
		return fmt.Sprintf("ratchet tighten: #%d already holds this list (%s)", p.Number, p.HeadRefName)
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
	if same, err := tipHoldsLists(wt, ref, lease); same || err != nil {
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
