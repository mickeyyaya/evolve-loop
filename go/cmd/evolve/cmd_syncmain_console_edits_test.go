package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const smInboxRel = ".evolve/inbox/"

func smTrackedInbox(t *testing.T, items map[string]string) (repo, bare string) {
	t.Helper()
	repo, bare = smInitRepoWithRemote(t)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".evolve/*\n!.evolve/inbox/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smWriteInbox(t, repo, items)
	smGit(t, repo, "add", "-f", ".gitignore", ".evolve/inbox")
	smGit(t, repo, "commit", "-q", "-m", "track the inbox")
	smGit(t, repo, "push", "-q", "origin", "main")
	return repo, bare
}

func smWriteInbox(t *testing.T, repo string, items map[string]string) {
	t.Helper()
	for rel, body := range items {
		path := filepath.Join(repo, smInboxRel, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func smItemFields(t *testing.T, repo, rel string) map[string]any {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal([]byte(smRead(t, filepath.Join(repo, smInboxRel, rel))), &fields); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return fields
}

func smJSONObject(t *testing.T, body string) map[string]any {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func smSync(t *testing.T, repo string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runSyncMain([]string{"--project-root", repo}, nil, &out, &errb)
	return code, out.String(), errb.String()
}

func smLinesAbout(output string, needles ...string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		matches := true
		for _, needle := range needles {
			matches = matches && strings.Contains(line, needle)
		}
		if matches {
			lines = append(lines, line)
		}
	}
	return lines
}

func smTracked(t *testing.T, repo string) []string {
	t.Helper()
	return strings.Fields(smGit(t, repo, "ls-files"))
}

func TestSyncMain_CarriesAConsoleRouteEditThroughTheMerge(t *testing.T) {
	t.Parallel()
	repo, bare := smTrackedInbox(t, map[string]string{
		"a.json": `{"id":"a","title":"A","weight":0.5}`,
		"b.json": `{"id":"b","title":"B","weight":0.5}`,
	})
	smOriginWrites(t, bare, ".evolve/inbox/a.json", `{"id":"a","title":"A at origin","weight":0.5}`)
	routedByConsole := func(id, title string) string {
		return `{"id":"` + id + `","title":"` + title + `","weight":0.5,"route":"console-manual","routed_reason":"console owns the protected surface","routed_at":"2026-11-20T09:00:00Z","routed_by":"operator","deps":["blocker-x"]}`
	}
	smWriteInbox(t, repo, map[string]string{"a.json": routedByConsole("a", "A"), "b.json": routedByConsole("b", "B")})

	code, stdout, stderr := smSync(t, repo)

	if code != 0 {
		t.Fatalf("sync-main = %d, want the console's route-console edit carried through the merge\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if got := smPorcelain(t, repo); strings.TrimSpace(got) != "" {
		t.Errorf("tree after sync = %q, want clean", got)
	}
	for rel, want := range map[string]string{
		"a.json": routedByConsole("a", "A at origin"),
		"b.json": routedByConsole("b", "B"),
	} {
		if got := smItemFields(t, repo, rel); !reflect.DeepEqual(got, smJSONObject(t, want)) {
			t.Errorf("%s = %v, want origin's fields with the console's route and blocker: %s", rel, got, want)
		}
	}
	for _, said := range []string{
		"replayed 1 inbox stamp(s) onto origin's edits: .evolve/inbox/a.json",
		"committed 1 inbox stamp(s) the loop wrote: .evolve/inbox/b.json",
	} {
		if !strings.Contains(stdout, said) {
			t.Errorf("stdout = %q, want it to say %q", stdout, said)
		}
	}
}

func smConsumeMove(t *testing.T, repo, name, consumedBody string) {
	t.Helper()
	if err := os.Remove(filepath.Join(repo, smInboxRel, name)); err != nil {
		t.Fatal(err)
	}
	smWriteInbox(t, repo, map[string]string{"consumed/" + name: consumedBody})
}

func TestSyncMain_CarriesAConsoleConsumeMove(t *testing.T) {
	t.Parallel()
	body := `{"id":"c","title":"C","kind":"bug","weight":0.5}`
	repo, bare := smTrackedInbox(t, map[string]string{"c.json": body})
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	consumed := `{"id":"c","title":"C","kind":"bug","weight":0.5,"consumed":{"at":"2026-11-20T10:00:00Z","via":"console-manual","cycle":"console","resolution":"fixed by hand"}}`
	smConsumeMove(t, repo, "c.json", consumed)

	code, stdout, stderr := smSync(t, repo)

	if code != 0 {
		t.Fatalf("sync-main = %d, want the console's consume move carried through the merge\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if got := smPorcelain(t, repo); strings.TrimSpace(got) != "" {
		t.Errorf("tree after sync = %q, want clean", got)
	}
	tracked := smTracked(t, repo)
	for _, want := range []string{".evolve/inbox/consumed/c.json", "remote-change.txt"} {
		if !slices.Contains(tracked, want) {
			t.Errorf("tracked files %v, want %s", tracked, want)
		}
	}
	if slices.Contains(tracked, ".evolve/inbox/c.json") {
		t.Errorf("tracked files %v, want the pending copy gone", tracked)
	}
	if got := smRead(t, filepath.Join(repo, smInboxRel, "consumed", "c.json")); got != consumed {
		t.Errorf("consumed/c.json = %s, want the console's record untouched", got)
	}
}

func TestSyncMain_AConsumeMoveThatAlsoRewritesAnAuthoredFieldStillRefuses(t *testing.T) {
	t.Parallel()
	repo, _ := smTrackedInbox(t, map[string]string{"c.json": `{"id":"c","kind":"bug","weight":0.5}`})
	rewritten := `{"id":"c","kind":"feature","weight":0.5,"consumed":{"at":"2026-11-20T10:00:00Z","via":"console-manual","cycle":"console"}}`
	smConsumeMove(t, repo, "c.json", rewritten)
	headBefore := smHead(t, repo)

	code, stdout, stderr := smSync(t, repo)

	if code == 0 || !strings.Contains(stderr, ".evolve/inbox/c.json") {
		t.Errorf("sync-main = %d (stdout %q, stderr %q), want a refusal naming the item: a move that rewrites kind is not the console's consume", code, stdout, stderr)
	}
	if got := smHead(t, repo); got != headBefore {
		t.Errorf("HEAD moved from %s to %s: a refused sync records nothing", headBefore, got)
	}
	if got := smRead(t, filepath.Join(repo, smInboxRel, "consumed", "c.json")); got != rewritten {
		t.Errorf("consumed/c.json = %s, want the console's copy untouched", got)
	}
}

func TestSyncMain_RemovesUntrackedCopiesIdenticalToOrigin(t *testing.T) {
	t.Parallel()
	t.Run("an identical copy is removed so the merge can land", func(t *testing.T) {
		t.Parallel()
		repo, bare := smInitRepoWithRemote(t)
		smRemoteCommit(t, bare, "incoming.txt", "from origin\n")
		local := filepath.Join(repo, "incoming.txt")
		if err := os.WriteFile(local, []byte("from origin\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		code, stdout, stderr := smSync(t, repo)

		if code != 0 {
			t.Fatalf("sync-main = %d, want success: the untracked copy equals origin's file\nstdout=%s\nstderr=%s", code, stdout, stderr)
		}
		if got := smRead(t, local); got != "from origin\n" {
			t.Errorf("incoming.txt = %q, want origin's content", got)
		}
		if got := smPorcelain(t, repo); strings.TrimSpace(got) != "" {
			t.Errorf("tree after sync = %q, want clean: the file is tracked now", got)
		}
		if !strings.Contains(stdout, "incoming.txt") {
			t.Errorf("stdout = %q, want it to name the untracked copy it removed", stdout)
		}
	})
	t.Run("a differing copy is kept and the merge refuses", func(t *testing.T) {
		t.Parallel()
		repo, bare := smInitRepoWithRemote(t)
		smRemoteCommit(t, bare, "incoming.txt", "from origin\n")
		local := filepath.Join(repo, "incoming.txt")
		if err := os.WriteFile(local, []byte("operator's own copy\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		headBefore := smHead(t, repo)

		code, stdout, stderr := smSync(t, repo)

		if code == 0 {
			t.Fatalf("sync-main = 0, want a refusal: the untracked copy differs from origin's file\nstdout=%s", stdout)
		}
		if got := smRead(t, local); got != "operator's own copy\n" {
			t.Errorf("incoming.txt = %q, want the differing copy kept", got)
		}
		if got := smHead(t, repo); got != headBefore {
			t.Errorf("HEAD moved from %s to %s on a refusal (stderr %q)", headBefore, got, stderr)
		}
	})
}

func TestSyncMain_ASameFieldConflictKeepsOriginAndNamesTheDroppedConsoleEdit(t *testing.T) {
	t.Parallel()
	repo, bare := smTrackedInbox(t, map[string]string{
		"x.json": `{"id":"x","title":"X","weight":0.5}`,
		"y.json": `{"id":"y","title":"Y","weight":0.5}`,
	})
	smOriginWrites(t, bare, ".evolve/inbox/x.json", `{"id":"x","title":"X","weight":0.9}`)
	smOriginWrites(t, bare, ".evolve/inbox/y.json", `{"id":"y","title":"Y","weight":0.9}`)
	smWriteInbox(t, repo, map[string]string{
		"x.json": `{"id":"x","title":"X by console","weight":0.7}`,
		"y.json": `{"id":"y","title":"Y","weight":0.7}`,
	})

	code, stdout, stderr := smSync(t, repo)

	if code != 0 {
		t.Fatalf("sync-main = %d, want success with origin's value kept\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	for rel, want := range map[string]string{
		"x.json": `{"id":"x","title":"X by console","weight":0.9}`,
		"y.json": `{"id":"y","title":"Y","weight":0.9}`,
	} {
		if got := smItemFields(t, repo, rel); !reflect.DeepEqual(got, smJSONObject(t, want)) {
			t.Errorf("%s = %v, want %s: origin wins the shared weight, the console's other fields stay", rel, got, want)
		}
	}
	for _, path := range []string{".evolve/inbox/x.json", ".evolve/inbox/y.json"} {
		if len(smLinesAbout(stdout, path, "weight", "0.7")) == 0 {
			t.Errorf("stdout = %q, want a line naming %s, the dropped field weight and the dropped value 0.7", stdout, path)
		}
	}
	if len(smLinesAbout(stdout, ".evolve/inbox/x.json", "weight", "title")) != 0 {
		t.Errorf("stdout = %q, want the title edit of x.json reported as carried, never as dropped", stdout)
	}
}
