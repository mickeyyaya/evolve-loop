package recoveryguard

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// An unfenced path is that path, or a directory and what is below it; a name that merely begins the same is a
// planted file and is fenced.
func TestGuard_UnfencedPathsHaveABoundary(t *testing.T) {
	ws := workspace(t)
	signals := filepath.Join(ws, "signals.ndjson")
	planted := filepath.Join(ws, "signals-evil.json")
	logs := filepath.Join(ws, "logs")
	sibling := filepath.Join(ws, "logs-evil", "payload.json")
	write(t, filepath.Join(logs, "run.log"), "one\n")

	out := run(t, Scope{Workspace: ws, Unfenced: []string{signals, logs}}, func() {
		write(t, signals, "{\"seq\":1}\n{\"seq\":2}\n")
		write(t, filepath.Join(logs, "run.log"), "one\ntwo\n")
		write(t, planted, "{\"verdict\":\"PASS\"}")
		write(t, sibling, "{\"verdict\":\"PASS\"}")
	})

	for _, p := range []string{planted, sibling} {
		if !slices.Contains(out.Restored, p) {
			t.Errorf("%s shares an unfenced name's prefix and must be restored, got %+v", p, out)
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("%s survived", p)
		}
	}
	if read(t, signals) != "{\"seq\":1}\n{\"seq\":2}\n" || read(t, filepath.Join(logs, "run.log")) != "one\ntwo\n" {
		t.Fatal("unfenced telemetry was rolled back")
	}
}

// The dispatch creates artifacts with generated names (its sandbox profile dir, its own logs); they survive by
// stem and are reported, never silently tolerated.
func TestGuard_ToleratesTheDispatchesGeneratedArtifactsByStemAndReportsThem(t *testing.T) {
	ws := workspace(t)
	write(t, filepath.Join(ws, "sbprofile-1", "sandbox.sb"), "(version 1)\n")
	profile := filepath.Join(ws, "sbprofile-2")
	log := filepath.Join(ws, "deliverable-recovery-interactions.ndjson")
	nested := filepath.Join(ws, "sub", "sbprofile-3")

	out := run(t, Scope{Workspace: ws, UnfencedStems: []string{"sbprofile-", "deliverable-recovery-"}}, func() {
		write(t, filepath.Join(profile, "sandbox.sb"), "(version 1)\n")
		write(t, log, "{\"kind\":\"nudge\"}\n")
		write(t, nested, "planted\n")
	})

	if !slices.Equal(out.Unfenced, []string{log, profile}) {
		t.Fatalf("the new generated artifacts must be reported: %+v", out)
	}
	if !slices.Equal(out.Restored, []string{nested}) || out.Err != nil {
		t.Fatalf("a stem tolerates direct children only; got %+v", out)
	}
	if _, err := os.Stat(filepath.Join(profile, "sandbox.sb")); err != nil {
		t.Fatalf("the tolerated profile dir was rolled back: %v", err)
	}
}

// An allowed path may be written, not turned into a link: every later reader would follow it.
func TestGuard_UndoesAnAllowedPathSwappedForALink(t *testing.T) {
	ws := workspace(t)
	report := filepath.Join(ws, "build-report.md")

	out := run(t, Scope{Workspace: ws, Allowed: []string{report}}, func() {
		if err := os.Remove(report); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(ws, "lane-scope.json"), report); err != nil {
			t.Fatal(err)
		}
	})

	if !slices.Contains(out.Restored, report) {
		t.Fatalf("the swapped allowed path must be reported, got %+v", out)
	}
	if _, err := os.Lstat(report); !os.IsNotExist(err) {
		t.Fatalf("the link must be removed, lstat err = %v", err)
	}
	if read(t, filepath.Join(ws, "lane-scope.json")) != `{"todo_ids":["one"]}` {
		t.Fatal("the link's target was touched")
	}
}

func TestBegin_RefusesADirectoryAsAnAllowedPath(t *testing.T) {
	ws := workspace(t)
	dir := filepath.Join(ws, "reports")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Begin(context.Background(), Scope{Workspace: ws, Allowed: []string{dir}}); err == nil {
		t.Fatal("a directory in Allowed would unfence its whole subtree; Begin must refuse it")
	}
}

// A link at an allowed path could already point outside every fence; the guard refuses to start over it.
func TestBegin_RefusesALinkAsAnAllowedPath(t *testing.T) {
	ws := workspace(t)
	link := filepath.Join(ws, "build-report.md")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/hosts", link); err != nil {
		t.Fatal(err)
	}

	if _, err := Begin(context.Background(), Scope{Workspace: ws, Allowed: []string{link}}); err == nil {
		t.Fatal("an allowed path that is a link must refuse the guard")
	}
}
