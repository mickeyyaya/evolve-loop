package releasepreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

const deadTokenManifest = `{"forbidden":[{"token":"oldname","replacement":"newname","reason":"renamed"}],"exclude":[".evolve/naming.json"]}`

func nameGuardRepo(t *testing.T, manifest string, files map[string]string) string {
	t.Helper()
	r := gittest.Fixture(t)
	all := map[string]string{naminguard.DefaultManifestPath: manifest}
	for k, v := range files {
		all[k] = v
	}
	for rel, body := range all {
		p := filepath.Join(r.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "seed")
	return r.Dir
}

func TestDefaultNameGuard_InvalidManifestIsAnError(t *testing.T) {
	for name, manifest := range map[string]string{
		"not json":          "{",
		"no tokens":         `{"forbidden":[]}`,
		"empty replacement": `{"forbidden":[{"token":"x","replacement":""}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, naminguard.DefaultManifestPath)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(manifest), 0o644); err != nil {
				t.Fatal(err)
			}

			violations, err := defaultNameGuard(root)

			if err == nil || violations != nil || !strings.HasPrefix(err.Error(), "naminguard: ") {
				t.Errorf("= (%v, %v), want (nil, a naminguard manifest error)", violations, err)
			}
		})
	}
}

func TestDefaultNameGuard_ScansTrackedFilesForDeadTokens(t *testing.T) {
	root := nameGuardRepo(t, deadTokenManifest, map[string]string{
		"docs/a.md":  "line one\nsee oldname here\n",
		"docs/ok.md": "only newname\n",
	})

	violations, err := defaultNameGuard(root)

	if err != nil {
		t.Fatalf("defaultNameGuard: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want exactly one", violations)
	}
	if v := violations[0]; v.File != "docs/a.md" || v.Line != 2 || v.Token != "oldname" {
		t.Errorf("violation = %+v, want docs/a.md:2 oldname", v)
	}
}

func TestDefaultNameGuard_CleanTreeHasNoViolations(t *testing.T) {
	root := nameGuardRepo(t, deadTokenManifest, map[string]string{"docs/ok.md": "newname only\n"})

	violations, err := defaultNameGuard(root)

	if err != nil || len(violations) != 0 {
		t.Errorf("= (%v, %v), want no violations", violations, err)
	}
}
