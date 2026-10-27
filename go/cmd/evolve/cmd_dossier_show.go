package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const dossierShowUsage = "usage: evolve dossier show <cycle> [--json] [--project-root P]"

type dossierShowResult struct {
	Cycle   int              `json:"cycle"`
	Path    string           `json:"path"`
	Dossier *dossier.Dossier `json:"dossier"`
}

func runDossierShow(args []string, stdout, stderr io.Writer) int {
	var projectRoot string
	var asJSON bool
	operands, err := (cliFlags{values: map[string]*string{"--project-root": &projectRoot}, bools: map[string]*bool{"--json": &asJSON}}).parse(args)
	if err != nil || len(operands) != 1 {
		fmt.Fprintln(stderr, dossierShowUsage)
		return cliExitUsage
	}
	cycle, err := strconv.Atoi(operands[0])
	if err != nil || cycle < 1 {
		fmt.Fprintf(stderr, "dossier show: the cycle %q is not a positive number\n%s\n", operands[0], dossierShowUsage)
		return cliExitUsage
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "dossier show: cwd: %v\n", err)
		return cliExitIO
	}
	result, rc := readCycleDossier(root, cycle, stderr)
	if rc != cliExitOK {
		return rc
	}
	if asJSON {
		return printVerbJSON("dossier show", result, stdout, stderr)
	}
	fmt.Fprintf(stdout, "cycle %d: %s: %s\n%s\n", cycle, result.Dossier.FinalVerdict, result.Dossier.Goal, result.Path)
	return cliExitOK
}

func readCycleDossier(root string, cycle int, stderr io.Writer) (dossierShowResult, int) {
	result := dossierShowResult{Cycle: cycle}
	path, ok := dossier.CloseoutPath(root, cycle)
	if !ok {
		fmt.Fprintf(stderr, "dossier show: cycle %d has no dossier in %s or %s\n", cycle, dossier.CyclesDir(root), dossier.PendingDir(root))
		return result, cliExitFailed
	}
	result.Path = path
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "dossier show: %v\n", err)
		return result, cliExitIO
	}
	if result.Dossier, err = dossier.ParseJSON(raw); err != nil {
		fmt.Fprintf(stderr, "dossier show: %s: %v\n", path, err)
		return result, cliExitInput
	}
	return result, cliExitOK
}
