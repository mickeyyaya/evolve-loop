package phasecmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecoherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const (
	provenanceExitClean      = 0
	provenanceExitViolation  = 1
	provenanceExitUnreadable = 2
	provenanceExitUsage      = 10
)

type provenanceFinding struct {
	Artifact string `json:"artifact"`
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	Message  string `json:"message"`
}

type provenanceReport struct {
	Cycle            int                 `json:"cycle"`
	ArtifactsChecked int                 `json:"artifacts_checked"`
	Violations       []provenanceFinding `json:"violations"`
}

func phasesCheckProvenance(c phasesCall) int {
	fs := flag.NewFlagSet("evolve phases check-provenance", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	cycle := fs.Int("cycle", 0, "cycle whose phase artifacts to check (required, > 0)")
	asJSON := fs.Bool("json", false, "print one JSON document on stdout")
	if err := fs.Parse(c.args); err != nil {
		return provenanceExitUsage
	}
	if *cycle <= 0 || fs.NArg() > 0 {
		fmt.Fprintln(c.stderr, "usage: evolve phases check-provenance --cycle N [--json] (N a positive cycle number)")
		return provenanceExitUsage
	}
	report, err := collectProvenance(c.project, *cycle, c.stderr)
	if err != nil {
		fmt.Fprintf(c.stderr, "check-provenance: %v\n", err)
		return provenanceExitUnreadable
	}
	if err := writeProvenanceReport(c.stdout, report, *asJSON); err != nil {
		fmt.Fprintf(c.stderr, "check-provenance: write: %v\n", err)
		return provenanceExitUnreadable
	}
	for _, v := range report.Violations {
		if v.Severity == phasecoherence.SeverityError {
			return provenanceExitViolation
		}
	}
	return provenanceExitClean
}

func collectProvenance(project string, cycle int, stderr io.Writer) (provenanceReport, error) {
	report := provenanceReport{Cycle: cycle, Violations: []provenanceFinding{}}
	cat, _, warns, err := phasespec.MergedCatalog(project)
	if err != nil {
		return report, fmt.Errorf("load phase catalog: %w", err)
	}
	for _, w := range warns {
		fmt.Fprintln(stderr, "WARN:", w)
	}
	workspace := filepath.Join(project, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	for _, s := range cat.All() {
		if len(s.Outputs.Files) == 0 || !strings.HasSuffix(s.Outputs.Files[0], ".md") {
			continue
		}
		artifact := filepath.Join(workspace, filepath.Base(s.Outputs.Files[0]))
		raw, err := os.ReadFile(artifact)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return report, fmt.Errorf("read artifact %s: %w", artifact, err)
		}
		violations, err := phasecoherence.CheckProvenance(string(raw), phasecoherence.ProvenanceFields{Phase: s.Name, Cycle: cycle})
		if err != nil {
			return report, err
		}
		report.ArtifactsChecked++
		for _, v := range violations {
			report.Violations = append(report.Violations, provenanceFinding{
				Artifact: relTo(project, artifact), Severity: v.Severity, Kind: v.Kind, Message: v.Message,
			})
		}
	}
	return report, nil
}

func writeProvenanceReport(w io.Writer, report provenanceReport, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	for _, v := range report.Violations {
		if _, err := fmt.Fprintf(w, "%s: %s: %s\n", v.Severity, v.Artifact, v.Message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "cycle %d: %d artifact(s) checked, %d provenance finding(s)\n",
		report.Cycle, report.ArtifactsChecked, len(report.Violations))
	return err
}
