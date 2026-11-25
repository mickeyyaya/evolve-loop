package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

var widerSecretFiles = []string{".npmrc", ".pypirc", ".cargo/credentials.toml", ".cargo/credentials", ".codex/auth.json", ".gemini/oauth_creds.json", ".gemini/google_accounts.json"}
var widerSecretDirs = []string{".config/gcloud", ".azure"}

func wideHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	for _, rel := range widerSecretFiles {
		writeFixtureFile(t, filepath.Join(home, rel), "fake")
	}
	for _, rel := range widerSecretDirs {
		writeFixtureFile(t, filepath.Join(home, rel, "token.fixture"), "fake")
	}
	return home
}

func canonicalAll(home string, rels []string) []string {
	out := make([]string, len(rels))
	for i, rel := range rels {
		out[i] = canonicalOrSelf(filepath.Join(home, rel))
	}
	return out
}

func TestLoginSecrets_AreHomeSecrets(t *testing.T) {
	t.Parallel()
	for _, rel := range loginSecrets {
		if !slices.Contains(sandbox.HomeSecrets, rel) {
			t.Errorf("login secret %s must also be a home secret", rel)
		}
	}
}

func TestSecretsOf_ListsEachWiderSecretThatExists(t *testing.T) {
	t.Parallel()
	home := wideHome(t)
	dirs, files := secretsOf(home, homeReadsOf("claude-tmux"))
	for _, want := range canonicalAll(home, widerSecretFiles) {
		if !slices.Contains(files, want) {
			t.Errorf("files %v must list %s", files, want)
		}
	}
	for _, want := range canonicalAll(home, widerSecretDirs) {
		if !slices.Contains(dirs, want) {
			t.Errorf("dirs %v must list %s", dirs, want)
		}
	}
}

func TestSecretsOf_AMissingPathAddsNoEntry(t *testing.T) {
	t.Parallel()
	dirs, files := secretsOf(t.TempDir(), homeReadsOf("claude-tmux"))
	if len(dirs)+len(files) != 0 {
		t.Fatalf("an absent path is not listed: %v %v", dirs, files)
	}
}

func TestSecretsOf_ASymlinkedSecretIsDeniedAtItsRealTarget(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	real := filepath.Join(t.TempDir(), "dotfiles", "npmrc")
	writeFixtureFile(t, real, "fake")
	if err := os.Symlink(real, filepath.Join(home, ".npmrc")); err != nil {
		t.Fatal(err)
	}
	_, files := secretsOf(home, homeReadsOf("claude-tmux"))
	if want := canonicalOrSelf(real); !slices.Equal(files, []string{want}) {
		t.Fatalf("files %v, want the real target %s", files, want)
	}
}

func TestSecretsOf_ACellKeepsOnlyTheLoginItsManifestReads(t *testing.T) {
	t.Parallel()
	home := wideHome(t)
	codexAuth := canonicalOrSelf(filepath.Join(home, ".codex/auth.json"))
	geminiFiles := canonicalAll(home, []string{".gemini/oauth_creds.json", ".gemini/google_accounts.json"})
	cases := []struct {
		cli        string
		wantCodex  bool
		wantGemini bool
	}{
		{"codex", false, true}, {"codex-tmux", false, true},
		{"agy", true, false}, {"agy-tmux", true, false}, {"agy-claude-tmux", true, false},
		{"claude-tmux", true, true}, {"claude-p", true, true}, {"ollama-tmux", true, true},
		{"", true, true}, {"no-such-cli", true, true},
	}
	for _, c := range cases {
		_, files := secretsOf(home, homeReadsOf(c.cli))
		if got := slices.Contains(files, codexAuth); got != c.wantCodex {
			t.Errorf("cli %q: .codex/auth.json denied = %v, want %v", c.cli, got, c.wantCodex)
		}
		for _, g := range geminiFiles {
			if got := slices.Contains(files, g); got != c.wantGemini {
				t.Errorf("cli %q: %s denied = %v, want %v", c.cli, g, got, c.wantGemini)
			}
		}
	}
}

func TestValidateHomeReads_RefusesEachSecretThatIsNotALogin(t *testing.T) {
	t.Parallel()
	for _, read := range []string{
		"~/.config/gcloud", "~/.config/gcloud/application_default_credentials.json", "~/.azure", "~/.AZURE/x", "~/.npmrc", "~/.pypirc",
		"~/.cargo", "~/.cargo/credentials.toml", "~/.cargo/credentials", "~/.git-credentials", "~/.config/git/credentials", "~/.config",
	} {
		if err := validateHomeReads("x", []string{read}); !errors.Is(err, errHomeReadOverlapsSecret) {
			t.Errorf("home_reads %s: err = %v, want errHomeReadOverlapsSecret", read, err)
		}
	}
}

func TestValidateHomeReads_AcceptsTheLoginDirectoryOfTheOwnTool(t *testing.T) {
	t.Parallel()
	for _, read := range []string{"~/.codex", "~/.gemini", "~/.codex/config.toml", "~/.config/agy"} {
		if err := validateHomeReads("x", []string{read}); err != nil {
			t.Errorf("home_reads %s must be valid: %v", read, err)
		}
	}
}

func TestReadIsolation_TheRequestCLIDecidesWhichLoginTheCellKeeps(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	home := wideHome(t)
	req := c.request()
	req.CLI = "codex"
	ri := Deps{ReadStage: "enforce"}.readIsolation(req, home)
	if slices.Contains(ri.SecretFiles, canonicalOrSelf(filepath.Join(home, ".codex/auth.json"))) {
		t.Fatalf("a codex cell keeps its own login: %v", ri.SecretFiles)
	}
	req.CLI = "claude-tmux"
	ri = Deps{ReadStage: "enforce"}.readIsolation(req, home)
	if !slices.Contains(ri.SecretFiles, canonicalOrSelf(filepath.Join(home, ".codex/auth.json"))) {
		t.Fatalf("a claude cell must not read the codex login: %v", ri.SecretFiles)
	}
}

func TestOwnsLogin_OnlyAnEntryThatNamesThePathOrAParentCovers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		reads []string
		want  bool
	}{
		{"the parent directory", []string{"~/.codex"}, true},
		{"the exact file", []string{"~/.codex/auth.json"}, true},
		{"the parent in upper case", []string{"~/.CODEX"}, true},
		{"a trailing slash", []string{"~/.codex/"}, true},
		{"a name prefix of the directory", []string{"~/.code"}, false},
		{"a name prefix of another directory", []string{"~/.gem"}, false},
		{"a sibling file", []string{"~/.codex/config.toml"}, false},
		{"another family directory", []string{"~/.gemini"}, false},
		{"no entry", nil, false},
		{"an entry outside the home", []string{"/.codex"}, false},
	}
	for _, c := range cases {
		if got := ownsLogin(c.reads, ".codex/auth.json"); got != c.want {
			t.Errorf("%s: ownsLogin(%v) = %v, want %v", c.name, c.reads, got, c.want)
		}
	}
}

func TestHomeReadsOf_AnUnknownOrEmptyCLIReadsNothing(t *testing.T) {
	t.Parallel()
	for _, cli := range []string{"", "no-such-cli"} {
		if got := homeReadsOf(cli); len(got) != 0 {
			t.Errorf("homeReadsOf(%q) = %v, want none", cli, got)
		}
	}
	if got := homeReadsOf("agy-claude-tmux"); !slices.Contains(got, "~/.gemini") {
		t.Errorf("a manifest that inherits its base keeps the home_reads of the base: %v", got)
	}
}
