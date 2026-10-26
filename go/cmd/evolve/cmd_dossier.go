package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func runDossier(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve dossier: usage: dossier verify | retro-mislabel [--project-root P] [--json] | publish [--dry-run] [--project-root P] | sweep [--dry-run] [--project-root P]")
		return 10
	}
	switch args[0] {
	case "verify":
		return runDossierVerify(args[1:], stdout, stderr)
	case "retro-mislabel":
		return runDossierRetroMislabel(args[1:], stdout, stderr)
	case "publish":
		return runDossierPublish(args[1:], stdout, stderr)
	case "sweep":
		return runDossierSweep(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve dossier: unknown subcommand %q (want: verify | retro-mislabel | publish | sweep)\n", args[0])
		return 10
	}
}

func runDossierPublish(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve dossier publish", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", ".", "the plane whose .evolve/dossiers-pending is published")
	dryRun := fs.Bool("dry-run", false, "list what would publish and what would hold it; change nothing")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "dossier publish: unexpected arguments %q\n", fs.Args())
		return 10
	}
	projectRoot := paths.AbsoluteRoot("the project root", *root, nil)
	pairs, halfPairs, err := dossier.ListPending(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "dossier publish: %v\n", err)
		return 2
	}
	if len(halfPairs) > 0 {
		fmt.Fprintf(stdout, "dossier publish: half pairs %v stay pending (a .md without its .json)\n", halfPairs)
	}
	if len(pairs) == 0 {
		fmt.Fprintln(stdout, "dossier publish: nothing pending to publish")
		return 0
	}
	lock := tryDossierPublishLock
	if *dryRun {
		lock = probeDossierPublishLock
	}
	release, hold, err := holdDossierPublish(projectRoot, lock)
	if err != nil {
		fmt.Fprintf(stderr, "dossier publish: %v\n", err)
		return 2
	}
	if hold != "" {
		fmt.Fprintf(stderr, "dossier publish: held: %s; pending cycles %v stay pending\n", hold, pairs)
		return 1
	}
	defer release()
	if *dryRun {
		fmt.Fprintf(stdout, "dossier publish: dry run: would publish cycles %v\n", pairs)
		return 0
	}
	return publishDossierPairs(projectRoot, stdout, stderr)
}

func runDossierSweep(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve dossier sweep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", ".", "the plane whose uncommitted knowledge-base/cycles pairs are recommitted")
	dryRun := fs.Bool("dry-run", false, "list what would be recommitted; change nothing")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "dossier sweep: unexpected arguments %q\n", fs.Args())
		return 10
	}
	projectRoot := paths.AbsoluteRoot("the project root", *root, nil)
	g := gitexec.Default(projectRoot)
	plan, err := dossier.PlanOrphans(g)
	if err != nil {
		fmt.Fprintf(stderr, "dossier sweep: %v\n", err)
		return 2
	}
	if len(plan.Half) > 0 {
		fmt.Fprintf(stdout, "dossier sweep: half pairs %v stay uncommitted (a .md without its .json, or the reverse)\n", plan.Half)
	}
	if len(plan.Complete) == 0 {
		fmt.Fprintln(stdout, "dossier sweep: nothing to recommit")
		return 0
	}
	lock := tryDossierPublishLock
	if *dryRun {
		lock = probeDossierPublishLock
	}
	release, hold, err := holdDossierPublish(projectRoot, lock)
	if err != nil {
		fmt.Fprintf(stderr, "dossier sweep: %v\n", err)
		return 2
	}
	if hold != "" {
		fmt.Fprintf(stderr, "dossier sweep: held: %s; orphan cycles %v stay uncommitted\n", hold, plan.Complete)
		return 1
	}
	defer release()
	if *dryRun {
		fmt.Fprintf(stdout, "dossier sweep: dry run: would recommit cycles %v\n", plan.Complete)
		return 0
	}
	return sweepDossierPairs(g, stdout, stderr)
}

func sweepDossierPairs(g gitexec.Git, stdout, stderr io.Writer) int {
	res, err := dossier.SweepOrphans(g, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "dossier sweep: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "dossier sweep: recommitted cycles %v\n", res.Recommitted)
	if len(res.Failed) > 0 {
		fmt.Fprintf(stderr, "dossier sweep: %d pairs failed to commit and stay uncommitted\n", len(res.Failed))
		return 2
	}
	return 0
}

func publishDossierPairs(projectRoot string, stdout, stderr io.Writer) int {
	res, err := dossier.PublishPending(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "dossier publish: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "dossier publish: published cycles %v\n", res.Published)
	if len(res.Failed) > 0 {
		fmt.Fprintf(stderr, "dossier publish: %d pending pairs were refused or failed and stay pending\n", len(res.Failed))
		return 2
	}
	return 0
}

type dossierRetroMislabelReport struct {
	Candidates     int   `json:"candidates"`
	Mislabeled     []int `json:"mislabeled"`
	Uncorroborated []int `json:"uncorroborated"`
}

func runDossierRetroMislabel(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve dossier retro-mislabel", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", ".", "project root containing knowledge-base/cycles")
	asJSON := fs.Bool("json", false, "emit the derived classification as JSON")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if _, err := os.ReadDir(dossier.CyclesDir(*root)); err != nil {
		fmt.Fprintf(stderr, "dossier retro-mislabel: read knowledge-base/cycles: %v\n", err)
		return 1
	}

	report := dossierRetroMislabelReport{
		Mislabeled:     []int{},
		Uncorroborated: []int{},
	}
	for _, d := range dossier.ReadCommitted(*root, 0) {
		if d.SchemaVersion != 0 {
			continue
		}
		switch dossier.PhaseSkipEvidence(*root, d, "retro") {
		case dossier.SkipEvidenceContradicted:
			report.Candidates++
			report.Mislabeled = append(report.Mislabeled, d.Cycle)
		case dossier.SkipEvidenceUnverified:
			report.Candidates++
			report.Uncorroborated = append(report.Uncorroborated, d.Cycle)
		}
	}

	if *asJSON {
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "dossier retro-mislabel: encode report: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "dossier retro-mislabel: %d candidates; %d mislabeled; %d uncorroborated\n",
		report.Candidates, len(report.Mislabeled), len(report.Uncorroborated))
	return 0
}

type dossierVerifyResult struct {
	Dir      string   `json:"dir"`
	Enforced bool     `json:"enforced"`
	Verified []string `json:"verified"`
	Invalid  []string `json:"invalid"`
}

func runDossierVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve dossier verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: evolve dossier verify [--json] [--project-root P]")
		return cliExitUsage
	}
	root, err := loopStopRoot(*projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "dossier verify: cwd: %v\n", err)
		return cliExitIO
	}
	human := humanWriter(stdout, *asJSON)
	result, rc := verifyDossiers(root, human, stderr)
	if *asJSON && rc == cliExitOK {
		return printVerbJSON("dossier verify", result, stdout, stderr)
	}
	return rc
}

func verifyDossiers(root string, stdout, stderr io.Writer) (dossierVerifyResult, int) {
	result := dossierVerifyResult{Dir: dossier.CyclesDir(root), Verified: []string{}, Invalid: []string{}}
	pol, perr := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if perr != nil {
		fmt.Fprintf(stderr, "dossier verify: load policy: %v\n", perr)
		return result, cliExitFailed
	}
	result.Enforced = pol.FloorEnrolls("dossier-closeout")
	entries, err := os.ReadDir(result.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			if result.Enforced {
				fmt.Fprintln(stderr, `dossier verify: FAIL — policy floor enrolls "dossier-closeout" but knowledge-base/cycles/ is absent (no dossiers written)`)
				return result, cliExitFailed
			}
			fmt.Fprintln(stdout, "dossier verify: knowledge-base/cycles/ absent — no dossiers to verify (OK)")
			return result, cliExitOK
		}
		fmt.Fprintf(stderr, "dossier verify: read dir: %v\n", err)
		return result, cliExitFailed
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if err := verifyDossierFile(filepath.Join(result.Dir, e.Name())); err != nil {
			fmt.Fprintf(stderr, "dossier verify: %v\n", err)
			result.Invalid = append(result.Invalid, e.Name())
			continue
		}
		result.Verified = append(result.Verified, e.Name())
		fmt.Fprintf(stdout, "dossier verify: OK %s\n", e.Name())
	}
	if result.Enforced && len(result.Verified)+len(result.Invalid) == 0 {
		fmt.Fprintln(stderr, `dossier verify: FAIL — policy floor enrolls "dossier-closeout" but knowledge-base/cycles/ contains no dossiers`)
		return result, cliExitFailed
	}
	if len(result.Invalid) > 0 {
		return result, cliExitFailed
	}
	return result, cliExitOK
}

func verifyDossierFile(path string) error {
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	d, err := dossier.ParseJSON(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	if err := d.Validate(); err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	return nil
}
