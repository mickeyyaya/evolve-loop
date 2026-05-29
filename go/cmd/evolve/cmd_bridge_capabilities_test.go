package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCapCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := runBridge(args, nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCapabilitiesCLI_Text(t *testing.T) {
	code, out, _ := runCapCLI("capabilities", "--cli=claude-tmux")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "/plugin") || !strings.Contains(out, "plugin_marketplace") {
		t.Errorf("unexpected text output: %q", out)
	}
}

func TestCapabilitiesCLI_JSON(t *testing.T) {
	code, out, _ := runCapCLI("capabilities", "--cli=ollama-tmux", "--json")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out, "\"cli\": \"ollama-tmux\"") || !strings.Contains(out, "\"kind\": \"none\"") {
		t.Errorf("unexpected json output: %q", out)
	}
}

func TestCapabilitiesCLI_Errors(t *testing.T) {
	if code, _, _ := runCapCLI("capabilities"); code != 10 {
		t.Errorf("no --cli code=%d want 10", code)
	}
	if code, _, _ := runCapCLI("capabilities", "--cli=bogus-cli"); code != 10 {
		t.Errorf("bad cli code=%d want 10", code)
	}
	if code, _, _ := runCapCLI("capabilities", "--zzz"); code != 10 {
		t.Errorf("bad flag code=%d want 10", code)
	}
	if code, out, _ := runCapCLI("capabilities", "--help"); code != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help code=%d", code)
	}
}

// writeOverrideCatalog points EVOLVE_BRIDGE_CATALOG_DIR at a temp dir holding a
// tiny deterministic catalog, so drift tests don't depend on the full embedded
// command list.
func writeOverrideCatalog(t *testing.T, cmds string) {
	t.Helper()
	dir := t.TempDir()
	cat := `{"cli":"claude-tmux","slash_commands":[` + cmds + `],"extension":{"kind":"plugin_marketplace"}}`
	if err := os.WriteFile(filepath.Join(dir, "claude-tmux.json"), []byte(cat), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_BRIDGE_CATALOG_DIR", dir)
}

func TestIntrospectCLI_OfflineClean(t *testing.T) {
	writeOverrideCatalog(t, `{"name":"/help"},{"name":"/model"}`)
	paneFile := filepath.Join(t.TempDir(), "pane.txt")
	if err := os.WriteFile(paneFile, []byte("/help  Show help\n/model Switch\n❯"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCapCLI("introspect", "--cli=claude-tmux", "--pane-file="+paneFile)
	if code != 0 {
		t.Fatalf("clean drift code=%d out=%q", code, out)
	}
	if !strings.Contains(out, "\"catalog_count\": 2") {
		t.Errorf("out=%q", out)
	}
}

func TestIntrospectCLI_OfflineDrift(t *testing.T) {
	writeOverrideCatalog(t, `{"name":"/help"},{"name":"/model"}`)
	paneFile := filepath.Join(t.TempDir(), "pane.txt")
	// Live has a new command and is missing /model → drift in both directions.
	if err := os.WriteFile(paneFile, []byte("/help  Show help\n/newcmd  New\n❯"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCapCLI("introspect", "--cli=claude-tmux", "--pane-file="+paneFile)
	if code != 3 {
		t.Fatalf("drift code=%d want 3 out=%q", code, out)
	}
	if !strings.Contains(out, "/newcmd") || !strings.Contains(out, "/model") {
		t.Errorf("drift report missing entries: %q", out)
	}
}

func TestIntrospectCLI_Errors(t *testing.T) {
	if code, _, _ := runCapCLI("introspect"); code != 10 {
		t.Errorf("no --cli code=%d want 10", code)
	}
	if code, _, _ := runCapCLI("introspect", "--cli=bogus-cli"); code != 10 {
		t.Errorf("bad cli code=%d want 10", code)
	}
	if code, _, _ := runCapCLI("introspect", "--cli=claude-tmux"); code != 10 {
		t.Errorf("live without --workspace code=%d want 10", code)
	}
	if code, _, _ := runCapCLI("introspect", "--cli=claude-tmux", "--pane-file=/no/such/file"); code != 10 {
		t.Errorf("missing pane-file code=%d want 10", code)
	}
	if code, _, _ := runCapCLI("introspect", "--zzz"); code != 10 {
		t.Errorf("bad flag code=%d want 10", code)
	}
	if code, out, _ := runCapCLI("introspect", "--help"); code != 0 || !strings.Contains(out, "Usage") {
		t.Errorf("help code=%d", code)
	}
}
