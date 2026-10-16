// Package topngate checks the TDD and build deliverables against the task set
// triage committed. See docs/architecture/packages/internal-topngate.md.
package topngate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var (
	triageReportName = phasecontract.ArtifactName("triage")
	buildReportName  = phasecontract.ArtifactName("build")
	tddReportName    = phasecontract.ArtifactName("tdd")
	scoutReportName  = phasecontract.ArtifactName(string(core.PhaseScout))
)

type gate interface {
	name() string
	appliesTo(phase string) bool
	check(in core.ReviewInput) (reason string, block bool)
}

type topNBindingGate struct{}

func (topNBindingGate) name() string { return "topn-task-binding" }

func (topNBindingGate) appliesTo(phase string) bool { return phase == string(core.PhaseBuild) }

func (topNBindingGate) check(in core.ReviewInput) (string, bool) {
	topN, ok := readTopNSlugs(in.Workspace)
	if !ok || len(topN) == 0 {
		return "", false
	}
	claimed, ok := readClaimedSlug(in.Workspace)
	if !ok || claimed == "" {
		return "", false
	}
	for _, s := range topN {
		if s == claimed {
			return "", false
		}
	}
	return "label drift (advisory since 2026-07-22): build-report labels its task '" + claimed + "' but triage committed {" + strings.Join(topN, ", ") + "} — binding to the committed set", false
}

type tddScopeGate struct{}

func (tddScopeGate) name() string { return "topn-tdd-scope" }

func (tddScopeGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

func (tddScopeGate) check(in core.ReviewInput) (string, bool) {
	topN, haveReport := readTopNSlugs(in.Workspace)
	claimed, declared, authored, ok := readTDDScope(in.Workspace)
	if !ok {
		return "", false
	}
	committed := normalizedSlugs(core.ContractTaskIDs(in.Workspace))
	if !haveReport {
		switch len(committed) {
		case 0:
			return "TDD scope is unverifiable: triage-report.md, triage-decision.json and lane-scope.json are all missing or empty", true
		case 1:
			topN = committed
		}
	}
	if len(committed) > 1 {
		if reason, block := reconcileMemberSets(committed, normalizedSlugs(declared)); block {
			return reason, true
		}
		return fileScopeAdvisoryMulti(in.Workspace, committed, authored), false
	}
	if len(authored) == 0 {
		return "", false
	}
	if len(topN) == 0 {
		return "triage committed an EMPTY ## top_n so the TDD phase must author nothing, but test-report.md claims '" +
			claimed + "' and declares authored test file(s) {" + strings.Join(authored, ", ") + "}", true
	}
	if claimed == "" {
		return "", false
	}
	for _, s := range topN {
		if s == claimed {
			return fileScopeAdvisory(in.Workspace, claimed, authored), false
		}
	}
	return "label drift (advisory since 2026-07-23): TDD authored test file(s) {" + strings.Join(authored, ", ") +
		"} labelled '" + claimed + "' but triage committed {" + strings.Join(topN, ", ") +
		"} — binding to the committed set", false
}

func fileScopeAdvisory(workspace, slug string, authored []string) string {
	declared := readScoutTargetFiles(workspace, slug)
	if len(declared) == 0 || len(authored) == 0 || anyPathOverlaps(authored, declared) {
		return ""
	}
	return "file scope drift (advisory): TDD authored test file(s) {" + strings.Join(authored, ", ") +
		"} but the committed item '" + slug + "' declares targetFiles {" + strings.Join(declared, ", ") +
		"} — zero path overlap"
}

func fileScopeAdvisoryMulti(workspace string, members, authored []string) string {
	if len(authored) == 0 {
		return ""
	}
	var union []string
	for _, slug := range members {
		declared := readScoutTargetFiles(workspace, slug)
		if len(declared) == 0 {
			return ""
		}
		union = append(union, declared...)
	}
	if anyPathOverlaps(authored, union) {
		return ""
	}
	return "file scope drift (advisory): TDD authored test file(s) {" + strings.Join(authored, ", ") +
		"} but the committed members {" + strings.Join(members, ", ") + "} declare targetFiles {" +
		strings.Join(union, ", ") + "} — zero path overlap"
}

func anyPathOverlaps(authored, declared []string) bool {
	for _, a := range authored {
		for _, d := range declared {
			if pathsOverlap(a, d) {
				return true
			}
		}
	}
	return false
}

func pathsOverlap(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	return a == b || filepath.Dir(a) == filepath.Dir(b)
}

func readScoutTargetFiles(workspace, slug string) []string {
	body, ok := readWorkspaceFile(workspace, scoutReportName)
	if !ok {
		return nil
	}
	current := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "### "):
			current = taskHeaderSlug(trimmed)
			continue
		case strings.HasPrefix(trimmed, "## "):
			current = ""
			continue
		}
		if current != slug || !strings.HasPrefix(trimmed, "- **targetFiles:**") {
			continue
		}
		if paths := backtickedPaths(trimmed); len(paths) > 0 {
			return paths
		}
	}
	return nil
}

func taskHeaderSlug(trimmed string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
	i := strings.Index(rest, ":")
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(rest[i+1:])
}

func backtickedPaths(line string) []string {
	var paths []string
	parts := strings.Split(line, "`")
	for i := 1; i < len(parts); i += 2 {
		if p := strings.TrimSpace(parts[i]); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func reconcileMemberSets(committed, declared []string) (string, bool) {
	var missing, extra []string
	for _, id := range committed {
		if !slices.Contains(declared, id) {
			missing = append(missing, id)
		}
	}
	for _, id := range declared {
		if !slices.Contains(committed, id) {
			extra = append(extra, id)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		return "scope-mismatch at TDD->Build: missing committed members {" + strings.Join(missing, ", ") + "}; unexpected declared members {" + strings.Join(extra, ", ") + "}", true
	}
	return "", false
}

func normalizedSlugs(slugs []string) []string {
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		if slug = strings.TrimSpace(slug); slug != "" {
			out = append(out, slug)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func readTDDScope(workspace string) (slug string, slugs, testFiles []string, ok bool) {
	body, ok := readWorkspaceFile(workspace, tddReportName)
	if !ok {
		return "", nil, nil, false
	}
	slug, slugs, testFiles = parseTDDReport(body)
	return slug, slugs, testFiles, true
}

func parseTDDReport(body string) (slug string, slugs, testFiles []string) {
	var fence string
	var block []string
	inHandoff, capture, found := false, false, false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			run := len(trimmed) - len(strings.TrimLeft(trimmed, fence[:1]))
			if run >= len(fence) && strings.TrimSpace(trimmed[run:]) == "" {
				if capture {
					var payload struct {
						Slugs     []string `json:"slugs"`
						TestFiles []string `json:"testFiles"`
					}
					if json.Unmarshal([]byte(strings.Join(block, "\n")), &payload) == nil && (len(payload.Slugs) > 0 || len(payload.TestFiles) > 0) {
						slugs, testFiles, found = payload.Slugs, payload.TestFiles, true
					}
				}
				fence, block, capture = "", nil, false
			} else if capture {
				block = append(block, line)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			run := len(trimmed) - len(strings.TrimLeft(trimmed, trimmed[:1]))
			fence = trimmed[:run]
			info := strings.TrimSpace(trimmed[run:])
			capture = inHandoff && !found && (info == "json" || info == "")
			continue
		}
		if strings.HasPrefix(trimmed, "## Task:") && slug == "" {
			slug = strings.TrimSpace(strings.TrimPrefix(trimmed, "## Task:"))
		}
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			inHandoff = trimmed == "## Handoff to Builder"
		}
	}
	if slugs == nil {
		slugs = strings.Split(slug, ",")
	}
	return slug, slugs, testFiles
}

func readTopNSlugs(workspace string) ([]string, bool) {
	body, ok := readWorkspaceFile(workspace, triageReportName)
	if !ok {
		return nil, false
	}
	var slugs []string
	inSection := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inSection = strings.HasPrefix(trimmed, "## top_n")
			continue
		}
		if !inSection {
			continue
		}
		if slug := listItemSlug(trimmed); slug != "" {
			slugs = append(slugs, slug)
		}
	}
	return slugs, true
}

func listItemSlug(trimmed string) string {
	if !strings.HasPrefix(trimmed, "- ") {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	if i := strings.Index(rest, ":"); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}

func readClaimedSlug(workspace string) (string, bool) {
	body, ok := readWorkspaceFile(workspace, buildReportName)
	if !ok {
		return "", false
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Task:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "## Task:")), true
		}
	}
	return "", true
}

func readWorkspaceFile(workspace, name string) (string, bool) {
	if workspace == "" {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(workspace, name))
	if err != nil {
		return "", false
	}
	return string(data), true
}
