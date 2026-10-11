package versionbump

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func runFixture(t *testing.T) Paths {
	t.Helper()
	tmp := t.TempDir()
	paths := Paths{
		PluginJSON:      filepath.Join(tmp, "plugin.json"),
		MarketplaceJSON: filepath.Join(tmp, "marketplace.json"),
		CodexPluginJSON: filepath.Join(tmp, ".codex-plugin", "plugin.json"),
		SkillMD:         filepath.Join(tmp, "SKILL.md"),
		ReadmeMD:        filepath.Join(tmp, "README.md"),
	}
	writeFile(t, paths.PluginJSON, `{"name":"evo","version":"21.4.1"}`)
	writeFile(t, paths.MarketplaceJSON, `{"plugins":[{"name":"evo","version":"21.4.1"}]}`)
	writeFile(t, paths.SkillMD, "# Evolve Loop v21.4\n")
	writeFile(t, paths.ReadmeMD, "Current (v21.4)\n\n| v21.4 | Jun 29 | shipped |\n")
	return paths
}

func TestRun_UnstatableCodexMirrorStopsTheBumpLoudly(t *testing.T) {
	paths := runFixture(t)
	blocker := filepath.Join(t.TempDir(), "file")
	writeFile(t, blocker, "")
	paths.CodexPluginJSON = filepath.Join(blocker, "plugin.json")

	res, err := Run(paths, "21.5.0", false, time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))

	if err == nil || !strings.HasPrefix(err.Error(), "versionbump: stat "+paths.CodexPluginJSON) {
		t.Fatalf("err = %v, want the codex stat failure", err)
	}
	want := []string{".claude-plugin/plugin.json", ".claude-plugin/marketplace.json"}
	if !reflect.DeepEqual(res.Modified, want) {
		t.Errorf("Modified = %v, want only the steps before the failure %v", res.Modified, want)
	}
	if skill, _ := os.ReadFile(paths.SkillMD); string(skill) != "# Evolve Loop v21.4\n" {
		t.Errorf("SKILL.md = %q, want untouched after the failure", skill)
	}
}

func TestBumpReadmeHistory_SecondReadFailureIsReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	writeFile(t, path, "| v21.4 | Jun 29 | shipped |\n")
	prev := readFile
	t.Cleanup(func() { readFile = prev })
	cause := errors.New("file vanished")
	readFile = func(string) ([]byte, error) { return nil, cause }

	changed, err := BumpReadmeHistory(path, "21.5.0", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), false)

	if changed || !errors.Is(err, cause) {
		t.Fatalf("= (%v, %v), want (false, %v)", changed, err, cause)
	}
	if body, _ := os.ReadFile(path); string(body) != "| v21.4 | Jun 29 | shipped |\n" {
		t.Errorf("README = %q, want untouched", body)
	}
}
