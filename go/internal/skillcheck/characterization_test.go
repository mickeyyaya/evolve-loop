package skillcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestManifestProblems_UnreadableManifestNamesItsPath(t *testing.T) {
	root := manifestTree(t)
	if err := os.Mkdir(filepath.Join(root, ".claude-plugin", "plugin.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("unreadable manifest is a problem, not an infra error: %v", err)
	}
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "MANIFEST: cannot read .claude-plugin/plugin.json: ") {
		t.Errorf("problems = %q, want one naming .claude-plugin/plugin.json", problems)
	}
}

func TestManifestProblems_InvalidJSONNamesItsPath(t *testing.T) {
	root := manifestTree(t)
	writeRawManifest(t, root, "{not json")
	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatalf("invalid JSON is a problem, not an infra error: %v", err)
	}
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "MANIFEST: .claude-plugin/plugin.json is not valid JSON: ") {
		t.Errorf("problems = %q, want one naming .claude-plugin/plugin.json", problems)
	}
}

func TestManifestProblems_DirWithoutSkillMDIsNotUnlisted(t *testing.T) {
	root := manifestTree(t)
	writeSkillDir(t, root, "real", wellFormedSkill("real"))
	if err := os.MkdirAll(filepath.Join(root, "skills", "notaskill"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, root, []string{"real"}, nil)
	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("a dir without SKILL.md must not be reported, got %q", problems)
	}
}

func TestManifestProblems_UnreadableSkillsDirIsInfraError(t *testing.T) {
	root := manifestTree(t)
	writeManifest(t, root, nil, nil)
	skills := filepath.Join(root, "skills")
	if err := os.Remove(skills); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skills, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ManifestProblems(root); err == nil || !strings.Contains(err.Error(), "read skills dir") {
		t.Errorf("err = %v, want a read skills dir infra error", err)
	}
}

func TestManifestProblems_MissingAgentQuotesEntryAndProblemsAreSorted(t *testing.T) {
	root := manifestTree(t)
	writeManifest(t, root, []string{"ghost"}, []string{"./agents/ghost.md"})
	problems, err := ManifestProblems(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`MANIFEST: agents[] lists "./agents/ghost.md" but the file is missing`,
		`MANIFEST: skills[] lists "ghost" but skills/ghost/SKILL.md is missing — the install breaks`,
	}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("problems = %q\nwant %q", problems, want)
	}
}

func dropRegistryPhase(t *testing.T, root, phase string) {
	t.Helper()
	path := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var reg map[string]any
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatal(err)
	}
	var kept []any
	for _, p := range reg["phases"].([]any) {
		if p.(map[string]any)["name"] != phase {
			kept = append(kept, p)
		}
	}
	reg["phases"] = kept
	out, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_CatalogWarningsCarryWARNPrefix(t *testing.T) {
	tmp := prepareSkillsTree(t)
	dropRegistryPhase(t, tmp, "intent")
	var stdout, stderr strings.Builder
	Run(tmp, false, &stdout, &stderr)
	if want := "WARN: phase \"intent\" not in catalog; skipping\n"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing %q:\n%s", want, stderr.String())
	}
}

func TestRun_NameMismatchFailsCheckButNotWrite(t *testing.T) {
	tmp := prepareSkillsTree(t)
	writeSkillDir(t, tmp, "zz-dir", "---\nname: zz-other\ndescription: d\n---\n\nbody\n")
	var stdout, stderr strings.Builder
	if code := Run(tmp, true, &stdout, &stderr); code != 0 {
		t.Fatalf("generate exit = %d, want 0\nstderr:\n%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(tmp, false, &stdout, &stderr); code != 2 {
		t.Errorf("check exit = %d, want 2", code)
	}
	if want := `DRIFT: skills/zz-dir/SKILL.md frontmatter name "zz-other" != dir name`; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing %q:\n%s", want, stderr.String())
	}
}

func TestRun_StaleFactsDriftLineNamesRemedy(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)
	var stdout, stderr strings.Builder
	if code := Run(tmp, false, &stdout, &stderr); code != 2 {
		t.Errorf("check exit = %d, want 2", code)
	}
	if want := "DRIFT: skills/build/SKILL.md phase-facts region is stale (run `evolve skills generate`)\n"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing %q:\n%s", want, stderr.String())
	}
}

func TestRun_OrphanCommandCheckAndReapLines(t *testing.T) {
	tmp := prepareSkillsTree(t)
	orphan := filepath.Join(tmp, "commands", "zz-orphan.md")
	if err := os.WriteFile(orphan, []byte(commandGenMarker+"zz-orphan/SKILL.md -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := Run(tmp, false, &stdout, &stderr); code != 2 {
		t.Errorf("check exit = %d, want 2", code)
	}
	if want := "DRIFT: commands/zz-orphan.md is an orphaned generated command (run `evolve skills generate`)\n"; !strings.Contains(stderr.String(), want) {
		t.Errorf("check stderr missing %q:\n%s", want, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(tmp, true, &stdout, &stderr); code != 0 {
		t.Fatalf("generate exit = %d\nstderr:\n%s", code, stderr.String())
	}
	if want := "[skills] reaped orphan commands/zz-orphan.md\n"; !strings.Contains(stdout.String(), want) {
		t.Errorf("generate stdout missing %q:\n%s", want, stdout.String())
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan still on disk: %v", err)
	}
}

func TestRun_CheckOKLine(t *testing.T) {
	tmp := prepareSkillsTree(t)
	var stdout, stderr strings.Builder
	if code := Run(tmp, false, &stdout, &stderr); code != 0 {
		t.Fatalf("check exit = %d\nstderr:\n%s", code, stderr.String())
	}
	want := "[skills] check OK — all phase-facts regions in sync, all commands mirrored, Codex manifests projected, all names match dirs\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestCollectSkillFacts_UserPhaseArtifactName(t *testing.T) {
	root := t.TempDir()
	bare := collectSkillFacts(root, phasespec.PhaseSpec{Name: "zz-user-phase"}, nil)
	if bare.ArtifactName != "" {
		t.Errorf("no-output user phase ArtifactName = %q, want empty", bare.ArtifactName)
	}
	declared := phasespec.PhaseSpec{Name: "zz-user-phase", Outputs: phasespec.IO{Files: []string{"out/zz.md"}}}
	if got := collectSkillFacts(root, declared, nil).ArtifactName; got != "zz.md" {
		t.Errorf("declared-output user phase ArtifactName = %q, want zz.md", got)
	}
}

func TestCollectSkillFacts_WriteTargetLabel(t *testing.T) {
	root := t.TempDir()
	for phase, want := range map[string]string{"orchestrator": ".evolve/", "build": "cycle workspace"} {
		if got := collectSkillFacts(root, phasespec.PhaseSpec{Name: phase}, nil).WriteTargetLabel; got != want {
			t.Errorf("%s WriteTargetLabel = %q, want %q", phase, got, want)
		}
	}
}

func commandRels(t *testing.T, root string) []string {
	t.Helper()
	diffs, err := commandDiffs(root)
	if err != nil {
		t.Fatal(err)
	}
	rels := []string{}
	for _, d := range diffs {
		rels = append(rels, d.rel)
	}
	return rels
}

func TestCommandDiffs_UnparseableFrontmatterGetsNoStub(t *testing.T) {
	root := t.TempDir()
	writeSkillDir(t, root, "broken", "---\nname: broken\n")
	if rels := commandRels(t, root); len(rels) != 0 {
		t.Errorf("unparseable skill projected %q, want none", rels)
	}
}

func TestCommandDiffs_NamelessSkillStubsToDirName(t *testing.T) {
	root := t.TempDir()
	writeSkillDir(t, root, "anon", "---\ndescription: d\n---\n\nbody\n")
	if rels := commandRels(t, root); !reflect.DeepEqual(rels, []string{"commands/anon.md"}) {
		t.Errorf("rels = %q, want [commands/anon.md]", rels)
	}
}

func TestCommandDiffs_NonMarkdownMarkerFileIsNotReaped(t *testing.T) {
	root := manifestTree(t)
	if err := os.WriteFile(filepath.Join(root, "commands", "notes.txt"), []byte(commandGenMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	if rels := commandRels(t, root); len(rels) != 0 {
		t.Errorf("rels = %q, want none", rels)
	}
}

func TestCommandDiffs_OrphanSortedWithCommandsPrefix(t *testing.T) {
	root := manifestTree(t)
	writeSkillFixture(t, root, "zzz", "d", "")
	if err := os.WriteFile(filepath.Join(root, "commands", "aaa.md"), []byte(commandGenMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	diffs, err := commandDiffs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 2 || diffs[0].rel != "commands/aaa.md" || !diffs[0].orphan || diffs[1].rel != "commands/zzz.md" {
		t.Errorf("diffs = %+v, want orphan commands/aaa.md then commands/zzz.md", diffs)
	}
}
