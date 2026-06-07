// cmd_skills.go implements `evolve skills` — the projection half of
// ADR-0040. Phase skill docs (skills/<name>/SKILL.md) must not hand-duplicate
// facts whose authoritative homes already exist (phase registry, phasecontract
// headings, dispatch profiles). This command renders those facts into a
// marker-delimited region of each phase SKILL.md:
//
//	evolve skills generate   rewrite the GENERATED:phase-facts region in place
//	evolve skills check      exit 2 if any region drifted from its SSOTs, or
//	                         any skill's frontmatter name != its directory name
//
// Hand-written prose outside the markers is preserved byte-for-byte. The
// region is inserted before "## Composition" (or appended) on first run.
// Drift enforcement: cmd_skills_drift_test.go runs `check` against the live
// repo, same pattern as phasecontract/contract_test.go.
package main

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
	skillFactsBegin = "<!-- GENERATED:phase-facts BEGIN"
	skillFactsEnd   = "<!-- GENERATED:phase-facts END -->"
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

// skillSection is one required artifact section as rendered into the doc:
// the canonical heading plus any tolerated legacy variants.
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

func runSkills(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evolve skills <generate|check|publish>")
		return 10
	}
	project := envOrCwd("EVOLVE_PROJECT_ROOT")
	switch args[0] {
	case "generate":
		return skillsRun(project, true, stdout, stderr)
	case "check":
		return skillsRun(project, false, stdout, stderr)
	case "publish":
		cfg, ok := parsePublishFlags(args[1:], stderr)
		if !ok {
			return 10
		}
		return runSkillsPublish(project, cfg, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q (want generate|check|publish)\n", args[0])
		return 10
	}
}

// skillsRun renders every phase skill's expected content and either writes it
// (generate) or compares it against disk (check). It also asserts the
// frontmatter-name == dir-name invariant for EVERY skill, phase or utility.
func skillsRun(project string, write bool, stdout, stderr io.Writer) int {
	cat, _, warns, err := mergedCatalog(project)
	if err != nil {
		fmt.Fprintf(stderr, "load phase catalog: %v\n", err)
		return 1
	}
	for _, w := range warns {
		fmt.Fprintln(stderr, "WARN:", w)
	}
	tmpl, err := template.New("skill-facts").Parse(skillFactsTmpl)
	if err != nil {
		fmt.Fprintf(stderr, "parse embedded template: %v\n", err)
		return 1
	}

	drift := skillNameMismatches(project, stderr)
	roles := registryRoles(project)

	for _, phase := range sortedPhaseSkillNames() {
		spec, ok := cat.Get(phase)
		if !ok {
			fmt.Fprintf(stderr, "WARN: phase %q not in catalog; skipping\n", phase)
			continue
		}
		skillPath := filepath.Join(project, "skills", phaseSkillDirs[phase], "SKILL.md")
		current, err := os.ReadFile(skillPath)
		if err != nil {
			fmt.Fprintf(stderr, "read %s: %v\n", skillPath, err)
			return 1
		}
		facts := collectSkillFacts(project, spec, roles)
		var block bytes.Buffer
		if err := tmpl.Execute(&block, facts); err != nil {
			fmt.Fprintf(stderr, "render %s: %v\n", phase, err)
			return 1
		}
		next, err := spliceGeneratedRegion(string(current), block.String())
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", skillPath, err)
			return 1
		}
		rel := filepath.Join("skills", phaseSkillDirs[phase], "SKILL.md")
		if next == string(current) {
			continue
		}
		if write {
			if err := atomicwrite.Bytes(skillPath, []byte(next)); err != nil {
				fmt.Fprintf(stderr, "write %s: %v\n", skillPath, err)
				return 1
			}
			fmt.Fprintf(stdout, "[skills] generated %s\n", rel)
		} else {
			fmt.Fprintf(stderr, "DRIFT: %s phase-facts region is stale (run `evolve skills generate`)\n", rel)
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

// skillNameMismatches enforces the ADR-0040 naming rule: every skill dir's
// SKILL.md frontmatter `name` must equal the directory name. Reports each
// violation to stderr; returns true if any found.
func skillNameMismatches(project string, stderr io.Writer) bool {
	skillsDir := filepath.Join(project, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		fmt.Fprintf(stderr, "read skills dir: %v\n", err)
		return true
	}
	bad := false
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
			fmt.Fprintf(stderr, "DRIFT: skills/%s/SKILL.md: unparseable frontmatter: %v\n", e.Name(), err)
			bad = true
			continue
		}
		name, _ := fm["name"].(string)
		if name != e.Name() {
			fmt.Fprintf(stderr, "DRIFT: skills/%s/SKILL.md frontmatter name %q != dir name (ADR-0040 rule 3)\n", e.Name(), name)
			bad = true
		}
	}
	return bad
}

// collectSkillFacts gathers the template payload from the three SSOTs. Every
// lookup is fail-soft: a missing profile or persona renders as a gap, never an
// error — the drift test (not this collector) decides what is fatal.
func collectSkillFacts(project string, spec phasespec.PhaseSpec, roles map[string]string) skillFacts {
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
	f.PersonaPath = personaPath(project, spec, f.Role)

	// Dispatch facts — .evolve/profiles/<role>.json is the SSOT.
	loader := profiles.NewFromDir(filepath.Join(project, ".evolve", "profiles"))
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
			// FromSpec defaults to "<phase>-report.md"; for a phase that
			// declares no output files (e.g. ship — its deliverable is the
			// commit itself) that default would be fiction. Suppress it.
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

// registryRoles reads the phase→agent/profile-name mapping from the
// registry's raw "role" keys, once. PhaseSpec maps JSON "archetype" onto its
// Role field and does not carry the registry's "role" (profile name), so we
// read it directly — the registry stays the single home for the mapping.
// Fail-soft: an unreadable registry yields an empty map (gaps, not errors).
func registryRoles(project string) map[string]string {
	roles := map[string]string{}
	raw, err := os.ReadFile(filepath.Join(project, "docs", "architecture", "phase-registry.json"))
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
func personaPath(project string, spec phasespec.PhaseSpec, role string) string {
	var candidates []string
	if spec.Agent != "" {
		candidates = append(candidates, spec.Agent)
	}
	candidates = append(candidates, "evolve-"+role, role)
	for _, c := range candidates {
		rel := filepath.Join("agents", c+".md")
		if _, err := os.Stat(filepath.Join(project, rel)); err == nil {
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

// alternatesOf returns a section's tolerated legacy headings (Accepted minus
// the canonical first entry).
func alternatesOf(s phasecontract.Section) []string {
	var alts []string
	for _, a := range s.Accepted {
		if a != s.Canonical {
			alts = append(alts, a)
		}
	}
	return alts
}

// spliceGeneratedRegion replaces the marker-delimited region in doc with
// block, preserving everything outside the markers. When no markers exist the
// block is inserted before "## Composition" (the conventional tail section) or
// appended. Exactly ONE marker pair is the invariant: a BEGIN without END is
// corruption, and a second BEGIN (e.g. from a botched manual merge) errors out
// rather than leaving an orphaned stale region behind.
func spliceGeneratedRegion(doc, block string) (string, error) {
	block = strings.TrimRight(block, "\n") + "\n"
	begin := strings.Index(doc, skillFactsBegin)
	if begin >= 0 {
		endRel := strings.Index(doc[begin:], skillFactsEnd)
		if endRel < 0 {
			return "", fmt.Errorf("found %q without matching END marker", skillFactsBegin)
		}
		end := begin + endRel + len(skillFactsEnd)
		if strings.Contains(doc[end:], skillFactsBegin) {
			return "", fmt.Errorf("multiple %q regions found; keep exactly one pair", skillFactsBegin)
		}
		// Swallow a single trailing newline so regeneration is idempotent.
		if end < len(doc) && doc[end] == '\n' {
			end++
		}
		return doc[:begin] + block + doc[end:], nil
	}
	if at := strings.Index(doc, "\n## Composition"); at >= 0 {
		return doc[:at+1] + block + "\n" + doc[at+1:], nil
	}
	return strings.TrimRight(doc, "\n") + "\n\n" + block, nil
}
