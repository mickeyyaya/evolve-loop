package skillcheck

// manifest_test.go guards the registry-membership invariant that the phase-facts,
// command-stub, and codex projections never check: .claude-plugin/plugin.json
// (the list Claude Code's loader actually reads) must be a bijection with the
// skills/ dirs on disk. A well-formed skill dir that plugin.json does not list is
// invisible to the loader — it surfaces to a user as "Unknown skill" (the failure
// that motivated this guard). The reverse (a registry entry with no backing dir),
// a malformed entry, a duplicate, and a dangling agents[] path are caught too.
//
// Strong-test discipline: TestManifestProblems_CleanRepoNoProblems is the CI
// regression gate on the LIVE tree; every other case injects exactly one defect
// into a minimal synthetic tree and asserts a DEFECT-SPECIFIC message (not just
// the skill name) so each test bites only on the code path it names.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- fixtures ---------------------------------------------------------------

// manifestTree builds an empty plugin skeleton under a temp dir and returns its
// root. Callers layer skills/agents/manifest on, then mutate to inject one defect.
func manifestTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{".claude-plugin", "skills", "commands", "agents"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	return root
}

// writeSkillDir writes skills/<name>/SKILL.md verbatim.
func writeSkillDir(t *testing.T, root, name, skillMD string) {
	t.Helper()
	dir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skills/%s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatalf("write skills/%s/SKILL.md: %v", name, err)
	}
}

// wellFormedSkill is a valid SKILL.md whose frontmatter name matches its dir.
func wellFormedSkill(name string) string {
	return "---\nname: " + name + "\ndescription: The " + name + " skill.\n---\n\n# " + name + "\n\nbody\n"
}

// writeManifest writes .claude-plugin/plugin.json listing skill dirs (bare names,
// wrapped as "./skills/<name>/") and agent paths (verbatim).
func writeManifest(t *testing.T, root string, skills, agents []string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(`{"name":"evo","version":"0.0.0","skills":[`)
	for i, s := range skills {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"./skills/` + s + `/"`)
	}
	b.WriteString(`],"agents":[`)
	for i, a := range agents {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"` + a + `"`)
	}
	b.WriteString(`]}`)
	writeRawManifest(t, root, b.String())
}

// writeRawManifest writes verbatim bytes to plugin.json (malformed-JSON /
// malformed-entry cases the typed writeManifest cannot express).
func writeRawManifest(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
}

// hasProblemContaining reports whether any problem string contains every substr.
func hasProblemContaining(problems []string, substrs ...string) bool {
	for _, p := range problems {
		all := true
		for _, s := range substrs {
			if !strings.Contains(p, s) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// manifestDefectTree copies the live repo's plugin surfaces into a temp dir and
// registers a skill the tree does not provide ("ghostskill"). Only the manifest
// membership surface can flag it (no disk dir for the command/name/facts surfaces
// to see), so it isolates the Run/Check wiring. Reverting the wiring drops
// "ghostskill" from the result → red.
func manifestDefectTree(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	tmp := t.TempDir()
	copyFile(t, filepath.Join(root, "docs", "architecture", "phase-registry.json"),
		filepath.Join(tmp, "docs", "architecture", "phase-registry.json"))
	for _, dir := range []string{"skills", "agents", "commands", ".claude-plugin", filepath.Join(".evolve", "profiles")} {
		copyTree(t, filepath.Join(root, dir), filepath.Join(tmp, dir))
	}
	mfPath := filepath.Join(tmp, ".claude-plugin", "plugin.json")
	raw, err := os.ReadFile(mfPath)
	if err != nil {
		t.Fatalf("read copied plugin.json: %v", err)
	}
	mutated := strings.Replace(string(raw), `"skills": [`, `"skills": [`+"\n    \"./skills/ghostskill/\",", 1)
	if mutated == string(raw) {
		t.Fatal("fixture mutation did not apply — `\"skills\": [` anchor not found in plugin.json")
	}
	if err := os.WriteFile(mfPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write mutated plugin.json: %v", err)
	}
	return tmp
}

// --- the CI regression gate (live tree) -------------------------------------

// TestManifestProblems_CleanRepoNoProblems: the live repo's plugin.json is a
// bijection with disk. This is the regression gate — a future skill added to disk
// but not registered (or a registry entry whose dir was deleted) turns it red.
func TestManifestProblems_CleanRepoNoProblems(t *testing.T) {
	problems, err := ManifestProblems(repoRoot(t))
	if err != nil {
		t.Fatalf("ManifestProblems on live repo: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("live repo manifest must be consistent; got %d problem(s):\n%s",
			len(problems), strings.Join(problems, "\n"))
	}
}

// TestManifestProblems_HealthyMinimalTree: no false positives on a hand-built
// consistent tree (guards the gate against crying wolf).
func TestManifestProblems_HealthyMinimalTree(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	if err := os.WriteFile(filepath.Join(root, "agents", "a.md"), []byte("agent"), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}
	writeManifest(t, root, []string{"alpha"}, []string{"./agents/a.md"})

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("healthy minimal tree must have no problems; got %v", problems)
	}
}

// TestManifestProblems_AbsentManifestIsSkipped: a tree with no plugin.json at all
// is "not a plugin tree" — nothing to validate (present-but-broken is a problem;
// absent is a skip). Keeps partial fixtures clean.
func TestManifestProblems_AbsentManifestIsSkipped(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("absent manifest must be skipped, not flagged; got %v", problems)
	}
}

// --- mutation cases: each injects one defect, each asserts its own message ----

// TestManifestProblems_OrphanSkillDirNotInManifest: a well-formed skill dir the
// manifest never lists is invisible to the loader ("Unknown skill" — the exact
// fable-class regression). MUST be flagged.
func TestManifestProblems_OrphanSkillDirNotInManifest(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	writeSkillDir(t, root, "beta", wellFormedSkill("beta")) // on disk, NOT listed
	writeManifest(t, root, []string{"alpha"}, nil)

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "beta", "Unknown skill") {
		t.Fatalf("orphan skill dir 'beta' must be flagged as unregistered; got %v", problems)
	}
}

// TestManifestProblems_ListedSkillMissingDir: a registry entry whose dir/SKILL.md
// does not exist breaks the install. MUST be flagged.
func TestManifestProblems_ListedSkillMissingDir(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	writeManifest(t, root, []string{"alpha", "ghost"}, nil) // 'ghost' has no dir

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "ghost", "install breaks") {
		t.Fatalf("listed-but-missing skill 'ghost' must be flagged; got %v", problems)
	}
}

// TestManifestProblems_MalformedSkillEntry: a skills[] entry that is not a
// ./skills/<name>/ path is malformed and must be flagged as such.
func TestManifestProblems_MalformedSkillEntry(t *testing.T) {
	root := manifestTree(t)
	writeRawManifest(t, root, `{"name":"evo","skills":["./agents/not-a-skill.md"],"agents":[]}`)

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "not a ./skills") {
		t.Fatalf("malformed skills[] entry must be flagged; got %v", problems)
	}
}

// TestManifestProblems_DotDotSkillEntryRejected: a "./skills/.." entry must be
// rejected as malformed, NOT silently resolved to projectRoot/SKILL.md. A
// well-formed SKILL.md is planted one level above skills/ to prove the traversal
// does not produce a false pass.
func TestManifestProblems_DotDotSkillEntryRejected(t *testing.T) {
	root := manifestTree(t)
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(wellFormedSkill("root")), 0o644); err != nil {
		t.Fatalf("plant root SKILL.md: %v", err)
	}
	writeRawManifest(t, root, `{"name":"evo","skills":["./skills/.."],"agents":[]}`)

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "not a ./skills") {
		t.Fatalf("`./skills/..` must be rejected as malformed (no path-traversal false pass); got %v", problems)
	}
}

// TestManifestProblems_DuplicateSkillEntry: the same skill listed twice.
func TestManifestProblems_DuplicateSkillEntry(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	writeManifest(t, root, []string{"alpha", "alpha"}, nil)

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "alpha", "more than once") {
		t.Fatalf("duplicate skills[] entry must be flagged; got %v", problems)
	}
}

// TestManifestProblems_AgentFileMissing: an agents[] entry with no backing file.
func TestManifestProblems_AgentFileMissing(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	writeManifest(t, root, []string{"alpha"}, []string{"./agents/ghost.md"})

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "ghost.md", "agents[]") {
		t.Fatalf("dangling agents[] path must be flagged; got %v", problems)
	}
}

// TestManifestProblems_MalformedManifestJSON: a corrupt plugin.json must fail
// closed with a JSON-specific message, not slip through to another surface.
func TestManifestProblems_MalformedManifestJSON(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	writeRawManifest(t, root, "{ this is not valid json ")

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "not valid JSON") {
		t.Fatalf("malformed plugin.json must be flagged with a JSON-specific message; got %v", problems)
	}
}

// TestSkillEntryName pins the normalization contract directly, incl. the
// path-traversal rejection ("." / ".." / separators → "" = malformed).
func TestSkillEntryName(t *testing.T) {
	cases := map[string]string{
		"./skills/loop/":       "loop",
		"./skills/loop":        "loop",
		"skills/loop/":         "loop",
		"./skills/plan-review": "plan-review",
		"./skills/..":          "", // traversal — must NOT become ".."
		"./skills/.":           "",
		"./skills/a/b":         "", // nested — not a single segment
		"./skills/":            "",
		"./agents/x.md":        "", // not under skills/
	}
	for entry, want := range cases {
		if got := skillEntryName(entry); got != want {
			t.Errorf("skillEntryName(%q) = %q, want %q", entry, got, want)
		}
	}
}

// --- wiring proofs: both Run (CLI/CI) and Check (audit) surface the problems ---

// TestCheck_SurfacesManifestProblems: the in-process audit gate includes manifest
// bijection problems alongside phase-facts/name drift.
func TestCheck_SurfacesManifestProblems(t *testing.T) {
	drift, err := Check(manifestDefectTree(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasProblemContaining(drift, "ghostskill") {
		t.Fatalf("Check must surface the listed-but-missing skill via ManifestProblems; got %v", drift)
	}
}

// TestRun_SurfacesManifestProblems: the `evolve skills check` CLI path (Run,
// write=false) — the one CI's TestSkills_NoDrift exercises — must also fail
// (exit 2) and name the broken skill, so a bijection break cannot pass locally.
func TestRun_SurfacesManifestProblems(t *testing.T) {
	tmp := manifestDefectTree(t)
	var out, errBuf bytes.Buffer
	rc := Run(tmp, false, &out, &errBuf)
	if rc != 2 {
		t.Fatalf("Run (check mode) with a broken manifest: rc=%d, want 2\nstderr:\n%s", rc, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "ghostskill") {
		t.Fatalf("Run must report the listed-but-missing skill; stderr:\n%s", errBuf.String())
	}
}
