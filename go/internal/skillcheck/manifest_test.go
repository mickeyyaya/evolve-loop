package skillcheck

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func wellFormedSkill(name string) string {
	return "---\nname: " + name + "\ndescription: The " + name + " skill.\n---\n\n# " + name + "\n\nbody\n"
}

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

func writeRawManifest(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
}

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

func TestManifestProblems_OrphanSkillDirNotInManifest(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	writeSkillDir(t, root, "beta", wellFormedSkill("beta"))
	writeManifest(t, root, []string{"alpha"}, nil)

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "beta", "Unknown skill") {
		t.Fatalf("orphan skill dir 'beta' must be flagged as unregistered; got %v", problems)
	}
}

func TestManifestProblems_ListedSkillMissingDir(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "alpha", wellFormedSkill("alpha"))
	writeManifest(t, root, []string{"alpha", "ghost"}, nil)

	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("ManifestProblems: %v", err)
	}
	if !hasProblemContaining(problems, "ghost", "install breaks") {
		t.Fatalf("listed-but-missing skill 'ghost' must be flagged; got %v", problems)
	}
}

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

func TestSkillEntryName(t *testing.T) {
	cases := map[string]string{
		"./skills/loop/":       "loop",
		"./skills/loop":        "loop",
		"skills/loop/":         "loop",
		"./skills/plan-review": "plan-review",
		"./skills/..":          "",
		"./skills/.":           "",
		"./skills/a/b":         "",
		"./skills/":            "",
		"./agents/x.md":        "",
	}
	for entry, want := range cases {
		if got := skillEntryName(entry); got != want {
			t.Errorf("skillEntryName(%q) = %q, want %q", entry, got, want)
		}
	}
}

func TestCheck_SurfacesManifestProblems(t *testing.T) {
	drift, err := Check(manifestDefectTree(t))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasProblemContaining(drift, "ghostskill") {
		t.Fatalf("Check must surface the listed-but-missing skill via ManifestProblems; got %v", drift)
	}
}

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
