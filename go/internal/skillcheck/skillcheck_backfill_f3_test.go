package skillcheck

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFileF3(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillF3(t *testing.T, root, dir, body string) {
	t.Helper()
	writeFileF3(t, filepath.Join(root, "skills", dir, "SKILL.md"), body)
}

func TestCommandDiffs_OnlyASkillDirectoryWithAReadableSkillGetsAStub(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skillF3(t, root, "alpha", "---\nname: alpha\ndescription: first skill\n---\nbody\n")
	if err := os.MkdirAll(filepath.Join(root, "skills", "no-skill-file"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileF3(t, filepath.Join(root, "skills", "README.md"), "a stray file, not a skill")

	diffs, err := commandDiffs(root)

	if err != nil || len(diffs) != 1 || diffs[0].rel != filepath.Join("commands", "alpha.md") || !diffs[0].drifted || diffs[0].orphan {
		t.Fatalf("diffs=%+v err=%v", diffs, err)
	}
}

func TestCommandDiffs_AMissingSkillsDirectoryIsAnError(t *testing.T) {
	t.Parallel()

	diffs, err := commandDiffs(t.TempDir())

	if diffs != nil || err == nil || !strings.Contains(err.Error(), "read skills dir:") {
		t.Fatalf("diffs=%v err=%v", diffs, err)
	}
}

func TestAgentProblems_BlankEntriesAreIgnoredAndMissingFilesAreQuoted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFileF3(t, filepath.Join(root, "agents", "a.md"), "agent")

	problems := agentProblems(root, []string{"", "./", "./agents/a.md", "agents/b.md", "./agents/c.md"})

	want := []string{
		`MANIFEST: agents[] lists "agents/b.md" but the file is missing`,
		`MANIFEST: agents[] lists "./agents/c.md" but the file is missing`,
	}
	if len(problems) != 2 || problems[0] != want[0] || problems[1] != want[1] {
		t.Fatalf("problems = %q, want %q", problems, want)
	}
	if got := agentProblems(root, nil); got != nil {
		t.Fatalf("no agents means no problems, got %q", got)
	}
}

func TestCodexManifestDiffs_AnUnusableClaudeManifestIsAnErrorNotAnAbsence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(t *testing.T, root string)
		wantErr string
	}{
		{name: "invalid json", setup: func(t *testing.T, root string) {
			writeFileF3(t, filepath.Join(root, ".claude-plugin", "plugin.json"), "{not json")
		}, wantErr: "parse .claude-plugin/plugin.json"},
		{name: "a directory where the file belongs", setup: func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".claude-plugin", "plugin.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, wantErr: "read .claude-plugin/plugin.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			tc.setup(t, root)

			diffs, err := codexManifestDiffs(root)

			if diffs != nil || err == nil || !strings.Contains(err.Error(), tc.wantErr) || errors.Is(err, os.ErrNotExist) {
				t.Fatalf("diffs=%v err=%v", diffs, err)
			}
		})
	}
}

func TestCodexManifestDiffsFor_ARendererFailureNamesTheManifestAndStopsTheProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeManifest(t, root)
	boom := errors.New("render exploded")
	specs := []codexManifestSpec{
		{rel: "first.json", render: func(claudePluginMeta) ([]byte, error) { return []byte("ok"), nil }},
		{rel: "second.json", render: func(claudePluginMeta) ([]byte, error) { return nil, boom }},
	}

	diffs, err := codexManifestDiffsFor(root, specs)

	if diffs != nil || !errors.Is(err, boom) || !strings.Contains(err.Error(), "render second.json:") {
		t.Fatalf("diffs=%v err=%v", diffs, err)
	}
}

func TestCodexManifestSpecs_ProjectBothManifestsInOrder(t *testing.T) {
	t.Parallel()
	specs := codexManifestSpecs()

	if len(specs) != 2 || specs[0].rel != codexPluginManifestRel || specs[1].rel != codexMarketplaceRel {
		t.Fatalf("specs = %+v", specs)
	}
	for _, s := range specs {
		if s.render == nil {
			t.Fatalf("%s has no renderer", s.rel)
		}
	}
}

func TestMarshalManifest_IndentsWithATrailingNewlineAndRefusesUnmarshalableValues(t *testing.T) {
	t.Parallel()
	b, err := marshalManifest(map[string]int{"a": 1})
	if err != nil || string(b) != "{\n  \"a\": 1\n}\n" {
		t.Fatalf("b=%q err=%v", b, err)
	}
	if b, err := marshalManifest(make(chan int)); b != nil || err == nil {
		t.Fatalf("b=%q err=%v, want an error for a channel", b, err)
	}
}

func TestWriteGenerated_WritesAndReportsOrNamesThePathItCouldNotWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var stdout strings.Builder

	if err := writeGenerated(filepath.Join(dir, "sub", "f.md"), "body", "sub/f.md", &stdout); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "sub", "f.md")); string(got) != "body" || stdout.String() != "[skills] generated sub/f.md\n" {
		t.Fatalf("file=%q stdout=%q", got, stdout.String())
	}
	blocker := filepath.Join(dir, "blocker")
	writeFileF3(t, blocker, "file")
	stdout.Reset()

	err := writeGenerated(filepath.Join(blocker, "f.md"), "body", "blocker/f.md", &stdout)

	if err == nil || !strings.Contains(err.Error(), "write "+filepath.Join(blocker, "f.md")+":") || stdout.Len() != 0 {
		t.Fatalf("err=%v stdout=%q", err, stdout.String())
	}
}

func TestProjectCommandDiffs_EachWriteModeFailureStopsAtOnceAndCheckModeOnlyReports(t *testing.T) {
	t.Parallel()
	t.Run("an orphan that cannot be removed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		full := filepath.Join(dir, "orphan")
		writeFileF3(t, filepath.Join(full, "child"), "x")
		var stdout, stderr strings.Builder

		drift, err := projectCommandDiffs([]commandDiff{{rel: "commands/orphan.md", path: full, drifted: true, orphan: true}}, true, &stdout, &stderr)

		if drift || err == nil || !strings.Contains(err.Error(), "remove "+full+":") || stdout.Len() != 0 {
			t.Fatalf("drift=%v err=%v stdout=%q", drift, err, stdout.String())
		}
	})
	t.Run("an orphan that is already gone", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr strings.Builder

		drift, err := projectCommandDiffs([]commandDiff{{rel: "commands/gone.md", path: filepath.Join(t.TempDir(), "gone.md"), drifted: true, orphan: true}}, true, &stdout, &stderr)

		if drift || err != nil || stdout.String() != "[skills] reaped orphan commands/gone.md\n" {
			t.Fatalf("drift=%v err=%v stdout=%q", drift, err, stdout.String())
		}
	})
	t.Run("a stub that cannot be written", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		blocker := filepath.Join(dir, "commands")
		writeFileF3(t, blocker, "file")
		var stdout, stderr strings.Builder

		drift, err := projectCommandDiffs([]commandDiff{{rel: "commands/x.md", path: filepath.Join(blocker, "x.md"), next: "stub", drifted: true}}, true, &stdout, &stderr)

		if drift || err == nil || !strings.Contains(err.Error(), "write ") {
			t.Fatalf("drift=%v err=%v", drift, err)
		}
	})
	t.Run("check mode reports each kind of drift and skips the clean ones", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr strings.Builder
		diffs := []commandDiff{
			{rel: "commands/stale.md", drifted: true},
			{rel: "commands/orphan.md", drifted: true, orphan: true},
			{rel: "commands/clean.md"},
		}

		drift, err := projectCommandDiffs(diffs, false, &stdout, &stderr)

		want := "DRIFT: commands/stale.md is stale or missing (run `evolve skills generate`)\nDRIFT: commands/orphan.md is an orphaned generated command (run `evolve skills generate`)\n"
		if !drift || err != nil || stderr.String() != want || stdout.Len() != 0 {
			t.Fatalf("drift=%v err=%v stderr=%q", drift, err, stderr.String())
		}
	})
}

func TestProjectSurfaces_EachStageFailureIsReturnedAndNothingLaterRuns(t *testing.T) {
	t.Parallel()
	t.Run("a phase skill that cannot be rewritten", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		blocker := filepath.Join(dir, "blocker")
		writeFileF3(t, blocker, "file")
		var stdout, stderr strings.Builder

		drift, err := projectSurfaces(dir, []factsDiff{{rel: "skills/x/SKILL.md", path: filepath.Join(blocker, "SKILL.md"), next: "n", drifted: true}}, true, &stdout, &stderr)

		if drift || err == nil || !strings.Contains(err.Error(), "write ") {
			t.Fatalf("drift=%v err=%v", drift, err)
		}
	})
	t.Run("a clean phase skill is left alone and check mode flags a stale one", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr strings.Builder
		diffs := []factsDiff{{rel: "skills/a/SKILL.md", drifted: false}, {rel: "skills/b/SKILL.md", drifted: true}}

		drift, err := projectSurfaces(root, diffs, false, &stdout, &stderr)

		if !drift || err != nil || stderr.String() != "DRIFT: skills/b/SKILL.md phase-facts region is stale (run `evolve skills generate`)\n" {
			t.Fatalf("drift=%v err=%v stderr=%q", drift, err, stderr.String())
		}
	})
	t.Run("no skills directory", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr strings.Builder

		drift, err := projectSurfaces(t.TempDir(), nil, false, &stdout, &stderr)

		if drift || err == nil || !strings.Contains(err.Error(), "read skills dir:") {
			t.Fatalf("drift=%v err=%v", drift, err)
		}
	})
	t.Run("a command stub that cannot be written", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		skillF3(t, root, "alpha", "---\nname: alpha\ndescription: d\n---\n")
		writeFileF3(t, filepath.Join(root, "commands"), "a file where the directory belongs")
		var stdout, stderr strings.Builder

		drift, err := projectSurfaces(root, nil, true, &stdout, &stderr)

		if drift || err == nil || !strings.Contains(err.Error(), "write ") {
			t.Fatalf("drift=%v err=%v", drift, err)
		}
	})
	t.Run("an unusable Claude manifest", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFileF3(t, filepath.Join(root, ".claude-plugin", "plugin.json"), "{not json")
		var stdout, stderr strings.Builder

		drift, err := projectSurfaces(root, nil, false, &stdout, &stderr)

		if drift || err == nil || !strings.Contains(err.Error(), "parse .claude-plugin/plugin.json") {
			t.Fatalf("drift=%v err=%v", drift, err)
		}
	})
	t.Run("a Codex manifest that cannot be written", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeClaudeManifest(t, root)
		writeFileF3(t, filepath.Join(root, ".codex-plugin"), "a file where the directory belongs")
		var stdout, stderr strings.Builder

		drift, err := projectSurfaces(root, nil, true, &stdout, &stderr)

		if drift || err == nil || !strings.Contains(err.Error(), "write ") {
			t.Fatalf("drift=%v err=%v", drift, err)
		}
	})
	t.Run("a Codex manifest that is stale in check mode", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeClaudeManifest(t, root)
		var stdout, stderr strings.Builder

		drift, err := projectSurfaces(root, nil, false, &stdout, &stderr)

		if !drift || err != nil || !strings.Contains(stderr.String(), codexPluginManifestRel) {
			t.Fatalf("drift=%v err=%v stderr=%q", drift, err, stderr.String())
		}
	})
}

func TestRun_AnUnusableClaudeManifestFailsTheRunWithExitOne(t *testing.T) {
	tmp := prepareSkillsTree(t)
	writeFileF3(t, filepath.Join(tmp, ".claude-plugin", "plugin.json"), "{not json")
	var stdout, stderr strings.Builder

	code := Run(tmp, false, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "parse .claude-plugin/plugin.json") || strings.Contains(stdout.String(), "check OK") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "MANIFEST: .claude-plugin/plugin.json is not valid JSON") {
		t.Fatalf("the manifest problem is reported before the projection fails: %q", stderr.String())
	}
}

func TestRun_ASkillsDirectoryThatCannotBeListedFailsTheRunWithExitOne(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root lists a mode-0111 directory")
	}
	tmp := prepareSkillsTree(t)
	writeClaudeManifest(t, tmp)
	skills := filepath.Join(tmp, "skills")
	if err := os.Chmod(skills, 0o111); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(skills, 0o755) })
	var stdout, stderr strings.Builder

	code := Run(tmp, false, &stdout, &stderr)

	if code != 1 || strings.Count(stderr.String(), "read skills dir:") != 2 {
		t.Fatalf("the name check and the manifest check each report the listing failure, and the run stops there: code=%d stderr=%q", code, stderr.String())
	}
}
