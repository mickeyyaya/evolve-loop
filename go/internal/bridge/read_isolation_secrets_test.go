package bridge

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

var widerSecrets = []struct {
	rel   string
	dir   bool
	owner string
}{
	{rel: ".git-credentials"},
	{rel: ".config/git/credentials"},
	{rel: ".config/gcloud", dir: true},
	{rel: ".azure", dir: true},
	{rel: ".npmrc"},
	{rel: ".pypirc"},
	{rel: ".cargo/credentials.toml"},
	{rel: ".codex/auth.json", owner: "codex"},
	{rel: ".gemini/oauth_creds.json", owner: "agy"},
	{rel: ".gemini/mcp-oauth-tokens.json", owner: "agy"},
}

func TestValidateHomeReads_RefusesAGrantThatOverlapsEachWiderSecret(t *testing.T) {
	t.Parallel()
	for _, s := range widerSecrets {
		field := fmt.Sprintf(`"home_reads":["~/%s"]`, s.rel)
		_, err := parseManifestWithStderr("x", []byte(`{"cli":"x","binary":"x",`+field+`}`), io.Discard)
		want := fmt.Sprintf(`home_reads entry "~/%s" for cli=x overlaps the secret path ~/%s`, s.rel, s.rel)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("manifest with %s: err = %v, want %q", field, err, want)
		}
	}
}

func TestValidateHomeReads_TheOwnerOfASignInMayReadItsHomeAndNoOtherBinaryMay(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ binary, read, want string }{
		{"codex", "~/.codex", ""},
		{"agy", "~/.gemini", ""},
		{"claude", "~/.codex", "overlaps the secret path ~/.codex/auth.json"},
		{"codex", "~/.gemini", "overlaps the secret path ~/.gemini/oauth_creds.json"},
		{"codex", "~/.git-credentials", "overlaps the secret path ~/.git-credentials"},
	} {
		_, err := parseManifestWithStderr("x", []byte(fmt.Sprintf(`{"cli":"x","binary":%q,"home_reads":[%q]}`, c.binary, c.read)), io.Discard)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("binary %s reads %s: err = %v, want %q", c.binary, c.read, err, c.want)
		}
	}
}

func TestReadIsolation_ReportsEachWiderSecretWithTheHomeSecretRuleForACellThatDoesNotOwnIt(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	for _, s := range widerSecrets {
		path := filepath.Join(c.home, s.rel)
		if s.dir {
			path = filepath.Join(path, "fixture")
		}
		writeFixtureFile(t, path, "secret\n")
	}
	ri := Deps{ReadStage: "shadow"}.readIsolation(c.request(), c.home)
	sbpl := sandbox.GenerateSBPL(sandbox.Config{ReadIsolation: ri})
	for _, s := range widerSecrets {
		path, filter, list := filepath.Join(c.home, s.rel), "literal", ri.SecretFiles
		if s.dir {
			filter, list = "subpath", ri.SecretDirs
		}
		if !slices.Contains(list, path) {
			t.Errorf("the secret %s is missing from dirs %v files %v", s.rel, ri.SecretDirs, ri.SecretFiles)
		}
		want := fmt.Sprintf(`(allow file-read-data (%s %q) (with report) (with message "evolve-cell=d42 rule=%s inv=I5"))`, filter, path, sandbox.RuleHomeSecret)
		if !strings.Contains(sbpl, want) {
			t.Errorf("the profile must carry %s", want)
		}
	}
}

func TestReadIsolation_ACellKeepsTheSignInOfItsOwnBinaryOutOfTheSecretList(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	for _, s := range widerSecrets {
		path := filepath.Join(c.home, s.rel)
		if s.dir {
			path = filepath.Join(path, "fixture")
		}
		writeFixtureFile(t, path, "secret\n")
	}
	for cli, binary := range map[string]string{"codex-tmux": "codex", "codex": "codex", "agy-tmux": "agy", "agy-claude-tmux": "agy", "claude-tmux": "claude"} {
		req := c.request()
		req.CLI = cli
		ri := Deps{ReadStage: "enforce"}.readIsolation(req, c.home)
		listed := slices.Concat(ri.SecretDirs, ri.SecretFiles)
		for _, s := range widerSecrets {
			held := slices.Contains(listed, filepath.Join(c.home, s.rel))
			if held == (s.owner == binary) {
				t.Errorf("a %s cell: secret %s listed = %v, owner %q", cli, s.rel, held, s.owner)
			}
		}
	}
}
