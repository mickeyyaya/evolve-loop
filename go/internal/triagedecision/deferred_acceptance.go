package triagedecision

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	deferredAcceptanceSection = "deferred_acceptance"
	reportFile                = "triage-report.md"
	minReasonWords            = 3
)

var acceptanceIndexRE = regexp.MustCompile(`^acceptance\s+([0-9]+)$`)

type AcceptanceDeferral struct {
	ID        string
	Index     int
	Reason    string
	Criterion string
}

func DeferredAcceptance(report string) []AcceptanceDeferral {
	body, ok := SectionBody(report, deferredAcceptanceSection)
	if !ok {
		return nil
	}
	committed := map[string]bool{}
	if top, ok := SectionBody(report, "top_n"); ok {
		for _, it := range ParseSection(top).Items {
			committed[it.ID] = true
		}
	}
	var out []AcceptanceDeferral
	for _, it := range ParseSection(body).Items {
		if !committed[it.ID] {
			continue
		}
		m := acceptanceIndexRE.FindStringSubmatch(ActionOf(it.Rest))
		reason := ReasonOf(it.Rest)
		if m == nil || len(strings.Fields(reason)) < minReasonWords {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			out = append(out, AcceptanceDeferral{ID: it.ID, Index: n, Reason: reason})
		}
	}
	return out
}

func DeferredAcceptanceIn(workspace string) []AcceptanceDeferral {
	report, err := os.ReadFile(filepath.Join(workspace, reportFile))
	if err != nil {
		return nil
	}
	return DeferredAcceptance(string(report))
}
