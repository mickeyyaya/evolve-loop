package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func e3InstallSource(t *testing.T, files ...string) string {
	t.Helper()
	src := t.TempDir()
	for _, rel := range files {
		writeFileForPublishTest(t, filepath.Join(src, rel), "# "+rel+"\n")
	}
	return src
}

func TestInstallClaudeDep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		files   []string
		wantErr string
	}{
		{"agents and skills install", []string{"agents/evolve-scout.md", "agents/evolve-build.md", "skills/loop/SKILL.md"}, ""},
		{"nothing to install", nil, "empty install (agents=0 skills=0)"},
		{"agents without skills", []string{"agents/evolve-scout.md"}, "empty install (agents=1 skills=0)"},
		{"skills without agents", []string{"skills/loop/SKILL.md", "skills/loop/phases.md"}, "empty install (agents=0 skills=2)"},
		{"non-evolve agent is ignored", []string{"agents/other.md", "skills/loop/SKILL.md"}, "empty install (agents=0 skills=1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()

			err := installClaudeDep(e3InstallSource(t, tc.files...), home)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("installClaudeDep: %v", err)
				}
				if _, serr := os.Stat(filepath.Join(home, ".claude", "agents", "evolve-build.md")); serr != nil {
					t.Errorf("the agent did not land in the isolated home: %v", serr)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestInstallClaudeDep_AnUnwritableHomeIsAnInstallError(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home-file")
	if err := os.WriteFile(home, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := installClaudeDep(e3InstallSource(t, "agents/evolve-scout.md", "skills/loop/SKILL.md"), home)

	if err == nil || !strings.HasPrefix(err.Error(), "claude install: mkdir agents dir: ") {
		t.Fatalf("err = %v, want the wrapped install error", err)
	}
}

func TestProjectTargetDryRunDep(t *testing.T) {
	t.Parallel()
	project := publishTestProject(t, map[string]string{
		"build": publishSkillDoc("build", "builder skill", "Build things."),
	})
	for _, target := range []string{"codex", "agy"} {
		if err := projectTargetDryRunDep(project, target); err != nil {
			t.Errorf("%s dry-run projection: %v", target, err)
		}
	}
	if _, err := os.Stat(filepath.Join(project, ".evolve", "publish")); !os.IsNotExist(err) {
		t.Errorf("a dry-run projection staged files: %v", err)
	}
	if err := projectTargetDryRunDep(t.TempDir(), "codex"); err == nil || err.Error() != "projection failed (rc=1)" {
		t.Errorf("a source with no plugin manifest: err = %v, want projection failed (rc=1)", err)
	}
}

func TestAssertGeminiPayloadPresentDep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		files   []string
		wantErr string
	}{
		{"complete payload", []string{"skills/loop/SKILL.md", ".claude-plugin/plugin.json"}, ""},
		{"no skills dir", []string{".claude-plugin/plugin.json"}, "payload missing skills: "},
		{"no plugin manifest", []string{"skills/loop/SKILL.md"}, "payload missing " + filepath.Join(".claude-plugin", "plugin.json") + ": "},
		{"empty source", nil, "payload missing skills: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertGeminiPayloadPresentDep(e3InstallSource(t, tc.files...))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) || !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("err = %v, want prefix %q wrapping a not-exist error", err, tc.wantErr)
			}
		})
	}
}
