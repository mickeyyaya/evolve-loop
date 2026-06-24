// Package skillcheck is the reusable projection half of ADR-0040: it renders
// the marker-delimited GENERATED:phase-facts region of each phase SKILL.md from
// its SSOTs (phase registry, phasecontract headings, dispatch profiles) and
// either writes it (generate) or reports drift (check).
//
// It was extracted from cmd/evolve so BOTH the `evolve skills` CLI AND the
// autonomous cycle's audit phase can run the SAME drift check in-process —
// without the audit (an internal package) importing package main. Run preserves
// the exact CLI behavior; Check is the pure, print-free gate the audit calls.
package skillcheck

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

//go:embed templates/skill.md.tmpl
var skillFactsTmpl string

const (
	factsBegin = "<!-- GENERATED:phase-facts BEGIN"
	factsEnd   = "<!-- GENERATED:phase-facts END -->"
)

// phaseSkillDirs maps registry phase name → skill directory under skills/.
// Only these built-in phase skills carry a generated phase-facts region; the
// macro (skills/loop) and utility skills are pipeline-wide or phase-agnostic.
var phaseSkillDirs = map[string]string{
	"scout":         "scout",
	"plan-review":   "plan-review",
	"tdd":           "tdd",
	"build":         "build",
	"audit":         "audit",
	"ship":          "ship",
	"retrospective": "retro",
	"intent":        "intent",
}

// skillSection is one required artifact section as rendered into the doc: the
// canonical heading plus any tolerated legacy variants.
type skillSection struct {
	Canonical  string
	Alternates []string
}

// skillFacts is the template payload — every field traces to exactly one SSOT
// (see the table in ADR-0040 §2).
type skillFacts struct {
	Phase            string
	Archetype        string
	Optional         bool
	EnableVar        string
	Role             string
	PersonaPath      string
	CLI              string
	Tier             string
	FanOut           int
	InputFiles       []string
	ArtifactName     string
	WriteTargetLabel string
	Sections         []skillSection
	Verdicts         []string
}

// factsDiff is one phase skill's projected-vs-disk comparison.
type factsDiff struct {
	rel     string // "skills/<dir>/SKILL.md" (display + drift identity)
	path    string // absolute on-disk path
	next    string // the regenerated full document
	drifted bool   // next != the current on-disk content
}

// inspect renders every phase skill's expected content and compares it to disk,
// and collects every skill's frontmatter name!=dir mismatch. It is PURE: no
// writes, no prints — the seam Run (CLI) and Check (audit) both build on.
func inspect(projectRoot string) (diffs []factsDiff, nameErrs, warns []string, err error) {
	cat, _, catWarns, err := phasespec.MergedCatalog(projectRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load phase catalog: %w", err)
	}
	warns = append(warns, catWarns...)

	tmpl, err := template.New("skill-facts").Parse(skillFactsTmpl)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse embedded template: %w", err)
	}

	nameErrs = nameMismatches(projectRoot)
	roles := registryRoles(projectRoot)

	for _, phase := range sortedPhaseSkillNames() {
		spec, ok := cat.Get(phase)
		if !ok {
			warns = append(warns, fmt.Sprintf("phase %q not in catalog; skipping", phase))
			continue
		}
		rel := filepath.Join("skills", phaseSkillDirs[phase], "SKILL.md")
		skillPath := filepath.Join(projectRoot, "skills", phaseSkillDirs[phase], "SKILL.md")
		current, readErr := os.ReadFile(skillPath)
		if readErr != nil {
			return nil, nil, nil, fmt.Errorf("read %s: %w", skillPath, readErr)
		}
		facts := collectSkillFacts(projectRoot, spec, roles)
		var block bytes.Buffer
		if execErr := tmpl.Execute(&block, facts); execErr != nil {
			return nil, nil, nil, fmt.Errorf("render %s: %w", phase, execErr)
		}
		next, spliceErr := spliceGeneratedRegion(string(current), block.String())
		if spliceErr != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", skillPath, spliceErr)
		}
		diffs = append(diffs, factsDiff{rel: rel, path: skillPath, next: next, drifted: next != string(current)})
	}
	return diffs, nameErrs, warns, nil
}

// Run reproduces `evolve skills <generate|check>`: write=true rewrites each
// stale GENERATED:phase-facts region in place; write=false reports drift and
// returns exit 2 if any region is stale or any frontmatter name != its dir.
// Output is byte-compatible with the pre-extraction cmd path.
func Run(projectRoot string, write bool, stdout, stderr io.Writer) int {
	diffs, nameErrs, warns, err := inspect(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	for _, w := range warns {
		fmt.Fprintln(stderr, "WARN:", w)
	}
	drift := false
	for _, ne := range nameErrs {
		fmt.Fprintln(stderr, ne)
		drift = true
	}
	for _, d := range diffs {
		if !d.drifted {
			continue
		}
		if write {
			if werr := atomicwrite.Bytes(d.path, []byte(d.next)); werr != nil {
				fmt.Fprintf(stderr, "write %s: %v\n", d.path, werr)
				return 1
			}
			fmt.Fprintf(stdout, "[skills] generated %s\n", d.rel)
		} else {
			fmt.Fprintf(stderr, "DRIFT: %s phase-facts region is stale (run `evolve skills generate`)\n", d.rel)
			drift = true
		}
	}
	if !write && drift {
		return 2
	}
	if !write {
		fmt.Fprintln(stdout, "[skills] check OK — all phase-facts regions in sync, all names match dirs")
	}
	return 0
}

// Check is the read-only drift gate for in-process callers (the cycle audit):
// it returns the rel-paths whose SKILL.md phase-facts region is stale plus any
// frontmatter-name!=dir messages (empty when everything is in sync). err is
// non-nil only on an infrastructure fault (catalog/template/read failure), so
// the caller can fail OPEN on infra while FAILing on real drift.
func Check(projectRoot string) ([]string, error) {
	diffs, nameErrs, _, err := inspect(projectRoot)
	if err != nil {
		return nil, err
	}
	var drift []string
	for _, d := range diffs {
		if d.drifted {
			drift = append(drift, d.rel)
		}
	}
	return append(drift, nameErrs...), nil
}

// sortedPhaseSkillNames returns the projected phase names in stable order so
// generate/check output is deterministic.
func sortedPhaseSkillNames() []string {
	names := make([]string, 0, len(phaseSkillDirs))
	for n := range phaseSkillDirs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// nameMismatches enforces the ADR-0040 naming rule: every skill dir's SKILL.md
// frontmatter `name` must equal the directory name. Returns one human message
// per violation (empty when all match) — pure, so both Run and Check decide
// what to do with them.
func nameMismatches(projectRoot string) []string {
	skillsDir := filepath.Join(projectRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return []string{fmt.Sprintf("read skills dir: %v", err)}
	}
	var bad []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(skillsDir, e.Name(), "SKILL.md"))
		if err != nil {
			continue // dirs without SKILL.md are not skills
		}
		fm, _, err := prompts.ParseFrontmatter(string(raw))
		if err != nil {
			bad = append(bad, fmt.Sprintf("DRIFT: skills/%s/SKILL.md: unparseable frontmatter: %v", e.Name(), err))
			continue
		}
		name, _ := fm["name"].(string)
		if name != e.Name() {
			bad = append(bad, fmt.Sprintf("DRIFT: skills/%s/SKILL.md frontmatter name %q != dir name (ADR-0040 rule 3)", e.Name(), name))
		}
	}
	return bad
}

// collectSkillFacts gathers the template payload from the three SSOTs. Every
// lookup is fail-soft: a missing profile or persona renders as a gap, never an
// error — the drift detection (not this collector) decides what is fatal.
func collectSkillFacts(projectRoot string, spec phasespec.PhaseSpec, roles map[string]string) skillFacts {
	f := skillFacts{
		Phase:     spec.Name,
		Archetype: string(spec.RoleOrDefault()),
		Optional:  spec.Optional,
		EnableVar: spec.EnableVar,
		Role:      roles[spec.Name],
	}
	if f.Role == "" {
		f.Role = spec.Name
	}
	f.PersonaPath = personaPath(projectRoot, spec, f.Role)

	// Dispatch facts — .evolve/profiles/<role>.json is the SSOT.
	loader := profiles.NewFromDir(filepath.Join(projectRoot, ".evolve", "profiles"))
	if p, err := loader.Get(f.Role); err == nil {
		f.CLI = p.CLI
		f.Tier = p.ModelTierDefault
		f.FanOut = parallelSubtaskCount(p.Raw)
	}

	// Artifact + section facts — phasecontract is the SSOT for built-in
	// headings; FromSpec derives user-phase contracts from classify rules.
	c, ok := phasecontract.For(spec.Name)
	if !ok {
		c = phasecontract.FromSpec(spec)
		if len(spec.Outputs.Files) == 0 {
			// FromSpec defaults to "<phase>-report.md"; for a phase that declares
			// no output files (e.g. ship — its deliverable is the commit itself)
			// that default would be fiction. Suppress it.
			c.ArtifactName = ""
		}
	}
	f.ArtifactName = c.ArtifactName
	f.WriteTargetLabel = "cycle workspace"
	if c.WriteTarget == phasecontract.TargetEvolveDir {
		f.WriteTargetLabel = ".evolve/"
	}
	for _, s := range c.Sections {
		f.Sections = append(f.Sections, skillSection{
			Canonical:  s.Canonical,
			Alternates: alternatesOf(s),
		})
	}
	f.Verdicts = c.Verdicts

	for _, in := range spec.Inputs.Files {
		f.InputFiles = append(f.InputFiles, filepath.Base(in))
	}
	return f
}

// registryRoles reads the phase→agent/profile-name mapping from the registry's
// raw "role" keys, once. PhaseSpec maps JSON "archetype" onto its Role field and
// does not carry the registry's "role" (profile name), so we read it directly —
// the registry stays the single home for the mapping. Fail-soft: an unreadable
// registry yields an empty map (gaps, not errors).
func registryRoles(projectRoot string) map[string]string {
	roles := map[string]string{}
	raw, err := os.ReadFile(filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json"))
	if err != nil {
		return roles
	}
	var reg struct {
		Phases []struct {
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"phases"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		return roles
	}
	for _, p := range reg.Phases {
		roles[p.Name] = p.Role
	}
	return roles
}

// personaPath resolves the persona markdown for a phase: explicit spec.Agent
// first, then the evolve-<role>.md / <role>.md conventions. Returns "" when no
// persona file exists (native phases like ship).
func personaPath(projectRoot string, spec phasespec.PhaseSpec, role string) string {
	var candidates []string
	if spec.Agent != "" {
		candidates = append(candidates, spec.Agent)
	}
	candidates = append(candidates, "evolve-"+role, role)
	for _, c := range candidates {
		rel := filepath.Join("agents", c+".md")
		if _, err := os.Stat(filepath.Join(projectRoot, rel)); err == nil {
			return rel
		}
	}
	return ""
}

// parallelSubtaskCount reads the un-modeled parallel_subtasks array from the
// profile's raw bytes (profiles.Profile keeps it in Raw by design).
func parallelSubtaskCount(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var p struct {
		ParallelSubtasks []json.RawMessage `json:"parallel_subtasks"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0
	}
	return len(p.ParallelSubtasks)
}

// alternatesOf returns a section's tolerated legacy headings (Accepted minus the
// canonical first entry).
func alternatesOf(s phasecontract.Section) []string {
	var alts []string
	for _, a := range s.Accepted {
		if a != s.Canonical {
			alts = append(alts, a)
		}
	}
	return alts
}

// spliceGeneratedRegion replaces the marker-delimited region in doc with block,
// preserving everything outside the markers. When no markers exist the block is
// inserted before "## Composition" (the conventional tail section) or appended.
func spliceGeneratedRegion(doc, block string) (string, error) {
	return SpliceMarkedRegion(doc, block, factsBegin, factsEnd, "\n## Composition")
}

// SpliceMarkedRegion replaces a marker-delimited region in doc with block,
// preserving everything outside the markers — the marker-agnostic splice shared
// by `evolve skills` and `evolve flags`. fallbackAnchor names the section the
// block is inserted before when no markers exist yet ("" = append at EOF).
// Exactly ONE marker pair is the invariant: a BEGIN without END is corruption,
// and a second BEGIN (e.g. from a botched manual merge) errors out rather than
// leaving an orphaned stale region behind.
func SpliceMarkedRegion(doc, block, beginMarker, endMarker, fallbackAnchor string) (string, error) {
	block = strings.TrimRight(block, "\n") + "\n"
	begin := strings.Index(doc, beginMarker)
	if begin >= 0 {
		endRel := strings.Index(doc[begin:], endMarker)
		if endRel < 0 {
			return "", fmt.Errorf("found %q without matching END marker", beginMarker)
		}
		end := begin + endRel + len(endMarker)
		if strings.Contains(doc[end:], beginMarker) {
			return "", fmt.Errorf("multiple %q regions found; keep exactly one pair", beginMarker)
		}
		// Swallow a single trailing newline so regeneration is idempotent.
		if end < len(doc) && doc[end] == '\n' {
			end++
		}
		return doc[:begin] + block + doc[end:], nil
	}
	if fallbackAnchor != "" {
		if at := strings.Index(doc, fallbackAnchor); at >= 0 {
			return doc[:at+1] + block + "\n" + doc[at+1:], nil
		}
	}
	return strings.TrimRight(doc, "\n") + "\n\n" + block, nil
}
