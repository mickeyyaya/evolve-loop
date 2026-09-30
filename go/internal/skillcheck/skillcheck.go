// Package skillcheck projects the phase SKILL.md facts regions, command stubs
// and Codex manifests from their single sources, and reports their drift.
// See docs/architecture/packages/internal-skillcheck.md.
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
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
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

type skillSection struct {
	Canonical  string
	Alternates []string
}

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

type factsDiff struct {
	rel     string
	path    string
	next    string
	drifted bool
}

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

	manifestProblems, mErr := ManifestProblems(projectRoot)
	if mErr != nil {
		fmt.Fprintf(stderr, "%v\n", mErr)
		return 1
	}
	for _, mp := range manifestProblems {
		fmt.Fprintln(stderr, mp)
		drift = true
	}
	projected, pErr := projectSurfaces(projectRoot, diffs, write, stdout, stderr)
	if pErr != nil {
		fmt.Fprintf(stderr, "%v\n", pErr)
		return 1
	}
	drift = drift || projected

	if !write && drift {
		return 2
	}
	if !write {
		fmt.Fprintln(stdout, "[skills] check OK — all phase-facts regions in sync, all commands mirrored, Codex manifests projected, all names match dirs")
	}
	return 0
}

func projectSurfaces(projectRoot string, diffs []factsDiff, write bool, stdout, stderr io.Writer) (bool, error) {
	drift := false
	for _, d := range diffs {
		if !d.drifted {
			continue
		}
		if write {
			if werr := writeGenerated(d.path, d.next, d.rel, stdout); werr != nil {
				return false, werr
			}
		} else {
			fmt.Fprintf(stderr, "DRIFT: %s phase-facts region is stale (run `evolve skills generate`)\n", d.rel)
			drift = true
		}
	}

	cmdDiffs, cmdErr := commandDiffs(projectRoot)
	if cmdErr != nil {
		return false, cmdErr
	}
	cmdDrift, cmdErr := projectCommandDiffs(cmdDiffs, write, stdout, stderr)
	if cmdErr != nil {
		return false, cmdErr
	}

	codexDiffs, codexErr := codexManifestDiffs(projectRoot)
	if codexErr != nil {
		return false, codexErr
	}
	codexDrift, codexErr := projectCommandDiffs(codexDiffs, write, stdout, stderr)
	if codexErr != nil {
		return false, codexErr
	}
	return drift || cmdDrift || codexDrift, nil
}

func projectCommandDiffs(diffs []commandDiff, write bool, stdout, stderr io.Writer) (bool, error) {
	drift := false
	for _, d := range diffs {
		if !d.drifted {
			continue
		}
		if write {
			if d.orphan {
				if rmErr := os.Remove(d.path); rmErr != nil && !os.IsNotExist(rmErr) {
					return false, fmt.Errorf("remove %s: %w", d.path, rmErr)
				}
				fmt.Fprintf(stdout, "[skills] reaped orphan %s\n", d.rel)
				continue
			}
			if werr := writeGenerated(d.path, d.next, d.rel, stdout); werr != nil {
				return false, werr
			}
		} else {
			if d.orphan {
				fmt.Fprintf(stderr, "DRIFT: %s is an orphaned generated command (run `evolve skills generate`)\n", d.rel)
			} else {
				fmt.Fprintf(stderr, "DRIFT: %s is stale or missing (run `evolve skills generate`)\n", d.rel)
			}
			drift = true
		}
	}
	return drift, nil
}

func writeGenerated(path, next, rel string, stdout io.Writer) error {
	if werr := atomicwrite.Bytes(path, []byte(next)); werr != nil {
		return fmt.Errorf("write %s: %w", path, werr)
	}
	fmt.Fprintf(stdout, "[skills] generated %s\n", rel)
	return nil
}

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
	cmdDiffs, cmdErr := commandDiffs(projectRoot)
	if cmdErr != nil {
		return nil, cmdErr
	}
	for _, d := range cmdDiffs {
		if d.drifted {
			drift = append(drift, d.rel)
		}
	}
	codexDiffs, codexErr := codexManifestDiffs(projectRoot)
	if codexErr != nil {
		return nil, codexErr
	}
	for _, d := range codexDiffs {
		if d.drifted {
			drift = append(drift, d.rel)
		}
	}
	manifestProblems, mErr := ManifestProblems(projectRoot)
	if mErr != nil {
		return nil, mErr
	}
	return append(append(drift, nameErrs...), manifestProblems...), nil
}

func sortedPhaseSkillNames() []string {
	names := make([]string, 0, len(phaseSkillDirs))
	for n := range phaseSkillDirs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

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
			continue
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

	loader := profiles.NewFromDir(filepath.Join(projectRoot, ".evolve", "profiles"))
	if p, err := loader.Get(f.Role); err == nil {
		f.CLI = p.CLI
		f.Tier = p.ModelTierDefault
		f.FanOut = parallelSubtaskCount(p.Raw)
	}

	c := phaseContract(spec)
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

func phaseContract(spec phasespec.PhaseSpec) phasecontract.Contract {
	c, ok := phasecontract.For(spec.Name)
	if !ok {
		c = phasecontract.FromSpec(spec)
		if len(spec.Outputs.Files) == 0 {
			c.ArtifactName = ""
		}
	}
	return c
}

func registryRoles(projectRoot string) map[string]string {
	roles := map[string]string{}
	raw, err := os.ReadFile(config.RegistryPath(projectRoot))
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

func alternatesOf(s phasecontract.Section) []string {
	var alts []string
	for _, a := range s.Accepted {
		if a != s.Canonical {
			alts = append(alts, a)
		}
	}
	return alts
}

func spliceGeneratedRegion(doc, block string) (string, error) {
	return SpliceMarkedRegion(doc, block, factsBegin, factsEnd, "\n## Composition")
}

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
