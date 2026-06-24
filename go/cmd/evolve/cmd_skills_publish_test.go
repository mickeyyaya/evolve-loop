// cmd_skills_publish_test.go covers `evolve skills publish` (ADR-0041): the
// cross-CLI projection of canonical skills/ into Codex, agy, and Ollama
// surfaces. All tests are hermetic — temp project trees, a temp CODEX_HOME,
// and seam-recorded exec calls (no real agy/ollama runs).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// publishSkillDoc builds a minimal canonical SKILL.md.
func publishSkillDoc(name, desc, body string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n" + body
}

// publishTestProject writes a temp repo with .claude-plugin/plugin.json and
// the given skills (name → SKILL.md content), returning its root.
func publishTestProject(t *testing.T, skills map[string]string) string {
	t.Helper()
	root := t.TempDir()
	var list []string
	for name := range skills {
		list = append(list, "./skills/"+name+"/")
	}
	sort.Strings(list)
	manifest, err := json.Marshal(map[string]any{"name": "evolve-loop", "version": "0.0.0-test", "skills": list})
	if err != nil {
		t.Fatalf("marshal plugin.json: %v", err)
	}
	writeFileForPublishTest(t, filepath.Join(root, ".claude-plugin", "plugin.json"), string(manifest))
	for name, content := range skills {
		writeFileForPublishTest(t, filepath.Join(root, "skills", name, "SKILL.md"), content)
	}
	return root
}

func writeFileForPublishTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// stubPublishExec replaces the exec seams for the test's lifetime, recording
// every invocation. haveBinaries controls LookPath success.
type execCall struct {
	Name string
	Args []string
}

func stubPublishExec(t *testing.T, haveBinaries bool) *[]execCall {
	t.Helper()
	var calls []execCall
	origLook, origRun := publishLookPath, publishRunCmd
	publishLookPath = func(name string) (string, error) {
		if haveBinaries {
			return "/stub/bin/" + name, nil
		}
		return "", fmt.Errorf("stub: %s not on PATH", name)
	}
	publishRunCmd = func(stdout, stderr io.Writer, dir, name string, args ...string) error {
		calls = append(calls, execCall{Name: name, Args: args})
		return nil
	}
	t.Cleanup(func() { publishLookPath, publishRunCmd = origLook, origRun })
	return &calls
}

func TestRewriteFrontmatterName(t *testing.T) {
	cases := []struct {
		label   string
		raw     string
		want    string // "" → expect error
		newName string
	}{
		{
			label:   "simple",
			raw:     "---\nname: build\ndescription: d\n---\n\nbody\n",
			newName: "evolve-build",
			want:    "---\nname: evolve-build\ndescription: d\n---\n\nbody\n",
		},
		{
			label:   "quoted name",
			raw:     "---\nname: \"build\"\ndescription: d\n---\nbody\n",
			newName: "evolve-build",
			want:    "---\nname: evolve-build\ndescription: d\n---\nbody\n",
		},
		{
			label:   "trailing comment collapses to clean line",
			raw:     "---\nname: build # legacy\n---\nbody\n",
			newName: "evolve-build",
			want:    "---\nname: evolve-build\n---\nbody\n",
		},
		{
			label:   "missing name errors",
			raw:     "---\ndescription: d\n---\nbody\n",
			newName: "evolve-x",
			want:    "",
		},
		{
			label:   "no frontmatter errors",
			raw:     "just prose\n",
			newName: "evolve-x",
			want:    "",
		},
		{
			label:   "crlf preserved",
			raw:     "---\r\nname: build\r\n---\r\nbody\r\n",
			newName: "evolve-build",
			want:    "---\r\nname: evolve-build\r\n---\r\nbody\r\n",
		},
		{
			label:   "name-looking line in body untouched",
			raw:     "---\nname: build\n---\nname: decoy\n",
			newName: "evolve-build",
			want:    "---\nname: evolve-build\n---\nname: decoy\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, err := rewriteFrontmatterName(tc.raw, tc.newName)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("want error, got:\n%s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestProvenanceHeader_ContainsSentinelAndSource(t *testing.T) {
	h := provenanceHeader("skills/build/SKILL.md", "codex")
	if !strings.Contains(h, publishProvenanceSentinel) {
		t.Errorf("header missing sentinel: %s", h)
	}
	if !strings.Contains(h, "skills/build/SKILL.md") {
		t.Errorf("header missing canonical path: %s", h)
	}
	if !strings.Contains(h, "evolve skills publish") {
		t.Errorf("header missing regenerate command: %s", h)
	}
}

func TestInjectProvenance_AfterFrontmatter(t *testing.T) {
	raw := "---\nname: x\n---\n\nbody\n"
	out := injectProvenance(raw, "<!-- MARK -->")
	fm, body, err := prompts.ParseFrontmatter(out)
	if err != nil {
		t.Fatalf("injected doc unparseable: %v", err)
	}
	if fm["name"] != "x" {
		t.Errorf("frontmatter damaged: %v", fm)
	}
	if !strings.HasPrefix(strings.TrimLeft(body, "\n"), "<!-- MARK -->") {
		t.Errorf("marker not at body head:\n%s", out)
	}
}

func TestPublishCodex_StageOnlyDoesNotTouchCodexHome(t *testing.T) {
	stubPublishExec(t, false)
	project := publishTestProject(t, map[string]string{
		"build": publishSkillDoc("build", "builder skill", "Build things."),
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
	})
	codexHome := t.TempDir()
	cfg := publishConfig{Targets: []string{"codex"}, Prune: true, CodexHome: codexHome, OllamaBase: "test:base"}

	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}

	// Mirror staged with rewritten name + sentinel, body preserved.
	staged, err := os.ReadFile(filepath.Join(project, ".evolve", "publish", "codex", "evolve-build", "SKILL.md"))
	if err != nil {
		t.Fatalf("staged mirror missing: %v", err)
	}
	s := string(staged)
	if !strings.Contains(s, "name: evolve-build") {
		t.Errorf("name not rewritten:\n%s", s)
	}
	if !strings.Contains(s, publishProvenanceSentinel) {
		t.Errorf("sentinel missing:\n%s", s)
	}
	if !strings.Contains(s, "Build things.") {
		t.Errorf("body lost:\n%s", s)
	}

	// CODEX_HOME untouched without --install.
	if entries, _ := os.ReadDir(filepath.Join(codexHome, "skills")); len(entries) != 0 {
		t.Errorf("codex home mutated without --install: %v", entries)
	}
}

func TestPublishCodex_InstallIsIdempotentAndNameMatchesDir(t *testing.T) {
	stubPublishExec(t, false)
	project := publishTestProject(t, map[string]string{
		"build": publishSkillDoc("build", "builder skill", "Build things."),
		"audit": publishSkillDoc("audit", "audit skill", "Audit things."),
	})
	codexHome := t.TempDir()
	cfg := publishConfig{Targets: []string{"codex"}, Prune: true, Install: true, CodexHome: codexHome, OllamaBase: "test:base"}

	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("first run exit %d\nstderr:\n%s", code, errBuf.String())
	}
	first := readTreeForPublishTest(t, filepath.Join(codexHome, "skills"))
	if len(first) != 2 {
		t.Fatalf("want 2 installed skills, got %d: %v", len(first), first)
	}

	// ADR-0041 analogue of the ADR-0040 invariant: frontmatter name == dir name.
	for rel, content := range first {
		dir := filepath.Dir(rel) // evolve-<name>
		fm, _, err := prompts.ParseFrontmatter(content)
		if err != nil {
			t.Fatalf("%s: unparseable: %v", rel, err)
		}
		if fm["name"] != dir {
			t.Errorf("%s: frontmatter name %q != dir %q", rel, fm["name"], dir)
		}
		if !strings.HasPrefix(dir, "evolve-") {
			t.Errorf("%s: missing evolve- prefix", rel)
		}
	}

	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("second run exit %d\nstderr:\n%s", code, errBuf.String())
	}
	second := readTreeForPublishTest(t, filepath.Join(codexHome, "skills"))
	if len(second) != len(first) {
		t.Fatalf("re-run changed file count: %d → %d", len(first), len(second))
	}
	for rel, content := range first {
		if second[rel] != content {
			t.Errorf("%s: not byte-identical on re-run", rel)
		}
	}
}

func TestPublishCodex_PrunesOnlySentinelMarked(t *testing.T) {
	stubPublishExec(t, false)
	project := publishTestProject(t, map[string]string{
		"build": publishSkillDoc("build", "builder skill", "Build things."),
	})
	codexHome := t.TempDir()
	skillsDir := filepath.Join(codexHome, "skills")
	// Stale projection (carries sentinel) → must be pruned.
	writeFileForPublishTest(t, filepath.Join(skillsDir, "evolve-gone", "SKILL.md"),
		"---\nname: evolve-gone\n---\n<!-- "+publishProvenanceSentinel+" -->\nold\n")
	// User-authored skill with evolve- prefix but NO sentinel → preserved.
	writeFileForPublishTest(t, filepath.Join(skillsDir, "evolve-mine", "SKILL.md"),
		"---\nname: evolve-mine\n---\nhand-written\n")
	// Unrelated skill → untouched.
	writeFileForPublishTest(t, filepath.Join(skillsDir, "other-tool", "SKILL.md"),
		"---\nname: other-tool\n---\nx\n")

	cfg := publishConfig{Targets: []string{"codex"}, Prune: true, Install: true, CodexHome: codexHome, OllamaBase: "test:base"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}

	if _, err := os.Stat(filepath.Join(skillsDir, "evolve-gone")); !os.IsNotExist(err) {
		t.Error("stale sentinel-marked skill not pruned")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "evolve-mine", "SKILL.md")); err != nil {
		t.Error("user-authored evolve-mine was wrongly pruned")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "other-tool", "SKILL.md")); err != nil {
		t.Error("unrelated skill was wrongly pruned")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "evolve-build", "SKILL.md")); err != nil {
		t.Error("current skill not installed")
	}
}

func TestPublishAgy_StagingLayoutAndValidate(t *testing.T) {
	calls := stubPublishExec(t, true)
	project := publishTestProject(t, map[string]string{
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
	})
	cfg := publishConfig{Targets: []string{"agy"}, Prune: true, CodexHome: t.TempDir(), OllamaBase: "test:base"}

	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}

	stagingPlugin := filepath.Join(project, ".evolve", "publish", "agy", "evolve-loop")
	manifest, err := os.ReadFile(filepath.Join(stagingPlugin, "plugin.json"))
	if err != nil {
		t.Fatalf("plugin.json missing: %v", err)
	}
	var pj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(manifest, &pj); err != nil || pj.Name != "evolve-loop" {
		t.Errorf("plugin.json wrong: %s (err %v)", manifest, err)
	}

	staged, err := os.ReadFile(filepath.Join(stagingPlugin, "skills", "scout", "SKILL.md"))
	if err != nil {
		t.Fatalf("staged skill missing: %v", err)
	}
	if !strings.Contains(string(staged), "name: scout") {
		t.Errorf("agy skill name must stay unprefixed (plugin supplies namespace):\n%s", staged)
	}
	if !strings.Contains(string(staged), publishProvenanceSentinel) {
		t.Errorf("sentinel missing:\n%s", staged)
	}

	// validate runs even without --install; install must NOT have run.
	var sawValidate, sawInstall bool
	for _, c := range *calls {
		if c.Name == "agy" && len(c.Args) > 1 && c.Args[1] == "validate" {
			sawValidate = true
		}
		if c.Name == "agy" && len(c.Args) > 1 && c.Args[1] == "install" {
			sawInstall = true
		}
	}
	if !sawValidate {
		t.Errorf("agy plugin validate not invoked; calls: %v", *calls)
	}
	if sawInstall {
		t.Errorf("agy plugin install invoked without --install; calls: %v", *calls)
	}
}

func TestPublishAgy_InstallRunsAgyInstall(t *testing.T) {
	calls := stubPublishExec(t, true)
	project := publishTestProject(t, map[string]string{
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
	})
	cfg := publishConfig{Targets: []string{"agy"}, Prune: true, Install: true, CodexHome: t.TempDir(), OllamaBase: "test:base"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}
	var sawInstall bool
	for _, c := range *calls {
		if c.Name == "agy" && len(c.Args) > 1 && c.Args[1] == "install" {
			sawInstall = true
		}
	}
	if !sawInstall {
		t.Errorf("agy plugin install not invoked with --install; calls: %v", *calls)
	}
}

func TestPublishAgy_InstallFailsLoudlyWithoutBinary(t *testing.T) {
	stubPublishExec(t, false)
	project := publishTestProject(t, map[string]string{
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
	})
	cfg := publishConfig{Targets: []string{"agy"}, Prune: true, Install: true, CodexHome: t.TempDir(), OllamaBase: "test:base"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 1 {
		t.Fatalf("want exit 1 when --install requested but agy missing, got %d\nstderr:\n%s", code, errBuf.String())
	}
}

func TestPublishOllama_SelectsReadOnlySubset(t *testing.T) {
	stubPublishExec(t, false)
	project := publishTestProject(t, map[string]string{
		"build": publishSkillDoc("build", "builder skill", "Build things."),
		"tdd":   publishSkillDoc("tdd", "tdd skill", "Write RED tests."),
		"loop":  publishSkillDoc("loop", "loop skill", "Run cycles."),
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
		"audit": publishSkillDoc("audit", "audit skill", "Audit things."),
	})
	cfg := publishConfig{Targets: []string{"ollama"}, Prune: true, CodexHome: t.TempDir(), OllamaBase: "llama3.1:8b"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}

	staging := filepath.Join(project, ".evolve", "publish", "ollama")
	for _, writer := range []string{"evolve-build", "evolve-tdd", "evolve-loop"} {
		if _, err := os.Stat(filepath.Join(staging, writer)); !os.IsNotExist(err) {
			t.Errorf("write/orchestration skill %s must not be projected to read-only ollama", writer)
		}
	}
	for _, reader := range []string{"evolve-scout", "evolve-audit"} {
		mf, err := os.ReadFile(filepath.Join(staging, reader, "Modelfile"))
		if err != nil {
			t.Fatalf("%s Modelfile missing: %v", reader, err)
		}
		s := string(mf)
		if !strings.Contains(s, "FROM llama3.1:8b") {
			t.Errorf("%s: FROM base wrong:\n%s", reader, s)
		}
		if !strings.Contains(s, publishProvenanceSentinel) {
			t.Errorf("%s: sentinel missing:\n%s", reader, s)
		}
		if !strings.Contains(s, "SYSTEM \"\"\"") {
			t.Errorf("%s: SYSTEM block missing:\n%s", reader, s)
		}
	}
	scout, _ := os.ReadFile(filepath.Join(staging, "evolve-scout", "Modelfile"))
	if !strings.Contains(string(scout), "scout skill") || !strings.Contains(string(scout), "Scout things.") {
		t.Errorf("description/body not embedded in SYSTEM:\n%s", scout)
	}

	// Manifest records exactly the projected models.
	raw, err := os.ReadFile(filepath.Join(staging, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest.json missing: %v", err)
	}
	var m struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest unparseable: %v", err)
	}
	want := []string{"evolve-audit", "evolve-scout"}
	if fmt.Sprint(m.Models) != fmt.Sprint(want) {
		t.Errorf("manifest models %v, want %v", m.Models, want)
	}
}

func TestPublishOllama_BaseOverrideAndTripleQuoteSanitized(t *testing.T) {
	stubPublishExec(t, false)
	project := publishTestProject(t, map[string]string{
		"scout": publishSkillDoc("scout", "scout skill", "Use \"\"\" fences carefully."),
	})
	cfg := publishConfig{Targets: []string{"ollama"}, Prune: true, CodexHome: t.TempDir(), OllamaBase: "qwen3:4b"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}
	mf, err := os.ReadFile(filepath.Join(project, ".evolve", "publish", "ollama", "evolve-scout", "Modelfile"))
	if err != nil {
		t.Fatalf("Modelfile missing: %v", err)
	}
	s := string(mf)
	if !strings.Contains(s, "FROM qwen3:4b") {
		t.Errorf("--ollama-base override not honored:\n%s", s)
	}
	// The only """ occurrences must be the SYSTEM delimiters (exactly 2).
	if n := strings.Count(s, `"""`); n != 2 {
		t.Errorf("body triple-quote not sanitized: %d fences found:\n%s", n, s)
	}
}

func TestPublishOllama_InstallCreatesAndPrunesStaleModels(t *testing.T) {
	calls := stubPublishExec(t, true)
	project := publishTestProject(t, map[string]string{
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
	})
	// Pre-existing manifest from an earlier publish that included evolve-audit.
	writeFileForPublishTest(t, filepath.Join(project, ".evolve", "publish", "ollama", "manifest.json"),
		`{"models": ["evolve-audit", "evolve-scout"]}`)

	cfg := publishConfig{Targets: []string{"ollama"}, Prune: true, Install: true, CodexHome: t.TempDir(), OllamaBase: "llama3.1:8b"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}

	var created, removed []string
	for _, c := range *calls {
		if c.Name != "ollama" || len(c.Args) == 0 {
			continue
		}
		switch c.Args[0] {
		case "create":
			created = append(created, c.Args[1])
		case "rm":
			removed = append(removed, c.Args[1])
		}
	}
	if fmt.Sprint(created) != fmt.Sprint([]string{"evolve-scout"}) {
		t.Errorf("created %v, want [evolve-scout]", created)
	}
	if fmt.Sprint(removed) != fmt.Sprint([]string{"evolve-audit"}) {
		t.Errorf("removed %v, want [evolve-audit] (stale model from old manifest)", removed)
	}
}

func TestRunSkillsPublish_DryRunWritesNothing(t *testing.T) {
	calls := stubPublishExec(t, true)
	project := publishTestProject(t, map[string]string{
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
	})
	codexHome := t.TempDir()
	cfg := publishConfig{Targets: []string{"codex", "agy", "ollama"}, Prune: true, DryRun: true, Install: true, CodexHome: codexHome, OllamaBase: "llama3.1:8b"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("exit %d\nstderr:\n%s", code, errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(project, ".evolve", "publish")); !os.IsNotExist(err) {
		t.Error("dry-run wrote staging files")
	}
	if entries, _ := os.ReadDir(filepath.Join(codexHome, "skills")); len(entries) != 0 {
		t.Error("dry-run mutated codex home")
	}
	if len(*calls) != 0 {
		t.Errorf("dry-run executed commands: %v", *calls)
	}
	if !strings.Contains(out.String(), "evolve-scout") {
		t.Errorf("dry-run plan missing projected names:\n%s", out.String())
	}
}

func TestRunSkillsPublish_CheckDetectsStagingDrift(t *testing.T) {
	stubPublishExec(t, true)
	project := publishTestProject(t, map[string]string{
		"scout": publishSkillDoc("scout", "scout skill", "Scout things."),
	})
	cfg := publishConfig{Targets: []string{"codex"}, Prune: true, CodexHome: t.TempDir(), OllamaBase: "llama3.1:8b"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 0 {
		t.Fatalf("stage exit %d\nstderr:\n%s", code, errBuf.String())
	}

	checkCfg := cfg
	checkCfg.Check = true
	out.Reset()
	errBuf.Reset()
	if code := runSkillsPublish(project, checkCfg, &out, &errBuf); code != 0 {
		t.Fatalf("check on fresh staging: exit %d, want 0\nstderr:\n%s", code, errBuf.String())
	}

	// Mutate the staged file → drift.
	staged := filepath.Join(project, ".evolve", "publish", "codex", "evolve-scout", "SKILL.md")
	if err := os.WriteFile(staged, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errBuf.Reset()
	if code := runSkillsPublish(project, checkCfg, &out, &errBuf); code != 2 {
		t.Fatalf("check on drifted staging: exit %d, want 2\nstderr:\n%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "DRIFT") {
		t.Errorf("stderr missing DRIFT report:\n%s", errBuf.String())
	}
}

// TestRunSkillsPublish_CheckFailsOnRenderError pins the review finding: a
// render failure (e.g. a skill with no name: line) must exit 1 even in check
// mode — never a false-green "check OK".
func TestRunSkillsPublish_CheckFailsOnRenderError(t *testing.T) {
	stubPublishExec(t, false)
	project := publishTestProject(t, map[string]string{
		"broken": "---\ndescription: no name line\n---\nbody\n",
	})
	cfg := publishConfig{Targets: []string{"codex"}, Prune: true, Check: true, CodexHome: t.TempDir(), OllamaBase: "test:base"}
	var out, errBuf bytes.Buffer
	if code := runSkillsPublish(project, cfg, &out, &errBuf); code != 1 {
		t.Fatalf("check with render error: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
	if strings.Contains(out.String(), "check OK") {
		t.Errorf("false-green check OK printed despite render failure:\n%s", out.String())
	}
}

func TestRunSkills_PublishUnknownTargetExits10(t *testing.T) {
	stubPublishExec(t, false)
	var out, errBuf bytes.Buffer
	if code := runSkills([]string{"publish", "--target", "vscode"}, nil, &out, &errBuf); code != 10 {
		t.Fatalf("exit %d, want 10\nstderr:\n%s", code, errBuf.String())
	}
}

func TestParsePublishFlags_Defaults(t *testing.T) {
	var errBuf bytes.Buffer
	cfg, ok := parsePublishFlags(nil, &errBuf)
	if !ok {
		t.Fatalf("default parse failed: %s", errBuf.String())
	}
	if fmt.Sprint(cfg.Targets) != fmt.Sprint([]string{"codex", "agy", "ollama"}) {
		t.Errorf("default targets %v", cfg.Targets)
	}
	if !cfg.Prune || cfg.Install || cfg.DryRun || cfg.Check {
		t.Errorf("default booleans wrong: %+v", cfg)
	}
	if cfg.OllamaBase == "" || cfg.CodexHome == "" {
		t.Errorf("defaults unresolved: %+v", cfg)
	}
}

// readTreeForPublishTest returns rel path → content for every file under root.
func readTreeForPublishTest(t *testing.T, root string) map[string]string {
	t.Helper()
	got := map[string]string{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		got[rel] = string(data)
		return nil
	})
	return got
}
