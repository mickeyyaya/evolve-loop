package phasecmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseinventory"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// createEnvelopeOut is the stable stdout JSON contract of `phases create`.
// See ADR-0038.
type createEnvelopeOut struct {
	OK               bool     `json:"ok"`
	Phase            string   `json:"phase,omitempty"`
	Artifact         string   `json:"artifact,omitempty"`
	RequiredSections []string `json:"required_sections,omitempty"`
	EmitsVerdict     bool     `json:"emits_verdict,omitempty"`
	PhaseJSON        string   `json:"phase_json,omitempty"`
	Persona          string   `json:"persona,omitempty"`
	InventoryRebuilt bool     `json:"inventory_rebuilt,omitempty"`
	Errors           []string `json:"errors,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	Hint             string   `json:"hint,omitempty"`
}

// mintSpec is an advisor mint promoted by --mint. Its JSON matches router.MintSpec plus Name,
// which a runtime mint carries on its plan entry instead.
type mintSpec struct {
	Name         string `json:"name"`
	Prompt       string `json:"prompt"`
	Tier         string `json:"tier,omitempty"`
	CLI          string `json:"cli,omitempty"`
	WritesSource bool   `json:"writes_source,omitempty"`
}

// phasesCreate implements `evolve phases create`; it exits 0 created, 1 I/O error, 2 validation or collision, 10 usage.
func phasesCreate(project string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("phases create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	specArg := fs.String("spec", "", "phase.json content: a file path, or - for stdin")
	personaArg := fs.String("persona", "", "persona body for agents/<agent>.md: a file path, or - for stdin")
	mintArg := fs.String("mint", "", "advisor MintSpec JSON to promote: a file path, or - for stdin")
	rootArg := fs.String("root", "", "discovery root to write the phase into (default: first EVOLVE_PHASE_ROOTS entry)")
	if err := fs.Parse(args); err != nil {
		return 10
	}

	if (*specArg == "") == (*mintArg == "") { // exactly one of --spec/--mint
		fmt.Fprintln(stderr, "usage: evolve phases create --spec <file|-> [--persona <file|->] | --mint <file|->")
		return 10
	}
	if stdinCount(*specArg, *personaArg, *mintArg) > 1 {
		fmt.Fprintln(stderr, "at most one of --spec/--persona/--mint may read stdin (-)")
		return 10
	}

	spec, personaBody, errs, code := loadCreateInputs(*specArg, *personaArg, *mintArg, stdin, stderr)
	if code != 0 {
		return code
	}
	if len(errs) > 0 {
		return emitEnvelope(stdout, createEnvelopeOut{OK: false, Phase: spec.Name, Errors: errs, Hint: createHint}, 2)
	}

	if v := phasespec.ValidateUserSpec(spec); len(v) > 0 {
		return emitEnvelope(stdout, createEnvelopeOut{OK: false, Phase: spec.Name, Errors: v, Hint: createHint}, 2)
	}
	warnings := softLintWarnings(spec)

	roots := phasespec.Roots(project)
	if collision := findCollision(project, roots, spec.Name); collision != "" {
		return emitEnvelope(stdout, createEnvelopeOut{
			OK: false, Phase: spec.Name, Errors: []string{collision}, Hint: createHint,
		}, 2)
	}

	// The spec is untrusted LLM input: the persona path must stay inside agents/ and never
	// overwrite another phase's persona.
	personaPath := filepath.Join(project, "agents", spec.AgentName()+".md")
	if agentsDir := filepath.Join(project, "agents") + string(filepath.Separator); !strings.HasPrefix(personaPath, agentsDir) {
		return emitEnvelope(stdout, createEnvelopeOut{
			OK: false, Phase: spec.Name,
			Errors: []string{fmt.Sprintf("agent %q resolves outside agents/ — refusing", spec.AgentName())},
			Hint:   createHint,
		}, 2)
	}
	if personaBody != "" {
		if _, err := os.Stat(personaPath); err == nil {
			return emitEnvelope(stdout, createEnvelopeOut{
				OK: false, Phase: spec.Name,
				Errors: []string{fmt.Sprintf("persona %s already exists — refusing to overwrite", relTo(project, personaPath))},
				Hint:   createHint,
			}, 2)
		}
	} else if _, err := os.Stat(personaPath); os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("no persona supplied and %s does not exist — the phase will have no prompt until one is added", relTo(project, personaPath)))
	}

	// Write phase.json, then the persona, rolling back on a persona failure. Check-then-write is not
	// atomic across concurrent invocations, which is accepted for a single-operator CLI.
	targetRoot, ok := resolveTargetRoot(*rootArg, project, roots)
	if !ok {
		fmt.Fprintf(stderr, "--root %q is neither a configured discovery root (EVOLVE_PHASE_ROOTS) nor inside the project\n", *rootArg)
		return 10
	}
	specJSON, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "marshal spec: %v\n", err)
		return 1
	}
	phaseDir := filepath.Join(targetRoot, spec.Name)
	_, statErr := os.Stat(phaseDir)
	dirCreatedByUs := os.IsNotExist(statErr)
	phasePath := filepath.Join(phaseDir, "phase.json")
	if err := atomicwrite.Bytes(phasePath, specJSON); err != nil {
		fmt.Fprintf(stderr, "write %s: %v\n", phasePath, err)
		return 1
	}
	if personaBody != "" {
		if err := atomicwrite.Bytes(personaPath, []byte(personaBody)); err != nil {
			// Remove only what this invocation created: a pre-existing directory may hold operator files.
			if dirCreatedByUs {
				_ = os.RemoveAll(phaseDir)
			} else {
				_ = os.Remove(phasePath)
			}
			fmt.Fprintf(stderr, "write %s: %v\n", personaPath, err)
			return 1
		}
	}

	// Make the phase visible to the next cycle immediately.
	invRes, invErr := phaseinventory.Build(phaseinventory.Options{
		ProjectRoot: project, Roots: roots, NowFn: time.Now, Force: true,
	})
	if invErr != nil {
		warnings = append(warnings, fmt.Sprintf("inventory rebuild failed (%v) — run `evolve phase-inventory build --force`", invErr))
	}

	contract := phasecontract.FromSpec(spec)
	sections := make([]string, 0, len(contract.Sections))
	for _, s := range contract.Sections {
		sections = append(sections, s.Canonical)
	}
	out := createEnvelopeOut{
		OK:               true,
		Phase:            spec.Name,
		Artifact:         contract.ArtifactName,
		RequiredSections: sections,
		EmitsVerdict:     len(contract.Verdicts) > 0,
		PhaseJSON:        relTo(project, phasePath),
		InventoryRebuilt: invErr == nil && !invRes.CacheHit,
		Warnings:         warnings,
	}
	if personaBody != "" {
		out.Persona = relTo(project, personaPath)
	}
	return emitEnvelope(stdout, out, 0)
}

const createHint = "fix errors and re-run: evolve phases create --spec -"

// loadCreateInputs returns parse failures as envelope errors and I/O failures as an exit code.
func loadCreateInputs(specArg, personaArg, mintArg string, stdin io.Reader, stderr io.Writer) (spec phasespec.PhaseSpec, personaBody string, errs []string, code int) {
	if mintArg != "" {
		raw, err := readArg(mintArg, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "read --mint: %v\n", err)
			return spec, "", nil, 1
		}
		var m mintSpec
		if err := json.Unmarshal(raw, &m); err != nil {
			return spec, "", []string{"mint JSON malformed: " + err.Error()}, 0
		}
		if strings.TrimSpace(m.Prompt) == "" {
			return phasespec.PhaseSpec{Name: m.Name}, "", []string{"mint.prompt is required — it becomes the persona body"}, 0
		}
		spec = phasespec.PhaseSpec{
			Name:         m.Name,
			Kind:         "llm",
			Optional:     true,
			Model:        m.Tier,
			WritesSource: m.WritesSource,
			Description:  firstLine(m.Prompt),
		}
		return spec, strings.TrimSpace(m.Prompt) + "\n", nil, 0
	}

	raw, err := readArg(specArg, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "read --spec: %v\n", err)
		return spec, "", nil, 1
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, "", []string{"spec JSON malformed: " + err.Error()}, 0
	}
	if personaArg != "" {
		body, err := readArg(personaArg, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "read --persona: %v\n", err)
			return spec, "", nil, 1
		}
		personaBody = string(body)
	}
	return spec, personaBody, nil, 0
}

// findCollision names an existing built-in or user phase called name. An unreadable registry
// fails open: the user roots are still checked.
func findCollision(project string, roots []string, name string) string {
	registryPath := config.RegistryPath(project)
	if builtin, err := phasespec.Load(registryPath); err == nil {
		if _, ok := builtin.Get(name); ok {
			return fmt.Sprintf("phase %q is a built-in — a user phase cannot redefine it", name)
		}
	}
	_, sources, _ := phasespec.DiscoverUserSpecsFromRoots(roots)
	if root, ok := sources[name]; ok {
		return fmt.Sprintf("phase %q already exists under %s", name, relTo(project, root))
	}
	return ""
}

// softLintWarnings holds the soft checks shared by `phase lint` and `phases create`.
func softLintWarnings(s phasespec.PhaseSpec) []string {
	var warnings []string
	if s.RoleOrDefault() == phasespec.RoleEvaluate {
		if s.Classify == nil || len(s.Classify.RequireSections) == 0 {
			warnings = append(warnings, "evaluate phase declares no classify.require_sections — its derived contract checks no structure")
		}
	}
	if len(s.Outputs.Files) == 0 {
		warnings = append(warnings, fmt.Sprintf("no outputs.files — the deliverable will default to %q", s.Name+"-report.md"))
	}
	for _, c := range phasespec.UnknownCategories(s) {
		warnings = append(warnings, fmt.Sprintf("unknown category %q — known: bugfix|feature|refactor|security|performance|release|docs", c))
	}
	return warnings
}

// resolveTargetRoot accepts --root only as a configured root or a path inside the project, so create
// never writes, or rolls back with RemoveAll, outside what the pipeline reads. Empty means the first root.
func resolveTargetRoot(arg, project string, roots []string) (string, bool) {
	if arg == "" {
		return roots[0], true
	}
	target := arg
	if !filepath.IsAbs(target) {
		target = filepath.Join(project, target)
	}
	target = filepath.Clean(target)
	for _, r := range roots {
		if target == filepath.Clean(r) {
			return target, true
		}
	}
	if strings.HasPrefix(target+string(filepath.Separator), filepath.Clean(project)+string(filepath.Separator)) {
		return target, true
	}
	return "", false
}

func emitEnvelope(stdout io.Writer, env createEnvelopeOut, code int) int {
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		// The fallback must still be valid JSON: the envelope is a machine contract.
		msg, _ := json.Marshal("internal: envelope marshal: " + err.Error())
		fmt.Fprintf(stdout, `{"ok":false,"errors":[%s]}`+"\n", msg)
		return 1
	}
	fmt.Fprintln(stdout, string(raw))
	return code
}

// readArg reads a file path, or stdin when the arg is "-".
func readArg(arg string, stdin io.Reader) ([]byte, error) {
	if arg == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("stdin unavailable")
		}
		return io.ReadAll(stdin)
	}
	return os.ReadFile(arg)
}

func stdinCount(args ...string) int {
	n := 0
	for _, a := range args {
		if a == "-" {
			n++
		}
	}
	return n
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// relTo renders path project-relative when possible (for stable envelopes).
func relTo(project, path string) string {
	if rel, err := filepath.Rel(project, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}
