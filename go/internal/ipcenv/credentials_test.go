package ipcenv

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestCredentialIsolation_IsTheOneSet(t *testing.T) {
	t.Parallel()
	got := CredentialIsolation("/ws/gh", "/ws/gitconfig")
	want := map[string]string{
		"GIT_CONFIG_GLOBAL":   "/ws/gitconfig",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_SSH_COMMAND":     "ssh -o BatchMode=yes",
		"GH_CONFIG_DIR":       "/ws/gh",
	}
	if len(got.Set) != len(want) {
		t.Fatalf("Set = %v, want %v", got.Set, want)
	}
	for k, v := range want {
		if got.Set[k] != v {
			t.Errorf("Set[%s] = %q, want %q", k, got.Set[k], v)
		}
	}
	if !slices.Equal(got.Unset, []string{"GH_TOKEN", "GITHUB_TOKEN"}) {
		t.Errorf("Unset = %v", got.Unset)
	}
}

func TestCredentialIsolation_SetsNoCommitIdentityVariable(t *testing.T) {
	t.Parallel()
	for k := range CredentialIsolation("/ws/gh", os.DevNull).Set {
		if strings.HasPrefix(k, "GIT_AUTHOR_") || strings.HasPrefix(k, "GIT_COMMITTER_") {
			t.Errorf("%s would beat every git config scope", k)
		}
	}
}

func TestGitIdentityConfig_HoldsOnlyTheUserSection(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ name, email, want string }{
		"both":       {"Ada", "ada@example.test", "[user]\n\tname = \"Ada\"\n\temail = \"ada@example.test\"\n"},
		"name only":  {"Ada", "", "[user]\n\tname = \"Ada\"\n"},
		"email only": {"", "ada@example.test", "[user]\n\temail = \"ada@example.test\"\n"},
		"none":       {"", "", ""},
		"escaped":    {"A \"B\" \\ C\nD", "", "[user]\n\tname = \"A \\\"B\\\" \\\\ C\\nD\"\n"},
	}
	for name, c := range cases {
		if got := GitIdentityConfig(c.name, c.email); got != c.want {
			t.Errorf("%s: GitIdentityConfig = %q, want %q", name, got, c.want)
		}
		if strings.Contains(GitIdentityConfig(c.name, c.email), "credential") {
			t.Errorf("%s: the file holds a credential key", name)
		}
	}
}

func TestCredentialSet_EnvIsSortedKeyValuePairs(t *testing.T) {
	t.Parallel()
	got := CredentialSet{Set: map[string]string{"B": "2", "A": "1"}}.Env()
	if !slices.Equal(got, []string{"A=1", "B=2"}) {
		t.Fatalf("Env = %q", got)
	}
}
