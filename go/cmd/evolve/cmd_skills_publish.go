// cmd_skills_publish.go implements `evolve skills publish` — the cross-CLI
// half of the skill projection story (ADR-0041, extends ADR-0040). Canonical
// skills (skills/<name>/SKILL.md, enumerated from .claude-plugin/plugin.json)
// are projected into the surfaces of three foreign LLM CLIs:
//
//	codex   $CODEX_HOME/skills/evolve-<name>/SKILL.md — flat namespace, so the
//	        frontmatter name is rewritten with an evolve- prefix
//	agy     a native plugin staging dir (plugin.json + skills/<name>/) that
//	        `agy plugin validate|install` consumes; the plugin name supplies
//	        the namespace, so skill names stay unprefixed
//	ollama  Modelfiles embedding the skill body as a SYSTEM prompt (ollama has
//	        no skill system); read-only-tier subset only, mirroring
//	        driver_ollamatmux.go's write-phase rejection
//
// Safety invariant: bare `publish` is stage-only — it writes gitignored
// mirrors under .evolve/publish/<target>/ and runs the read-only
// `agy plugin validate`; it never mutates the user environment. `--install`
// performs the mutating steps. Every projected artifact carries the
// EVOLVE-PUBLISH:projection provenance marker; prune deletes only
// evolve-*-prefixed artifacts carrying that marker, never user-authored files.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/mickeyyaya/evolveloop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolveloop/go/internal/prompts"
)

//go:embed templates/modelfile.tmpl
var modelfileTmpl string

// publishProvenanceSentinel marks every projected artifact. Prune refuses to
// delete anything that does not carry it.
const publishProvenanceSentinel = "EVOLVE-PUBLISH:projection"

// publishPluginName is the plugin/namespace identity used for the agy target.
const publishPluginName = "evolve-loop"

// Exec seams — overridable in tests so no real agy/ollama binaries run.
var (
	publishLookPath = exec.LookPath
	publishRunCmd   = func(stdout, stderr io.Writer, dir, name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	}
)

// ollamaCompatible is the read-only-tier subset projected to ollama. Plain
// `ollama run` has no agentic tool use, so write/orchestration skills (tdd,
// build, loop, ship, commit, release, publish, phase-create, setup, refactor)
// are excluded — the same rule driver_ollamatmux.go enforces by rejecting
// write phases. Reasoning/review skills are text-in/text-out and fit.
var ollamaCompatible = map[string]bool{
	"scout":                  true,
	"plan-review":            true,
	"audit":                  true,
	"retro":                  true,
	"intent":                 true,
	"evaluator":              true,
	"inspirer":               true,
	"adversarial-testing":    true,
	"golang-test-review":     true,
	"code-review-simplify":   true,
	"security-review-scored": true,
	"verify-release":         true,
}

// publishConfig captures the parsed `skills publish` flags.
type publishConfig struct {
	Targets    []string // subset of {codex, agy, ollama}
	DryRun     bool
	Install    bool
	Check      bool
	Prune      bool
	OllamaBase string
	CodexHome  string
}

// canonicalSkill is one source skill enumerated from plugin.json.
type canonicalSkill struct {
	Name        string // dir name == frontmatter name (ADR-0040 rule 3)
	Raw         string // full SKILL.md contents
	Body        string // markdown after frontmatter
	Description string
}

// parsePublishFlags parses args into a publishConfig. Returns ok=false (after
// reporting to stderr) on unknown flags or targets — caller exits 10.
func parsePublishFlags(args []string, stderr io.Writer) (publishConfig, bool) {
	cfg := publishConfig{Targets: []string{"codex", "agy", "ollama"}, Prune: true}
	for i := 0; i < len(args); i++ {
		flag, val := args[i], ""
		if eq := strings.Index(flag, "="); eq > 0 {
			flag, val = flag[:eq], flag[eq+1:]
		}
		needVal := func() bool {
			if val != "" {
				return true
			}
			if i+1 < len(args) {
				i++
				val = args[i]
				return true
			}
			fmt.Fprintf(stderr, "flag %s requires a value\n", flag)
			return false
		}
		switch flag {
		case "--target":
			if !needVal() {
				return cfg, false
			}
			cfg.Targets = strings.Split(val, ",")
		case "--dry-run":
			cfg.DryRun = true
		case "--install":
			cfg.Install = true
		case "--check":
			cfg.Check = true
		case "--no-prune":
			cfg.Prune = false
		case "--ollama-base":
			if !needVal() {
				return cfg, false
			}
			cfg.OllamaBase = val
		case "--codex-home":
			if !needVal() {
				return cfg, false
			}
			cfg.CodexHome = val
		default:
			fmt.Fprintf(stderr, "unknown flag %q (see `evolve skills publish` usage)\n", flag)
			return cfg, false
		}
	}
	for _, t := range cfg.Targets {
		if t != "codex" && t != "agy" && t != "ollama" {
			fmt.Fprintf(stderr, "unknown target %q (want codex|agy|ollama)\n", t)
			return cfg, false
		}
	}
	if cfg.Check && cfg.DryRun {
		fmt.Fprintln(stderr, "note: --check takes precedence over --dry-run (check never writes)")
	}
	if cfg.OllamaBase == "" {
		// Default mirrors driver_ollamatmux.go's model default; no ollama
		// profile exists in .evolve/profiles/ to read it from (ADR-0041).
		cfg.OllamaBase = "llama3.1:8b"
	}
	if cfg.CodexHome == "" {
		cfg.CodexHome = os.Getenv("CODEX_HOME")
	}
	if cfg.CodexHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cfg.CodexHome = filepath.Join(home, ".codex")
		}
	}
	return cfg, true
}

// runSkillsPublish stages (and optionally installs) every requested target.
// Targets are independent: one failure does not stop the others, but any
// failure yields exit 1. Check mode never writes and yields exit 2 on drift.
func runSkillsPublish(project string, cfg publishConfig, stdout, stderr io.Writer) int {
	skills, err := enumerateCanonicalSkills(project)
	if err != nil {
		fmt.Fprintf(stderr, "enumerate skills: %v\n", err)
		return 1
	}
	drift, failed := false, false
	for _, target := range cfg.Targets {
		files, err := renderTarget(target, skills, cfg, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "%s: render: %v\n", target, err)
			failed = true
			continue
		}
		staging := filepath.Join(project, ".evolve", "publish", target)
		switch {
		case cfg.Check:
			if diffStaging(staging, files, target, stderr) {
				drift = true
			}
		case cfg.DryRun:
			printPublishPlan(target, staging, files, cfg, stdout)
		default:
			if err := applyTarget(project, target, staging, files, skills, cfg, stdout, stderr); err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", target, err)
				failed = true
			}
		}
	}
	if failed {
		// Render failures are always fatal — including in check mode, where a
		// silent exit-0 would be a CI false-green on the exact class of error
		// this command exists to catch.
		return 1
	}
	if cfg.Check {
		if drift {
			return 2
		}
		fmt.Fprintln(stdout, "[skills] publish check OK — staged projections in sync")
	}
	return 0
}

// renderTarget produces the deterministic projection for one target: a map of
// path-relative-to-staging-root → content. Pure except for skip logging.
func renderTarget(target string, skills []canonicalSkill, cfg publishConfig, out io.Writer) (map[string][]byte, error) {
	switch target {
	case "codex":
		return renderCodex(skills)
	case "agy":
		return renderAgy(skills), nil
	case "ollama":
		return renderOllama(skills, cfg.OllamaBase, out)
	}
	return nil, fmt.Errorf("unknown target %q", target)
}

// applyTarget writes the staging mirror and, with --install, performs the
// target's mutating step (codex copy+prune, agy install, ollama create/rm).
func applyTarget(project, target, staging string, files map[string][]byte, skills []canonicalSkill, cfg publishConfig, stdout, stderr io.Writer) error {
	// Ollama's stale-model record lives in the staging manifest; read it
	// before the rewrite below destroys it.
	var oldModels []string
	if target == "ollama" {
		oldModels = readOllamaManifest(staging)
	}
	if err := writeStaging(staging, files); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "[skills] %s: staged %d files under %s\n", target, len(files), relToProject(project, staging))
	switch target {
	case "codex":
		return installCodex(files, skills, cfg, stdout)
	case "agy":
		return installAgy(project, staging, cfg, stdout, stderr)
	case "ollama":
		return installOllama(staging, oldModels, cfg, stdout, stderr)
	}
	return nil
}

// enumerateCanonicalSkills loads every skill listed in plugin.json (the same
// manifest `evolve skills check` trusts) in sorted order.
func enumerateCanonicalSkills(project string) ([]canonicalSkill, error) {
	raw, err := os.ReadFile(filepath.Join(project, ".claude-plugin", "plugin.json"))
	if err != nil {
		return nil, fmt.Errorf("read plugin.json: %w", err)
	}
	var manifest struct {
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("parse plugin.json: %w", err)
	}
	var skills []canonicalSkill
	for _, entry := range manifest.Skills {
		name := filepath.Base(strings.TrimSuffix(strings.TrimPrefix(entry, "./"), "/"))
		path := filepath.Join(project, "skills", name, "SKILL.md")
		doc, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		fm, body, err := prompts.ParseFrontmatter(string(doc))
		if err != nil {
			return nil, fmt.Errorf("skills/%s/SKILL.md: frontmatter: %w", name, err)
		}
		desc, _ := fm["description"].(string)
		skills = append(skills, canonicalSkill{Name: name, Raw: string(doc), Body: body, Description: desc})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// renderCodex projects skills for the flat codex namespace: dir and
// frontmatter name both become evolve-<name>.
func renderCodex(skills []canonicalSkill) (map[string][]byte, error) {
	files := make(map[string][]byte, len(skills))
	for _, s := range skills {
		doc, err := rewriteFrontmatterName(s.Raw, "evolve-"+s.Name)
		if err != nil {
			return nil, fmt.Errorf("skills/%s: %w", s.Name, err)
		}
		doc = injectProvenance(doc, provenanceHeader("skills/"+s.Name+"/SKILL.md", "codex"))
		files[filepath.Join("evolve-"+s.Name, "SKILL.md")] = []byte(doc)
	}
	return files, nil
}

// renderAgy projects skills into agy's native plugin layout. Skill names stay
// unprefixed — the evolve-loop plugin name supplies the namespace.
func renderAgy(skills []canonicalSkill) map[string][]byte {
	manifest, _ := json.MarshalIndent(struct {
		Name string `json:"name"`
	}{Name: publishPluginName}, "", "  ") // const input — cannot fail
	files := map[string][]byte{
		filepath.Join(publishPluginName, "plugin.json"): append(manifest, '\n'),
	}
	for _, s := range skills {
		doc := injectProvenance(s.Raw, provenanceHeader("skills/"+s.Name+"/SKILL.md", "agy"))
		files[filepath.Join(publishPluginName, "skills", s.Name, "SKILL.md")] = []byte(doc)
	}
	return files
}

// renderOllama projects the read-only-compatible subset as Modelfiles plus a
// manifest.json recording the model names (the only provenance `ollama list`
// can't give us back).
func renderOllama(skills []canonicalSkill, base string, out io.Writer) (map[string][]byte, error) {
	tmpl, err := template.New("modelfile").Parse(modelfileTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse embedded modelfile template: %w", err)
	}
	files := map[string][]byte{}
	var models []string
	for _, s := range skills {
		if !ollamaCompatible[s.Name] {
			fmt.Fprintf(out, "[skills] ollama: skip %s (write/orchestration skill; ollama is read-only tier)\n", s.Name)
			continue
		}
		var buf bytes.Buffer
		err := tmpl.Execute(&buf, struct {
			Sentinel, CanonicalRel, Base, Description, Body string
		}{
			Sentinel:     publishProvenanceSentinel,
			CanonicalRel: "skills/" + s.Name + "/SKILL.md",
			Base:         base,
			Description:  sanitizeModelfileSystem(s.Description),
			Body:         sanitizeModelfileSystem(strings.TrimSpace(s.Body)),
		})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", s.Name, err)
		}
		model := "evolve-" + s.Name
		files[filepath.Join(model, "Modelfile")] = buf.Bytes()
		models = append(models, model)
	}
	sort.Strings(models)
	manifest, err := json.MarshalIndent(struct {
		Models []string `json:"models"`
	}{Models: models}, "", "  ")
	if err != nil {
		return nil, err
	}
	files["manifest.json"] = append(manifest, '\n')
	return files, nil
}

// rewriteFrontmatterName replaces the single top-level `name:` line inside the
// frontmatter block with a clean `name: <newName>` line, preserving every
// other byte (and the line's CRLF ending, if any). Lines outside the
// frontmatter are never touched.
func rewriteFrontmatterName(raw, newName string) (string, error) {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return "", fmt.Errorf("no frontmatter block")
	}
	for i := 1; i < len(lines); i++ {
		l := strings.TrimRight(lines[i], "\r")
		if l == "---" {
			break
		}
		if strings.HasPrefix(l, "name:") {
			cr := ""
			if strings.HasSuffix(lines[i], "\r") {
				cr = "\r"
			}
			lines[i] = "name: " + newName + cr
			return strings.Join(lines, "\n"), nil
		}
	}
	return "", fmt.Errorf("frontmatter has no name: line")
}

// provenanceHeader is the marker comment injected into every projected .md.
func provenanceHeader(canonicalRel, target string) string {
	return fmt.Sprintf("<!-- %s — generated from %s by `evolve skills publish --target %s`; do not edit, edit the canonical skill and re-run. -->",
		publishProvenanceSentinel, canonicalRel, target)
}

// injectProvenance inserts header as the first body line after the
// frontmatter block (falling back to prepending when no frontmatter exists,
// which canonical skills never hit).
func injectProvenance(raw, header string) string {
	for _, fence := range []string{"---\n", "---\r\n"} {
		if !strings.HasPrefix(raw, fence) {
			continue
		}
		// Search for the closing fence: "\n---" (CRLF variant: "\n---\r").
		closingPrefix := "\n" + fence[:len(fence)-1]
		if at := strings.Index(raw[len(fence):], closingPrefix); at >= 0 {
			// End is right after the closing "---" (or "---\r") sequence.
			end := len(fence) + at + len(closingPrefix)
			return raw[:end] + "\n" + header + "\n" + raw[end:]
		}
	}
	return header + "\n" + raw
}

// sanitizeModelfileSystem keeps a skill body from terminating the SYSTEM
// """ block early. Ollama defines no escape, so the fence is downgraded.
func sanitizeModelfileSystem(s string) string {
	return strings.ReplaceAll(s, `"""`, "'''")
}

// writeStaging rebuilds a staging root from scratch — staging dirs are wholly
// owned projections under .evolve/publish/, so a full rewrite is safe and
// guarantees no stale files linger.
func writeStaging(root string, files map[string][]byte) error {
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clear staging %s: %w", root, err)
	}
	for _, rel := range sortedKeys(files) {
		if err := atomicwrite.Bytes(filepath.Join(root, rel), files[rel]); err != nil {
			return fmt.Errorf("stage %s: %w", rel, err)
		}
	}
	return nil
}

// diffStaging compares a fresh render against the staging dir: any missing,
// changed, or extra file is drift. Never writes.
func diffStaging(root string, files map[string][]byte, target string, stderr io.Writer) bool {
	drift := false
	for _, rel := range sortedKeys(files) {
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			fmt.Fprintf(stderr, "DRIFT: %s: %s missing from staging (run `evolve skills publish --target %s`)\n", target, rel, target)
			drift = true
			continue
		}
		if !bytes.Equal(got, files[rel]) {
			fmt.Fprintf(stderr, "DRIFT: %s: %s is stale (run `evolve skills publish --target %s`)\n", target, rel, target)
			drift = true
		}
	}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if _, ok := files[rel]; !ok {
			fmt.Fprintf(stderr, "DRIFT: %s: %s no longer rendered (stale staging file)\n", target, rel)
			drift = true
		}
		return nil
	})
	return drift
}

// printPublishPlan reports what a real run would write and execute.
func printPublishPlan(target, staging string, files map[string][]byte, cfg publishConfig, out io.Writer) {
	fmt.Fprintf(out, "[skills] %s (dry-run): would stage %d files under %s\n", target, len(files), staging)
	for _, rel := range sortedKeys(files) {
		fmt.Fprintf(out, "  %s\n", rel)
	}
	if !cfg.Install {
		return
	}
	switch target {
	case "codex":
		fmt.Fprintf(out, "  → would copy into %s\n", filepath.Join(cfg.CodexHome, "skills"))
	case "agy":
		fmt.Fprintf(out, "  → would run: agy plugin install %s\n", filepath.Join(staging, publishPluginName))
	case "ollama":
		fmt.Fprintf(out, "  → would run: ollama create evolve-<skill> -f <Modelfile> per model\n")
	}
}

// installCodex copies the rendered projection into $CODEX_HOME/skills and
// prunes stale sentinel-marked evolve-* dirs. Stage-only without --install.
func installCodex(files map[string][]byte, skills []canonicalSkill, cfg publishConfig, stdout io.Writer) error {
	skillsDir := filepath.Join(cfg.CodexHome, "skills")
	if !cfg.Install {
		fmt.Fprintf(stdout, "[skills] codex: staged only; re-run with --install to copy into %s\n", skillsDir)
		return nil
	}
	for _, rel := range sortedKeys(files) {
		if err := atomicwrite.Bytes(filepath.Join(skillsDir, rel), files[rel]); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "[skills] codex: installed %d skills into %s\n", len(files), skillsDir)
	if !cfg.Prune {
		return nil
	}
	keep := make(map[string]bool, len(skills))
	for _, s := range skills {
		keep["evolve-"+s.Name] = true
	}
	return pruneStaleProvenanced(skillsDir, keep, stdout)
}

// installAgy validates the staged plugin (always, read-only) and installs it
// with --install. A missing agy binary downgrades validation to a note but
// fails loudly when an install was requested.
func installAgy(project, staging string, cfg publishConfig, stdout, stderr io.Writer) error {
	pluginDir := filepath.Join(staging, publishPluginName)
	if _, err := publishLookPath("agy"); err != nil {
		if cfg.Install {
			return fmt.Errorf("--install requested but agy binary not on PATH")
		}
		fmt.Fprintf(stdout, "[skills] agy: binary not on PATH; staged only (validate skipped)\n")
		return nil
	}
	if err := publishRunCmd(stdout, stderr, project, "agy", "plugin", "validate", pluginDir); err != nil {
		return fmt.Errorf("agy plugin validate: %w", err)
	}
	if !cfg.Install {
		fmt.Fprintf(stdout, "[skills] agy: validated; re-run with --install to run `agy plugin install %s`\n", pluginDir)
		return nil
	}
	if err := publishRunCmd(stdout, stderr, project, "agy", "plugin", "install", pluginDir); err != nil {
		return fmt.Errorf("agy plugin install: %w", err)
	}
	fmt.Fprintf(stdout, "[skills] agy: installed plugin %s\n", publishPluginName)
	return nil
}

// installOllama creates each staged model and, with prune on, removes models
// recorded in the previous manifest that this render no longer produces.
func installOllama(staging string, oldModels []string, cfg publishConfig, stdout, stderr io.Writer) error {
	models := readOllamaManifest(staging)
	if !cfg.Install {
		fmt.Fprintf(stdout, "[skills] ollama: staged only; re-run with --install to create models: %s\n", strings.Join(models, ", "))
		return nil
	}
	if _, err := publishLookPath("ollama"); err != nil {
		return fmt.Errorf("--install requested but ollama binary not on PATH")
	}
	current := make(map[string]bool, len(models))
	for _, m := range models {
		current[m] = true
		modelfile := filepath.Join(staging, m, "Modelfile")
		if err := publishRunCmd(stdout, stderr, staging, "ollama", "create", m, "-f", modelfile); err != nil {
			return fmt.Errorf("ollama create %s: %w", m, err)
		}
	}
	fmt.Fprintf(stdout, "[skills] ollama: created %d models\n", len(models))
	if !cfg.Prune {
		return nil
	}
	for _, m := range oldModels {
		if current[m] || !strings.HasPrefix(m, "evolve-") {
			continue
		}
		if err := publishRunCmd(stdout, stderr, staging, "ollama", "rm", m); err != nil {
			return fmt.Errorf("ollama rm %s: %w", m, err)
		}
		fmt.Fprintf(stdout, "[skills] ollama: pruned stale model %s\n", m)
	}
	return nil
}

// readOllamaManifest returns the model names from a staging manifest, or nil
// when none exists (first publish, or staging cleared).
func readOllamaManifest(staging string) []string {
	raw, err := os.ReadFile(filepath.Join(staging, "manifest.json"))
	if err != nil {
		return nil
	}
	var m struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m.Models
}

// pruneStaleProvenanced removes evolve-*-prefixed skill dirs that carry the
// provenance sentinel but are no longer in keep. Anything without the marker
// (user-authored) or without the prefix is never touched.
func pruneStaleProvenanced(dir string, keep map[string]bool, out io.Writer) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "evolve-") || keep[e.Name()] {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil || !strings.Contains(string(raw), publishProvenanceSentinel) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return fmt.Errorf("prune %s: %w", e.Name(), err)
		}
		fmt.Fprintf(out, "[skills] pruned stale projection %s\n", filepath.Join(dir, e.Name()))
	}
	return nil
}

// sortedKeys returns map keys in stable order for deterministic output.
func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// relToProject renders path relative to project for log readability.
func relToProject(project, path string) string {
	if rel, err := filepath.Rel(project, path); err == nil {
		return rel
	}
	return path
}
