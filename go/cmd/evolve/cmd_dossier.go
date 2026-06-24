package main

// cmd_dossier.go — ADR-0055 D3: `evolve dossier verify` reads every
// knowledge-base/cycles/cycle-N.json, parses it, and calls dossier.Validate().
// Pure reader — no state/ledger mutation — safe to run mid-batch.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func runDossier(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve dossier: usage: dossier verify [--project-root P]")
		return 10
	}
	switch args[0] {
	case "verify":
		return runDossierVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve dossier: unknown subcommand %q (want: verify)\n", args[0])
		return 10
	}
}

func runDossierVerify(_ []string, stdout, stderr io.Writer) int {
	root := "."
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		root = r
	}
	// ADR-0055 D3: when the policy floor enrolls "dossier-closeout", an absent or
	// empty knowledge-base/cycles/ is a FAILURE — the gate exists precisely to
	// catch the no-dossier-written case. Without enrollment, absence stays a
	// no-op success (safe to run mid-batch). A malformed policy fails loudly.
	pol, perr := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if perr != nil {
		fmt.Fprintf(stderr, "dossier verify: load policy: %v\n", perr)
		return 1
	}
	enforced := pol.FloorEnrolls("dossier-closeout")

	dir := filepath.Join(root, "knowledge-base", "cycles")
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
