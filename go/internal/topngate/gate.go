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

// Report names come from the phasecontract registry so this gate never re-declares them.
var (
	triageReportName = phasecontract.ArtifactName("triage")
	buildReportName  = phasecontract.ArtifactName("build")
	tddReportName    = phasecontract.ArtifactName("tdd")
	scoutReportName  = phasecontract.ArtifactName(string(core.PhaseScout))
)

// gate is one inter-phase check. check returns a reason for every finding and
// block=true only for a certain violation; any ambiguity fails open.
type gate interface {
	name() string
	appliesTo(phase string) bool
	check(in core.ReviewInput) (reason string, block bool)
}

// topNBindingGate compares build-report.md's ## Task: slug with triage's
// ## top_n. It never blocks.
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
	// Label drift is advisory: the lane exists because triage committed these
	// ids, so the committed set binds, not the report's label.
	return "label drift (advisory since 2026-07-22): build-report labels its task '" + claimed + "' but triage committed {" + strings.Join(topN, ", ") + "} — binding to the committed set", false
}

// tddScopeGate checks the TDD phase's declared members and authored test files
// against the committed task set.
type tddScopeGate struct{}

func (tddScopeGate) name() string { return "topn-tdd-scope" }

func (tddScopeGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

// check requires exact set equality for a multi-member lane, then judges file
// scope against the union of the members' scopes. For a zero- or single-member
// lane it blocks only files authored under an empty top_n; label drift and
// file-scope drift are advisory.
func (tddScopeGate) check(in core.ReviewInput) (string, bool) {
	topN, ok := readTopNSlugs(in.Workspace)
	if !ok {
		return "", false
	}
	claimed, declared, authored, ok := readTDDScope(in.Workspace)
	if !ok {
		return "", false
	}
	// Multi-member lanes bind to the contract's ids, not the markdown top_n,
	// whose decomposition sub-ids would block a lane that declared its contract.
	if committed := normalizedSlugs(core.ContractTaskIDs(in.Workspace)); len(committed) > 1 {
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
			// A matching label proves nothing about the files authored, so check their scope too.
			return fileScopeAdvisory(in.Workspace, claimed, authored), false
		}
	}
	// Label drift is advisory here for the same reason as in topNBindingGate.
	return "label drift (advisory since 2026-07-23): TDD authored test file(s) {" + strings.Join(authored, ", ") +
		"} labelled '" + claimed + "' but triage committed {" + strings.Join(topN, ", ") +
		"} — binding to the committed set", false
}

// fileScopeAdvisory returns a reason when no authored file shares a scope with
// the committed item's scout targetFiles, and "" on any ambiguity.
func fileScopeAdvisory(workspace, slug string, authored []string) string {
	declared := readScoutTargetFiles(workspace, slug)
	if len(declared) == 0 || len(authored) == 0 || anyPathOverlaps(authored, declared) {
		return ""
	}
	return "file scope drift (advisory): TDD authored test file(s) {" + strings.Join(authored, ", ") +
		"} but the committed item '" + slug + "' declares targetFiles {" + strings.Join(declared, ", ") +
		"} — zero path overlap"
}

// fileScopeAdvisoryMulti returns a reason when no authored file shares a scope
// with ANY committed member's targetFiles. A member without a declared scope
// could own any file, so it makes the judgement ambiguous and returns "".
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

// pathsOverlap treats one directory as one scope: scout names the production
// file while TDD authors its sibling _test.go.
func pathsOverlap(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	return a == b || filepath.Dir(a) == filepath.Dir(b)
}

// readScoutTargetFiles returns the backticked paths on the "- **targetFiles:**"
// line of scout-report.md's "### Task N: <slug>" block for slug, or nil.
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

// taskHeaderSlug returns the slug of a "### Task N: <slug>" header, or "".
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
	for i := 1; i < len(parts); i += 2 { // odd indices are the quoted spans
		if p := strings.TrimSpace(parts[i]); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// reconcileMemberSets blocks unless declared equals committed as a set.
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

// normalizedSlugs makes set comparisons and defect messages order-independent.
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

// readTDDScope parses test-report.md; ok is false only when the report is unreadable.
func readTDDScope(workspace string) (slug string, slugs, testFiles []string, ok bool) {
	body, ok := readWorkspaceFile(workspace, tddReportName)
	if !ok {
		return "", nil, nil, false
	}
	slug, slugs, testFiles = parseTDDReport(body)
	return slug, slugs, testFiles, true
}

// parseTDDReport takes slugs from the first declaring JSON fence under
// "## Handoff to Builder", else from the comma-separated ## Task: header.
// A fence closes only on its own delimiter character, at least as long as the
// opener, so a fence nested inside an example stays part of the example.
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
					// A fence declaring neither slugs nor testFiles must not shadow a later declaration.
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

// readTopNSlugs returns the slugs under triage-report.md's ## top_n. ok is
// false only when the report is unreadable, so an empty top_n stays visible.
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

// listItemSlug returns the slug of a "- <slug>: description" bullet, or "".
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

// readClaimedSlug returns build-report.md's "## Task: <slug>" value; ok is
// false only when the report is unreadable.
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

// readWorkspaceFile refuses an empty workspace, which would resolve name against the cwd.
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
