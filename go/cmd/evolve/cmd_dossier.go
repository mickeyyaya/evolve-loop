package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func runDossier(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve dossier: usage: dossier verify | retro-mislabel [--project-root P] [--json] | publish [--dry-run] [--project-root P]")
		return 10
	}
	switch args[0] {
	case "verify":
		return runDossierVerify(args[1:], stdout, stderr)
	case "retro-mislabel":
		return runDossierRetroMislabel(args[1:], stdout, stderr)
	case "publish":
		return runDossierPublish(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve dossier: unknown subcommand %q (want: verify | retro-mislabel | publish)\n", args[0])
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

func runDossierVerify(_ []string, stdout, stderr io.Writer) int {
	root := "."
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		root = r
	}
	// Enrolling "dossier-closeout" turns an absent or empty knowledge-base/cycles/
	// into a FAIL; unenrolled, absence is a no-op success.
	// See ADR-0055.
	pol, perr := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if perr != nil {
		fmt.Fprintf(stderr, "dossier verify: load policy: %v\n", perr)
		return 1
	}
	enforced := pol.FloorEnrolls("dossier-closeout")

	dir := dossier.CyclesDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if enforced {
				fmt.Fprintln(stderr, `dossier verify: FAIL — policy floor enrolls "dossier-closeout" but knowledge-base/cycles/ is absent (no dossiers written)`)
				return 1
			}
			fmt.Fprintln(stdout, "dossier verify: knowledge-base/cycles/ absent — no dossiers to verify (OK)")
			return 0
		}
		fmt.Fprintf(stderr, "dossier verify: read dir: %v\n", err)
		return 1
	}
	errs := 0
	found := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		found++
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "dossier verify: read %s: %v\n", e.Name(), err)
			errs++
			continue
		}
		d, err := dossier.ParseJSON(data)
		if err != nil {
			fmt.Fprintf(stderr, "dossier verify: parse %s: %v\n", e.Name(), err)
			errs++
			continue
		}
		if err := d.Validate(); err != nil {
			fmt.Fprintf(stderr, "dossier verify: invalid %s: %v\n", e.Name(), err)
			errs++
			continue
		}
		fmt.Fprintf(stdout, "dossier verify: OK %s\n", e.Name())
	}
	if enforced && found == 0 {
		fmt.Fprintln(stderr, `dossier verify: FAIL — policy floor enrolls "dossier-closeout" but knowledge-base/cycles/ contains no dossiers`)
		return 1
	}
	if errs > 0 {
		return 1
	}
	return 0
}
