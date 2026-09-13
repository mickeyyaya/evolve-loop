// Package auditcalibration summarizes auditor narratives against deterministic
// ship-gate outcomes recorded in the cycle corpus.
package auditcalibration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

var cycleDirRE = regexp.MustCompile(`^cycle-([1-9][0-9]*)$`)

type pair struct {
	cycle                  int
	narrative, chain, gate string
	shipped, overriddenBy  string
	classes                []string
}

type exclusion struct {
	cycle  int
	reason string
	detail string
}

// Generate reads dossiersDir and runsDir and returns a deterministic Markdown
// calibration report. The sample is the set of canonical cycle-N run dirs.
func Generate(dossiersDir, runsDir string) ([]byte, error) {
	if err := requireDir("dossiers directory", dossiersDir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, fmt.Errorf("runs directory %q: %w", runsDir, err)
	}

	var cycles []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		match := cycleDirRE.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		cycle, _ := strconv.Atoi(match[1])
		cycles = append(cycles, cycle)
	}
	sort.Ints(cycles)

	pairs := make([]pair, 0, len(cycles))
	exclusions := make([]exclusion, 0)
	for _, cycle := range cycles {
		p, ex := loadPair(cycle, dossiersDir, runsDir)
		if ex != nil {
			exclusions = append(exclusions, *ex)
			continue
		}
		pairs = append(pairs, *p)
	}
	return render(pairs, exclusions), nil
}

func requireDir(label, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", label, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %q is not a directory", label, path)
	}
	return nil
}

func loadPair(cycle int, dossiersDir, runsDir string) (*pair, *exclusion) {
	runDir := filepath.Join(runsDir, fmt.Sprintf("cycle-%d", cycle))
	shadowRaw, err := os.ReadFile(filepath.Join(runDir, auditchain.ShadowRecordFile))
	if err != nil {
		reason := "malformed-shadow"
		if os.IsNotExist(err) {
			reason = "missing-shadow"
		}
		return nil, excluded(cycle, reason, err)
	}
	var shadow auditchain.ShadowRecord
	if err := json.Unmarshal(shadowRaw, &shadow); err != nil || !validShadow(shadow, cycle) {
		if err == nil {
			err = fmt.Errorf("invalid verdict fields or cycle binding")
		}
		return nil, excluded(cycle, "malformed-shadow", err)
	}

	dossierRaw, err := os.ReadFile(filepath.Join(dossiersDir, fmt.Sprintf("cycle-%d.json", cycle)))
	if err != nil {
		reason := "malformed-dossier"
		if os.IsNotExist(err) {
			reason = "missing-dossier"
		}
		return nil, excluded(cycle, reason, err)
	}
	d, err := dossier.ParseJSON(dossierRaw)
	if err != nil || d.Cycle != cycle || d.Validate() != nil {
		if err == nil {
			err = fmt.Errorf("invalid dossier or cycle binding")
		}
		return nil, excluded(cycle, "malformed-dossier", err)
	}

	classes, err := readClasses(filepath.Join(runDir, "audit-fail-reason.json"))
	if err != nil {
		return nil, excluded(cycle, "malformed-fail-reason", err)
	}
	shipped := strings.TrimSpace(shadow.ShippedVerdict)
	if shipped == "" {
		shipped = d.FinalVerdict
	}
	gate := "PASS"
	if len(shadow.OverrodeBy) > 0 {
		gate = "FAIL"
	}
	return &pair{
		cycle:        cycle,
		narrative:    strings.TrimSpace(shadow.NarrativeVerdict),
		chain:        strings.TrimSpace(shadow.ChainVerdict),
		gate:         gate,
		shipped:      shipped,
		overriddenBy: strings.Join(shadow.OverrodeBy, ", "),
		classes:      classes,
	}, nil
}

func validShadow(s auditchain.ShadowRecord, cycle int) bool {
	return s.Cycle == cycle && validNarrative(s.NarrativeVerdict) &&
		(strings.TrimSpace(s.ChainVerdict) == "absent" || validNarrative(s.ChainVerdict)) &&
		(strings.TrimSpace(s.ShippedVerdict) == "" || validNarrative(s.ShippedVerdict))
}

func validNarrative(v string) bool {
	switch strings.TrimSpace(v) {
	case dossier.VerdictPass, dossier.VerdictWarn, dossier.VerdictFail:
		return true
	default:
		return false
	}
}

func readClasses(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record struct {
		Reasons []string `json:"reasons"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, reason := range record.Reasons {
		class, _, _ := strings.Cut(strings.TrimSpace(reason), ":")
		if class != "" {
			seen[class] = true
		}
	}
	classes := make([]string, 0, len(seen))
	for class := range seen {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	return classes, nil
}

func excluded(cycle int, reason string, err error) *exclusion {
	detail := "artifact is unavailable"
	if err != nil {
		detail = err.Error()
	}
	return &exclusion{cycle: cycle, reason: reason, detail: detail}
}

func render(pairs []pair, exclusions []exclusion) []byte {
	matrix := make(map[[2]string]int)
	classCounts := make(map[string]int)
	for _, p := range pairs {
		matrix[[2]string{p.narrative, p.gate}]++
		for _, class := range p.classes {
			classCounts[class]++
		}
	}

	var b bytes.Buffer
	fmt.Fprintln(&b, "# Auditor Calibration Report")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Sample rule: canonical `cycle-N` directories under the runs directory, each paired with its committed dossier.")
	fmt.Fprintln(&b, "Exclusion rule: missing or malformed pair artifacts are counted and listed; they never enter the agreement matrix.")
	fmt.Fprintln(&b, "Interpretation rule: the gate is FAIL when a deterministic override is recorded, otherwise PASS. This report does not recommend changing a persona rubric from a single anecdote.")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Valid pairs: %d\n", len(pairs))
	fmt.Fprintf(&b, "Excluded: %d\n", len(exclusions))

	fmt.Fprintln(&b, "\n## Narrative × Deterministic Gate Matrix")
	fmt.Fprintln(&b, "| Narrative | Gate | Count |")
	fmt.Fprintln(&b, "|---|---|---:|")
	for _, narrative := range []string{"PASS", "WARN", "FAIL"} {
		for _, gate := range []string{"PASS", "FAIL"} {
			if count := matrix[[2]string{narrative, gate}]; count > 0 {
				fmt.Fprintf(&b, "| %s | %s | %d |\n", narrative, gate, count)
			}
		}
	}

	fmt.Fprintln(&b, "\n## Defect Classes")
	fmt.Fprintln(&b, "| Class | Count |")
	fmt.Fprintln(&b, "|---|---:|")
	classes := make([]string, 0, len(classCounts))
	for class := range classCounts {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	for _, class := range classes {
		fmt.Fprintf(&b, "| %s | %d |\n", markdownCell(class), classCounts[class])
	}

	fmt.Fprintln(&b, "\n## Force Overrides")
	fmt.Fprintln(&b, "| Cycle | Narrative | Shipped | Overrode By |")
	fmt.Fprintln(&b, "|---:|---|---|---|")
	for _, p := range pairs {
		if p.overriddenBy != "" {
			fmt.Fprintf(&b, "| %d | %s | %s | %s |\n", p.cycle, p.narrative, p.shipped, markdownCell(p.overriddenBy))
		}
	}

	fmt.Fprintln(&b, "\n## Valid Pairs")
	fmt.Fprintln(&b, "| Cycle | Narrative | Chain | Gate | Shipped | Overrode By | Defect Classes |")
	fmt.Fprintln(&b, "|---:|---|---|---|---|---|---|")
	for _, p := range pairs {
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | %s |\n", p.cycle, p.narrative, p.chain, p.gate, p.shipped, markdownCell(p.overriddenBy), markdownCell(strings.Join(p.classes, ", ")))
	}

	fmt.Fprintln(&b, "\n## Exclusions")
	fmt.Fprintln(&b, "| Cycle | Reason | Detail |")
	fmt.Fprintln(&b, "|---:|---|---|")
	for _, ex := range exclusions {
		fmt.Fprintf(&b, "| %d | %s | %s |\n", ex.cycle, ex.reason, markdownCell(ex.detail))
	}
	return b.Bytes()
}

func markdownCell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", `\|`)
}
