//go:build acs

package pluginnamespace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const wantNamespace = "evo"

type pluginManifest struct {
	Name string `json:"name"`
}

type marketplaceManifest struct {
	Name    string `json:"name"`
	Plugins []struct {
		Name string `json:"name"`
	} `json:"plugins"`
}

func loadManifest(t *testing.T, rel string, dst any) {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
}

func TestPluginManifest_NamespaceIsEvo(t *testing.T) {
	var pj pluginManifest
	loadManifest(t, ".claude-plugin/plugin.json", &pj)
	if pj.Name != wantNamespace {
		t.Errorf("plugin.json name = %q, want %q — the /%s:* slash-command namespace would break", pj.Name, wantNamespace, wantNamespace)
	}
}

func TestMarketplaceManifest_NamespaceIsEvo(t *testing.T) {
	var mp marketplaceManifest
	loadManifest(t, ".claude-plugin/marketplace.json", &mp)
	if mp.Name != wantNamespace {
		t.Errorf("marketplace.json name = %q, want %q", mp.Name, wantNamespace)
	}
	if len(mp.Plugins) == 0 {
		t.Fatal("marketplace.json has no plugins[] entry")
	}
	if got := mp.Plugins[0].Name; got != wantNamespace {
		t.Errorf("marketplace.json plugins[0].name = %q, want %q", got, wantNamespace)
	}
}

func TestManifests_NamespaceConsistent(t *testing.T) {
	var pj pluginManifest
	var mp marketplaceManifest
	loadManifest(t, ".claude-plugin/plugin.json", &pj)
	loadManifest(t, ".claude-plugin/marketplace.json", &mp)
	if len(mp.Plugins) == 0 {
		t.Fatal("marketplace.json has no plugins[] entry")
	}
	if pj.Name != mp.Plugins[0].Name {
		t.Errorf("namespace disagreement: plugin.json name = %q but marketplace plugins[0].name = %q", pj.Name, mp.Plugins[0].Name)
	}
}

func TestLoopSkill_DescribesEvoCommand(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "skills", "loop", "SKILL.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := "/" + wantNamespace + ":loop"
	if !strings.Contains(string(raw), want) {
		t.Errorf("skills/loop/SKILL.md does not advertise %q — half-done rename?", want)
	}
}

func bareEvoSkillRefs(root string) ([]string, error) {
	skillsDir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills/: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, regexp.QuoteMeta(e.Name()))
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no skills found under %s", skillsDir)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	re := regexp.MustCompile(`(^|[^:a-zA-Z0-9/._-])/(` + strings.Join(names, "|") + `)([^-/a-zA-Z0-9]|$)`)

	var offenders []string
	walkErr := filepath.WalkDir(skillsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for i, line := range strings.Split(string(raw), "\n") {
			if re.MatchString(line) {
				offenders = append(offenders, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	return offenders, walkErr
}

func TestSkillRefsAreEvoNamespaced(t *testing.T) {
	offenders, err := bareEvoSkillRefs(acsassert.RepoRoot(t))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d bare evo-command ref(s) — use /%s:<skill>, not bare /<skill>:\n  %s",
			len(offenders), wantNamespace, strings.Join(offenders, "\n  "))
	}
}

func TestBareEvoSkillRefDetection(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "ok: /evo:demo\nbad: /demo here\npath: see [x](../demo/SKILL.md)\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	offenders, err := bareEvoSkillRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) != 1 || !strings.Contains(offenders[0], "/demo here") {
		t.Errorf("want exactly the bare /demo line flagged (not /evo:demo or the ../demo/ path), got: %v", offenders)
	}
}
