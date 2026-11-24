package proctree

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const ownedID = "01R/1835/build/p4242n1"

func taggedProc(pid int, comm string, args ...string) Process {
	return Process{Pid: pid, Ppid: 1, Started: t0, Comm: comm, Args: args, Env: map[string]string{"EVOLVE_DISPATCH_ID": ownedID}}
}

func TestDispatchOwned_ATagAloneKillsOnlyAnAllowListedProgram(t *testing.T) {
	t.Parallel()
	allow := ProgramIn(append(Launchers(), "claude", "ollama", "antigravity-cli-x"))
	cases := []struct {
		name string
		p    Process
		want bool
	}{
		{"Firefox", taggedProc(1, "/Applications/Firefox.app/Contents/MacOS/firefox"), false},
		{"a WebKit web process", taggedProc(2, "WebKitWebProcess"), false},
		{"an Electron helper", taggedProc(3, "/Applications/X.app/Contents/Frameworks/X Helper (Renderer)"), false},
		{"ssh-agent", taggedProc(4, "/usr/bin/ssh-agent", "ssh-agent", "-l"), false},
		{"gpg-agent", taggedProc(5, "gpg-agent", "gpg-agent", "--daemon"), false},
		{"an unlisted language server", taggedProc(6, "gopls", "gopls"), false},
		{"a node MCP server", taggedProc(7, "node", "node", "mcp.js"), true},
		{"a pane shell", taggedProc(8, "/bin/zsh", "-zsh"), true},
		{"a versioned python", taggedProc(9, "/opt/homebrew/bin/python3.12", "python3.12", "-m", "srv"), true},
		{"a node shim with a version", taggedProc(10, "node20", "node20"), true},
		{"a name that only starts like python", taggedProc(11, "pythonista", "pythonista"), false},
		{"a family binary", taggedProc(12, "/Users/x/.local/bin/claude", "claude", "-p", "x"), true},
		{"ollama run", taggedProc(13, "ollama", "ollama", "run", "m"), true},
		{"ollama serve", taggedProc(14, "ollama", "ollama", "serve"), true},
		{"a Claude Code daemon", taggedProc(15, "claude", "claude", "daemon", "run"), false},
		{"a truncated comm completed by argv0", taggedProc(16, "uvx-extra-lon", "uvx-extra-long"), false},
		{"a truncated allow-listed comm", taggedProc(17, "python3.12-long", "python3.12-long"), false},
		{"a truncated comm that argv0 completes to an allowed name", taggedProc(19, "antigravity-cli", "antigravity-cli-x"), true},
		{"a truncated comm that argv0 completes to another name", taggedProc(20, "antigravity-cli", "antigravity-cli-y"), false},
		{"no program name at all", Process{Pid: 18, Ppid: 1, Started: t0, Env: map[string]string{"EVOLVE_DISPATCH_ID": ownedID}}, false},
	}
	for _, tc := range cases {
		got := Dispatch{ID: ownedID, Killable: allow}.Owned()(tc.p)
		if got != tc.want {
			t.Errorf("%s: selected=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDispatchOwned_ANilKillableSelectsNothingByTag(t *testing.T) {
	t.Parallel()
	node := taggedProc(7, "node", "node", "mcp.js")
	recorded := Dispatch{ID: ownedID, Tree: []Identity{node.Identity()}}.Owned()

	if (Dispatch{ID: ownedID}).Owned()(node) {
		t.Errorf("a nil Killable selected a tagged node: it must fail closed")
	}
	if !recorded(node) {
		t.Errorf("a nil Killable stopped a recorded member: only the tag branch is closed")
	}
}

func TestDispatchOwned_ARecordedSharedHelperNeedsALivePaneAttachment(t *testing.T) {
	t.Parallel()
	chrome := taggedProc(20, "Google Chrome")
	tree := []Identity{chrome.Identity()}

	detached := Dispatch{ID: ownedID, Tree: tree, Killable: ProgramIn(Launchers())}.Owned()
	attached := Dispatch{ID: ownedID, Tree: tree, Live: tree, Killable: ProgramIn(Launchers())}.Owned()

	if detached(chrome) {
		t.Errorf("a recorded shared helper with no live pane was selected")
	}
	if !attached(chrome) {
		t.Errorf("a recorded shared helper that is a live pane descendant was not selected")
	}
}

func TestLaunchers_HoldNoCLIFamilyAndReturnACopy(t *testing.T) {
	t.Parallel()
	got := Launchers()
	for _, fam := range []string{"claude", "agy", "codex", "gemini", "ollama"} {
		for _, l := range got {
			if l == fam {
				t.Errorf("launcher table holds the family binary %q: it belongs to the manifest", fam)
			}
		}
	}
	got[0] = "mutated"
	if Launchers()[0] == "mutated" {
		t.Errorf("Launchers returned its own backing array")
	}
}

func TestListTrees_ReturnsEachGoodFileAndWarnsOnTheRest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := "01R/1835/build/p4242n1"
	if err := SaveTree(dir, id, []Identity{{Pid: 30, Started: t0}}); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("bad.json", "{")
	write("liar.json", `{"dispatch":"other","members":[]}`)
	write(".tree-123", "{}")
	write("notes.txt", "x")
	if err := os.Mkdir(filepath.Join(dir, "sub.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	recs, warns, err := ListTrees(dir)

	want := []TreeRecord{{File: TreeFile(dir, id), Dispatch: id, Members: []Identity{{Pid: 30, Started: t0}}}}
	if err != nil || !reflect.DeepEqual(recs, want) || len(warns) != 2 {
		t.Errorf("ListTrees = %+v, %v, %v; want %+v, 2 warnings", recs, warns, err, want)
	}
	var syntax *json.SyntaxError
	if !errors.As(errors.Join(warns...), &syntax) {
		t.Errorf("warnings = %v, want a JSON syntax warning", warns)
	}
}

func TestListTrees_AMissingDirIsEmptyAndAFileForADirFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recs, warns, err := ListTrees(filepath.Join(dir, "none"))
	if recs != nil || warns != nil || err != nil {
		t.Errorf("missing dir: %v %v %v, want all nil", recs, warns, err)
	}
	file := filepath.Join(dir, "f")
	if werr := os.WriteFile(file, nil, 0o600); werr != nil {
		t.Fatal(werr)
	}
	if _, _, err := ListTrees(file); err == nil {
		t.Errorf("a regular file as the tree dir must fail")
	}
}

func TestListTrees_AnUnreadableFileWarns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	link := filepath.Join(dir, "dangling.json")
	if err := os.Symlink(filepath.Join(dir, "gone"), link); err != nil {
		t.Fatal(err)
	}
	_, warns, err := ListTrees(dir)
	if err != nil || len(warns) != 1 || !errors.Is(warns[0], fs.ErrNotExist) {
		t.Errorf("warns=%v err=%v, want one read warning", warns, err)
	}
}
