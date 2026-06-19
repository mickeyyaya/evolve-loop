package capabilities

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

type fakeSource struct {
	files  map[string][]byte
	dirErr error
}

func (f fakeSource) ReadFile(name string) ([]byte, error) {
	if b, ok := f.files[name]; ok {
		return b, nil
	}
	return nil, fs.ErrNotExist
}

func (f fakeSource) ReadDir(string) ([]fs.DirEntry, error) {
	if f.dirErr != nil {
		return nil, f.dirErr
	}
	return nil, nil
}

func withCatalogFS(t *testing.T, s catalogSource) {
	t.Helper()
	orig := catalogFS
	catalogFS = s
	t.Cleanup(func() { catalogFS = orig })
}

func TestLoadCatalog_AllEmbedded(t *testing.T) {
	for _, cli := range []string{"claude-tmux", "codex-tmux", "agy-tmux", "ollama-tmux"} {
		c, err := LoadCatalog(cli)
		if err != nil {
			t.Fatalf("%s: %v", cli, err)
		}
		if c.CLI != cli || len(c.SlashCommands) == 0 || c.Extension.Kind == "" {
			t.Errorf("%s: thin catalog %+v", cli, c)
		}
	}
}

func TestLoadCatalog_ExtensionKinds(t *testing.T) {
	// The research-grounded facts the catalog must encode.
	claude, _ := LoadCatalog("claude-tmux")
	if claude.Extension.Kind != "plugin_marketplace" || len(claude.Extension.InstallFlow) != 3 {
		t.Errorf("claude extension=%+v", claude.Extension)
	}
	codex, _ := LoadCatalog("codex-tmux")
	if codex.Extension.Kind != "plugin_marketplace" {
		t.Errorf("codex should now have a plugin system, got %q", codex.Extension.Kind)
	}
	ollama, _ := LoadCatalog("ollama-tmux")
	if ollama.Extension.Kind != "none" {
		t.Errorf("ollama should have no extension system, got %q", ollama.Extension.Kind)
	}
}

func TestLoadCatalog_EmptyName(t *testing.T) {
	if _, err := LoadCatalog(""); err == nil {
		t.Fatal("want error")
	}
}

func TestLoadCatalog_OverrideDirWins(t *testing.T) {
	dir := t.TempDir()
	override := `{"cli":"claude-tmux","slash_commands":[{"name":"/only"}],"extension":{"kind":"none"}}`
	if err := os.WriteFile(filepath.Join(dir, "claude-tmux.json"), []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := catalogDirFn
	catalogDirFn = func() string { return dir }
	t.Cleanup(func() { catalogDirFn = orig })
	c, err := LoadCatalog("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SlashCommands) != 1 || c.SlashCommands[0].Name != "/only" {
		t.Errorf("override not applied: %+v", c.SlashCommands)
	}
}

func TestLoadCatalog_NotFoundAndBadJSON(t *testing.T) {
	withCatalogFS(t, fakeSource{files: map[string][]byte{"catalogs/bad.json": []byte("{nope")}})
	if _, err := LoadCatalog("missing"); err == nil {
		t.Error("want not-found")
	}
	if _, err := LoadCatalog("bad"); err == nil {
		t.Error("want JSON error")
	}
}

func TestParseCatalog_NameFallback(t *testing.T) {
	c, err := parseCatalog("agy-tmux", []byte(`{"slash_commands":[],"extension":{"kind":"none"}}`))
	if err != nil || c.CLI != "agy-tmux" {
		t.Fatalf("c=%+v err=%v", c, err)
	}
}

func TestCatalogNames(t *testing.T) {
	names := CatalogNames()
	if len(names) < 4 {
		t.Errorf("expected >=4 catalogs, got %v", names)
	}
}

func TestCatalogNames_DirError(t *testing.T) {
	withCatalogFS(t, fakeSource{dirErr: errors.New("boom")})
	if names := CatalogNames(); names != nil {
		t.Errorf("want nil on ReadDir error, got %v", names)
	}
}

func TestParseHelpOutput(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want []string
	}{
		{"claude layout", "Available commands:\n  /help     Show help\n  /model    Switch model\n  /plugin   Manage plugins\n❯", []string{"/help", "/model", "/plugin"}},
		{"em-dash layout", "/clear — reset\n/bye — exit", []string{"/bye", "/clear"}},
		{"ollama layout", "Available Commands:\n  /set            Set session variables\n  /show           Show model information\n  /bye            Exit\n  /?              Help", []string{"/?", "/bye", "/set", "/show"}},
		{"dedup + prose ignored", "Type /help for help.\n/help\nsome prose here\n/help again", []string{"/help"}},
		{"no commands", "just prose, nothing here", nil},
		{"empty", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseHelpOutput(tc.pane)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", names(got), tc.want)
			}
			for i, w := range tc.want {
				if got[i].Name != w {
					t.Errorf("got[%d]=%q want %q", i, got[i].Name, w)
				}
			}
		})
	}
}

func TestParseHelp_Wrapper(t *testing.T) {
	if got := ParseHelp("/x\n/y"); len(got) != 2 {
		t.Errorf("ParseHelp got %v", got)
	}
}

func TestDiff(t *testing.T) {
	cat := Catalog{CLI: "claude-tmux", SlashCommands: []SlashCommand{{Name: "/help"}, {Name: "/model"}, {Name: "/plugin"}}}
	t.Run("clean when identical", func(t *testing.T) {
		live := []SlashCommand{{Name: "/help"}, {Name: "/model"}, {Name: "/plugin"}}
		d := Diff(cat, live)
		if !d.Clean() || d.CatalogCount != 3 || d.LiveCount != 3 {
			t.Errorf("drift=%+v", d)
		}
	})
	t.Run("reports both directions", func(t *testing.T) {
		live := []SlashCommand{{Name: "/help"}, {Name: "/newthing"}}
		d := Diff(cat, live)
		if d.Clean() {
			t.Fatal("expected drift")
		}
		if len(d.InCatalogNotLive) != 2 || d.InCatalogNotLive[0] != "/model" {
			t.Errorf("InCatalogNotLive=%v", d.InCatalogNotLive)
		}
		if len(d.InLiveNotCatalog) != 1 || d.InLiveNotCatalog[0] != "/newthing" {
			t.Errorf("InLiveNotCatalog=%v", d.InLiveNotCatalog)
		}
	})
}

func names(cmds []SlashCommand) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.Name
	}
	return out
}
