package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGuard_EachNamedGuardDispatchesAndAnUnknownNameListsThemAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range Names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var out, errb bytes.Buffer

			RunGuard([]string{"--evolve-dir", dir, name}, strings.NewReader(`{}`), &out, &errb)

			if strings.Contains(errb.String(), "unknown guard") {
				t.Fatalf("guard %q is listed in Names but the dispatcher does not know it: %s", name, errb.String())
			}
		})
	}
	var out, errb bytes.Buffer

	rc := RunGuard([]string{"--evolve-dir", dir, "quota"}, strings.NewReader(`{}`), &out, &errb)

	if want := "(known: " + strings.Join(Names, " ") + ")"; rc != 10 || !strings.Contains(errb.String(), want) {
		t.Fatalf("rc=%d stderr=%q, want rc 10 and %q", rc, errb.String(), want)
	}
}

func TestRunGuard_DocDeleteWithAnUnreadablePolicyIsAUsageErrorThatNamesThePolicy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	rc := RunGuard([]string{"--evolve-dir", dir, "docdelete"}, strings.NewReader(`{}`), &out, &errb)

	if rc != 10 || !strings.Contains(errb.String(), "policy") {
		t.Fatalf("rc=%d stderr=%q, want rc 10 and an error that names the policy", rc, errb.String())
	}
}
