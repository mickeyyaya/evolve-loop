package sandbox

import (
	"path"
	"slices"
	"strings"
	"testing"
)

func TestHomeSecrets_AreRelativeLowercaseCleanAndUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, rel := range HomeSecrets {
		if rel == "" || rel != strings.ToLower(rel) || rel != path.Clean(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "..") || seen[rel] {
			t.Errorf("home secret %q must be a unique, clean, lowercase path below HOME", rel)
		}
		seen[rel] = true
	}
}

func TestHomeSecrets_NameEachCredentialStoreOfTheWiderList(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		".ssh", ".aws", ".gnupg", ".netrc", ".config/gh", ".docker/config.json", ".kube", ".git-credentials", ".config/git/credentials",
		".config/gcloud", ".azure", ".npmrc", ".pypirc", ".cargo/credentials.toml", ".cargo/credentials",
		".codex/auth.json", ".gemini/oauth_creds.json", ".gemini/google_accounts.json",
	} {
		if !slices.Contains(HomeSecrets, want) {
			t.Errorf("HomeSecrets must name %s", want)
		}
	}
}

func TestReadIsolation_ASecretPathIsDeniedForWriteAsWellAsRead(t *testing.T) {
	t.Parallel()
	ri := ReadIsolation{Workspace: "/w", Tag: "d42", SecretDirs: []string{"/home/u/.azure"}, SecretFiles: []string{"/home/u/.npmrc"}}
	var b strings.Builder
	writeReadIsolation(&b, ri)
	got := b.String()
	for _, want := range []string{
		`(deny file-write* (subpath "/home/u/.azure") (with message "evolve-cell=d42 rule=read.home.secret inv=I5"))`,
		`(deny file-write* (literal "/home/u/.npmrc") (with message "evolve-cell=d42 rule=read.home.secret inv=I5"))`,
		`(deny file-read-data (subpath "/home/u/.azure")`,
		`(deny file-read-data (literal "/home/u/.npmrc")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the secret guard must emit %s:\n%s", want, got)
		}
	}
}
