package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const landingUsage = "usage: evolve landing list [--project-root P] | evolve landing approve --cycle N --tree SHA --audit-report PATH --operator [--project-root P] | evolve landing reject --cycle N --tree SHA --operator [--project-root P]"

var landingCredential = dispatchcred.Default

var (
	errLandingNotHeld      = errors.New("no held-for-operator record for this cycle")
	errLandingTreeDiffers  = errors.New("the tree differs from the held tree")
	errLandingNoCredential = errors.New("this process does not hold the dispatch credential: a phase cell never decides a held proposal")
	errLandingNoOperator   = errors.New("an approval or a rejection needs --operator: only the operator decides a held proposal")
	errLandingNoAudit      = errors.New("an approval needs --audit-report: the adversarial audit of the held proposal")
	errLandingAuditDir     = errors.New("the audit report is a directory, not a report file")
)

func runLanding(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve landing: "+landingUsage)
		return 10
	}
	switch args[0] {
	case "list":
		return runLandingList(args[1:], stdout, stderr)
	case "approve":
		return runLandingDecide(heldproposal.DecisionApprove, args[1:], stdout, stderr)
	case "reject":
		return runLandingDecide(heldproposal.DecisionReject, args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve landing: unknown subcommand %q\n%s\n", args[0], landingUsage)
	return 10
}

func landingEvolveDir(root string) string {
	if root == "" {
		root = envOrCwd("EVOLVE_PROJECT_ROOT")
	}
	return paths.EvolveDirOf(root)
}

func runLandingList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("landing list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "project root")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	records, err := heldproposal.ReadRecords(landingEvolveDir(*root))
	if err != nil {
		fmt.Fprintf(stderr, "evolve landing list: %v\n", err)
		return 2
	}
	if len(records) == 0 {
		fmt.Fprintln(stdout, "landing: no held proposals")
	}
	for _, r := range records {
		fmt.Fprintf(stdout, "cycle %d  %s  tree %s  %s\n", r.Cycle, r.Status, r.Tree, strings.Join(r.Reasons, ", "))
	}
	return 0
}

type landingDecision struct {
	cycle    int
	tree     string
	audit    string
	operator bool
	root     string
}

func parseLandingDecision(verb string, args []string, stderr io.Writer) (landingDecision, bool) {
	var d landingDecision
	fs := flag.NewFlagSet("landing "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.IntVar(&d.cycle, "cycle", 0, "the held cycle")
	fs.StringVar(&d.tree, "tree", "", "the tree digest that the hold record names")
	fs.StringVar(&d.audit, "audit-report", "", "the adversarial audit report (approve only)")
	fs.BoolVar(&d.operator, "operator", false, "the operator makes this decision")
	fs.StringVar(&d.root, "project-root", "", "project root")
	if err := fs.Parse(args); err != nil || d.cycle < 1 || d.tree == "" {
		fmt.Fprintln(stderr, "evolve landing: "+landingUsage)
		return d, false
	}
	return d, true
}

func runLandingDecide(verb string, args []string, stdout, stderr io.Writer) int {
	d, ok := parseLandingDecision(verb, args, stderr)
	if !ok {
		return 10
	}
	evolveDir := landingEvolveDir(d.root)
	rec, err := checkLandingDecision(evolveDir, verb, d)
	if err != nil {
		fmt.Fprintf(stderr, "evolve landing %s: refused: %v\n", verb, err)
		return 1
	}
	a := heldproposal.Approval{Cycle: d.cycle, Tree: d.tree, Decision: verb, Audit: d.audit, At: time.Now().UTC()}
	rec.Status = heldproposal.StatusAfter(verb)
	if err := errors.Join(heldproposal.WriteApproval(evolveDir, a), heldproposal.WriteRecord(evolveDir, rec)); err != nil {
		fmt.Fprintf(stderr, "evolve landing %s: %v\n", verb, err)
		return 2
	}
	fmt.Fprintf(stdout, "landing: cycle %d tree %s: %s recorded; run the ship of the cycle again (evolve loop resume) to land or close it\n", d.cycle, d.tree, verb)
	return 0
}

func checkLandingDecision(evolveDir, verb string, d landingDecision) (heldproposal.Record, error) {
	if v := dispatchcred.Admit(landingCredential); !v.Held {
		return heldproposal.Record{}, fmt.Errorf("%w (%v)", errLandingNoCredential, v.Cause)
	}
	if !d.operator {
		return heldproposal.Record{}, errLandingNoOperator
	}
	if verb == heldproposal.DecisionApprove {
		if d.audit == "" {
			return heldproposal.Record{}, errLandingNoAudit
		}
		info, err := os.Stat(d.audit)
		if err != nil {
			return heldproposal.Record{}, fmt.Errorf("the audit report: %w", err)
		}
		if info.IsDir() {
			return heldproposal.Record{}, errLandingAuditDir
		}
	}
	rec, found, err := heldproposal.ReadRecord(evolveDir, d.cycle)
	if err != nil {
		return rec, err
	}
	if !found || rec.Status != heldproposal.StatusHeld {
		return rec, errLandingNotHeld
	}
	if rec.Tree != d.tree {
		return rec, fmt.Errorf("%w: held %s, given %s", errLandingTreeDiffers, rec.Tree, d.tree)
	}
	return rec, nil
}
