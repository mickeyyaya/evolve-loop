package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
)

type steLintRow struct {
	Path string `json:"path"`
	stelint.Finding
}

type steLintReport struct {
	FilesChecked      int            `json:"files_checked"`
	FilesWithFindings int            `json:"files_with_findings"`
	FindingCount      int            `json:"finding_count"`
	ByRule            map[string]int `json:"by_rule"`
	Findings          []steLintRow   `json:"findings"`
	Skipped           []steLintRow   `json:"skipped"`
	rows              []steLintRow
}

func lintSteTargets(targets []steTarget, opt stelint.Options, stderr io.Writer) (steLintReport, bool) {
	r := steLintReport{ByRule: map[string]int{}, Findings: []steLintRow{}, Skipped: []steLintRow{}}
	failed := false
	for _, t := range targets {
		findings, err := stelint.LintFile(t.abs, opt)
		if err != nil {
			fmt.Fprintf(stderr, "%s%s: %v\n", steLintPrefix, t.display, err)
			failed = true
			continue
		}
		r.add(t.display, findings)
	}
	return r, failed
}

func (r *steLintReport) add(path string, findings []stelint.Finding) {
	r.FilesChecked++
	counted := 0
	for _, f := range findings {
		row := steLintRow{Path: path, Finding: f}
		r.rows = append(r.rows, row)
		if f.IsSkipNotice() {
			r.Skipped = append(r.Skipped, row)
			continue
		}
		r.Findings = append(r.Findings, row)
		r.ByRule[f.Rule]++
		counted++
	}
	r.FindingCount += counted
	if counted > 0 {
		r.FilesWithFindings++
	}
}

func (r steLintReport) writeJSON(w io.Writer) error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", raw)
	return err
}

func (r steLintReport) writeText(w io.Writer) error {
	for _, row := range r.rows {
		fmt.Fprintf(w, "%s:%d %s %s\n", row.Path, row.Line, row.Rule, row.Message)
	}
	_, err := fmt.Fprintf(w, "ste-lint: %d finding(s) in %d file(s); %d file(s) checked\n", r.FindingCount, r.FilesWithFindings, r.FilesChecked)
	return err
}
