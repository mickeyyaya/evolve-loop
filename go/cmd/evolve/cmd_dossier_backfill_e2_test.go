package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func writeE2File(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func e2PendingDirAsFile(t *testing.T, root string) {
	t.Helper()
	writeE2File(t, dossier.PendingDir(root), "not a directory")
}

func TestRunDossier_RoutesEachSubcommandAndRefusesTheRest(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{"no subcommand prints the usage", nil, 10, "usage: dossier verify | retro-mislabel"},
		{"an empty slice prints the usage", []string{}, 10, "usage: dossier verify"},
		{"an unknown subcommand is named", []string{"frob"}, 10, `unknown subcommand "frob"`},
		{"a unicode subcommand is quoted", []string{"διαβάζω"}, 10, `unknown subcommand "διαβάζω"`},
		{"retro-mislabel is routed", []string{"retro-mislabel", "--project-root", "/nonexistent-e2-root"}, 1, "dossier retro-mislabel: read knowledge-base/cycles"},
		{"publish is routed", []string{"publish", "stray"}, 10, "dossier publish: unexpected arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := runDossier(tc.args, nil, &stdout, &stderr)

			if code != tc.wantCode || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("runDossier(%q) = %d stderr=%q, want %d containing %q", tc.args, code, stderr.String(), tc.wantCode, tc.wantStderr)
			}
			if stdout.Len() != 0 {
				t.Fatalf("a refused subcommand wrote stdout %q", stdout.String())
			}
		})
	}
}

func TestRunDossierPublish_RefusesBadInvocations(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"an unknown flag", []string{"--bogus"}, "flag provided but not defined: -bogus"},
		{"a positional argument", []string{"cycle-1"}, `unexpected arguments ["cycle-1"]`},
		{"a positional after flags", []string{"--dry-run", "a", "b"}, `unexpected arguments ["a" "b"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := runDossierPublish(tc.args, &stdout, &stderr)

			if code != 10 || !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("runDossierPublish(%q) = %d stderr=%q, want 10 containing %q", tc.args, code, stderr.String(), tc.wantStderr)
			}
		})
	}
}

func TestRunDossierPublish_AnUnreadablePendingDirExits2(t *testing.T) {
	root := t.TempDir()
	e2PendingDirAsFile(t, root)
	var stdout, stderr bytes.Buffer

	code := runDossierPublish([]string{"--project-root", root}, &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), "dossier publish: dossier: list pending closeouts") {
		t.Fatalf("code=%d stderr=%q, want 2 naming the list failure", code, stderr.String())
	}
}

func TestRunDossierPublish_NothingPendingIsAQuietSuccess(t *testing.T) {
	for _, withDir := range []bool{false, true} {
		root := t.TempDir()
		if withDir {
			if err := os.MkdirAll(dossier.PendingDir(root), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		var stdout, stderr bytes.Buffer

		code := runDossierPublish([]string{"--project-root", root}, &stdout, &stderr)

		if code != 0 || stdout.String() != "dossier publish: nothing pending to publish\n" || stderr.Len() != 0 {
			t.Fatalf("withDir=%v: code=%d stdout=%q stderr=%q", withDir, code, stdout.String(), stderr.String())
		}
	}
}

func TestRunDossierPublish_HalfPairsAloneStayPendingAndPublishNothing(t *testing.T) {
	root := t.TempDir()
	writeE2File(t, filepath.Join(dossier.PendingDir(root), "cycle-8.md"), "# eight\n")
	var stdout, stderr bytes.Buffer

	code := runDossierPublish([]string{"--project-root", root}, &stdout, &stderr)

	want := "dossier publish: half pairs [8] stay pending (a .md without its .json)\ndossier publish: nothing pending to publish\n"
	if code != 0 || stdout.String() != want {
		t.Fatalf("code=%d stdout=%q, want 0 and %q", code, stdout.String(), want)
	}
	if _, err := os.Stat(filepath.Join(dossier.PendingDir(root), "cycle-8.md")); err != nil {
		t.Fatalf("the half pair must stay: %v", err)
	}
}

func TestRunDossierPublish_OutsideACheckoutTheHoldErrorExits2(t *testing.T) {
	root := t.TempDir()
	if err := writePendingCloseout(root); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runDossierPublish([]string{"--project-root", root}, &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), "is not a git checkout") {
		t.Fatalf("code=%d stderr=%q, want 2 naming the missing checkout", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dossier.PendingDir(root), "cycle-1705.json")); err != nil {
		t.Fatalf("the pair must stay pending: %v", err)
	}
}

func TestRunDossierPublish_ADryRunWithAFreeLockListsAndChangesNothing(t *testing.T) {
	r := planeWithAPendingCloseout(t, "main")
	head := r.Git("rev-parse", "HEAD")
	var stdout, stderr bytes.Buffer

	code := runDossierPublish([]string{"--dry-run", "--project-root", r.Dir}, &stdout, &stderr)

	if code != 0 || stdout.String() != "dossier publish: dry run: would publish cycles [1705]\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Fatal("a dry run moved HEAD")
	}
	if _, err := os.Lstat(flock.ShipLockPath(r.Dir)); !os.IsNotExist(err) {
		t.Fatalf("a dry run must not create the lock file: %v", err)
	}
}

func TestRunDossierPublish_PublishesThePendingPairIntoOneCommit(t *testing.T) {
	r := planeWithAPendingCloseout(t, "main")
	var stdout, stderr bytes.Buffer

	code := runDossierPublish([]string{"--project-root", r.Dir}, &stdout, &stderr)

	if code != 0 || stdout.String() != "dossier publish: published cycles [1705]\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != closeoutCommit {
		t.Fatalf("HEAD = %q, want the closeout commit", got)
	}
	if _, err := os.Stat(filepath.Join(dossier.PendingDir(r.Dir), "cycle-1705.json")); !os.IsNotExist(err) {
		t.Fatalf("the published pair must leave the pending dir: %v", err)
	}
}

func TestPublishDossierPairs_ARefusedPairExits2AndStaysPending(t *testing.T) {
	r := gitPlane(t, "main")
	bad := filepath.Join(dossier.PendingDir(r.Dir), "cycle-9.json")
	writeE2File(t, bad, "{not json")
	head := r.Git("rev-parse", "HEAD")
	var stdout, stderr bytes.Buffer

	code := publishDossierPairs(r.Dir, &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), "dossier publish: 1 pending pairs were refused or failed and stay pending") {
		t.Fatalf("code=%d stderr=%q, want 2 counting one refused pair", code, stderr.String())
	}
	if stdout.String() != "dossier publish: published cycles []\n" {
		t.Fatalf("stdout=%q, want an empty published list", stdout.String())
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Fatal("a refused pair moved HEAD")
	}
	if _, err := os.Stat(bad); err != nil {
		t.Fatalf("the refused pair must stay pending: %v", err)
	}
}

func TestPublishDossierPairs_AListFailureExits2(t *testing.T) {
	root := t.TempDir()
	e2PendingDirAsFile(t, root)
	var stdout, stderr bytes.Buffer

	code := publishDossierPairs(root, &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), "dossier publish: dossier: list pending closeouts") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPublishDossierPairs_PublishesEveryPair(t *testing.T) {
	r := planeWithAPendingCloseout(t, "main")
	var stdout, stderr bytes.Buffer

	code := publishDossierPairs(r.Dir, &stdout, &stderr)

	if code != 0 || stdout.String() != "dossier publish: published cycles [1705]\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestHasPendingDossiers_ReportsWhatTheDirectoryHolds(t *testing.T) {
	t.Run("absent dir is silent false", func(t *testing.T) {
		var warn bytes.Buffer
		if hasPendingDossiers(t.TempDir(), &warn) || warn.Len() != 0 {
			t.Fatalf("an absent dir must be false and silent, warn=%q", warn.String())
		}
	})
	t.Run("empty dir is false", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(dossier.PendingDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		var warn bytes.Buffer
		if hasPendingDossiers(root, &warn) || warn.Len() != 0 {
			t.Fatalf("an empty dir must be false and silent, warn=%q", warn.String())
		}
	})
	t.Run("a single entry is true", func(t *testing.T) {
		root := t.TempDir()
		writeE2File(t, filepath.Join(dossier.PendingDir(root), "cycle-1.md"), "x")
		var warn bytes.Buffer
		if !hasPendingDossiers(root, &warn) {
			t.Fatal("one entry must count as pending")
		}
	})
	t.Run("an unreadable dir warns and is false", func(t *testing.T) {
		root := t.TempDir()
		e2PendingDirAsFile(t, root)
		var warn bytes.Buffer
		if hasPendingDossiers(root, &warn) {
			t.Fatal("an unreadable dir must not count as pending")
		}
		if !strings.Contains(warn.String(), "[dossier] WARN: pending dossiers: cannot check:") || !strings.Contains(warn.String(), "nothing is published this pass") {
			t.Fatalf("warn=%q, want the cannot-check warning", warn.String())
		}
	})
}

func TestPublishPendingDossiers_APendingDirThatVanishesUnderTheLockWarnsAndKeepsHead(t *testing.T) {
	r := planeWithAPendingCloseout(t, "main")
	previous := dossierPublishLock
	dossierPublishLock = func(root string) (func(), error) {
		if err := os.RemoveAll(dossier.PendingDir(root)); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dossier.PendingDir(root), []byte("swapped"), 0o644); err != nil {
			return nil, err
		}
		return func() {}, nil
	}
	t.Cleanup(func() { dossierPublishLock = previous })
	head := r.Git("rev-parse", "HEAD")
	var warn bytes.Buffer

	publishPendingDossiers(r.Dir, &warn)

	if !strings.Contains(warn.String(), "[dossier] WARN: pending dossiers: dossier: list pending closeouts") || !strings.Contains(warn.String(), "they stay pending") {
		t.Fatalf("warn=%q, want the publish error reported", warn.String())
	}
	if r.Git("rev-parse", "HEAD") != head {
		t.Fatal("a failed publish moved HEAD")
	}
}

func TestTryDossierPublishLock_HoldsTheShipLockUntilReleased(t *testing.T) {
	root := t.TempDir()

	release, err := tryDossierPublishLock(root)
	if err != nil || release == nil {
		t.Fatalf("a free lock must be taken: release=%v err=%v", release != nil, err)
	}
	_, second := tryDossierPublishLock(root)
	if second == nil || !strings.Contains(second.Error(), "is held by another process") || !strings.Contains(second.Error(), flock.ShipLockPath(root)) {
		t.Fatalf("a second take must name the held lock, got %v", second)
	}
	release()
	again, err := tryDossierPublishLock(root)
	if err != nil {
		t.Fatalf("a released lock must be free again: %v", err)
	}
	again()
}

func TestTryDossierPublishLock_AnUncreatableLockDirIsAnError(t *testing.T) {
	root := t.TempDir()
	writeE2File(t, filepath.Join(root, ".evolve"), "a file where the dir goes")

	release, err := tryDossierPublishLock(root)

	if err == nil || release != nil {
		t.Fatalf("an uncreatable lock dir must fail, release=%v err=%v", release != nil, err)
	}
	if strings.Contains(err.Error(), "held by another process") {
		t.Fatalf("an I/O failure must not read as a held lock: %v", err)
	}
}

func TestProbeDossierPublishLock_ProbesOnlyAnExistingLockFile(t *testing.T) {
	t.Run("no lock file is free and creates nothing", func(t *testing.T) {
		root := t.TempDir()
		release, err := probeDossierPublishLock(root)
		if err != nil || release == nil {
			t.Fatalf("release=%v err=%v", release != nil, err)
		}
		release()
		if _, err := os.Lstat(flock.ShipLockPath(root)); !os.IsNotExist(err) {
			t.Fatalf("the probe created the lock file: %v", err)
		}
	})
	t.Run("an existing free lock file is taken", func(t *testing.T) {
		root := t.TempDir()
		writeE2File(t, flock.ShipLockPath(root), "")
		release, err := probeDossierPublishLock(root)
		if err != nil || release == nil {
			t.Fatalf("release=%v err=%v", release != nil, err)
		}
		_, second := tryDossierPublishLock(root)
		if second == nil {
			t.Fatal("the probe must hold the existing lock until released")
		}
		release()
	})
	t.Run("an existing held lock file is refused", func(t *testing.T) {
		root := t.TempDir()
		held, err := tryDossierPublishLock(root)
		if err != nil {
			t.Fatal(err)
		}
		defer held()
		_, perr := probeDossierPublishLock(root)
		if perr == nil || !strings.Contains(perr.Error(), "is held by another process") {
			t.Fatalf("the probe must report the held lock, got %v", perr)
		}
	})
}

func TestRunDossier_RoutesVerifyToTheVerifier(t *testing.T) {
	root := t.TempDir()
	writeDossierFloorPolicy(t, root)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var viaRouter, viaRouterErr, direct, directErr bytes.Buffer

	routed := runDossier([]string{"verify"}, nil, &viaRouter, &viaRouterErr)
	want := runDossierVerify(nil, &direct, &directErr)

	if routed == 0 || routed != want || viaRouterErr.String() != directErr.String() {
		t.Fatalf("routed=%d stderr=%q, want the verifier's own refusal %d %q", routed, viaRouterErr.String(), want, directErr.String())
	}
}
