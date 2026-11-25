//go:build acs

// Package cycle2631 encodes the acceptance of the recycle-x7-temp-shape-registry inbox item: the temp reaper takes shapes from one registry owned by gc, a guard fails on an unregistered os.MkdirTemp pattern, a dir with a live socket stays, and an evcodexhome dir past the horizon with no holder goes.
package cycle2631

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func privateBase(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("/tmp", "cycle2631-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	return base
}

var (
	pastTheHorizon   = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	horizon          = time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	insideTheHorizon = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
)

func setMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func staleDir(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "payload"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	setMtime(t, dir, pastTheHorizon)
	return dir
}

func noHolder(string) (bool, error) { return false, nil }

func reapPastTheHorizon(base string, held func(string) (bool, error)) gc.TempReapReport {
	return gc.ReapPipelineTemp(base, horizon, os.RemoveAll, gc.SparingHeld(held))
}

func listenOnSocket(t *testing.T, dir string) *net.UnixListener {
	t.Helper()
	ln, err := net.Listen("unix", filepath.Join(dir, "s"))
	if err != nil {
		t.Fatalf("listen on a unix socket in %s: %v", dir, err)
	}
	return ln.(*net.UnixListener)
}

func TestC2631_001_GuardFailsOnAnUnregisteredMkdirTempPattern(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"pkg/a.go": `package pkg

import "os"

func Make(name string) {
	_, _ = os.MkdirTemp("", "zzunregistered-")
	_, _ = os.MkdirTemp("", "zzconcat-"+name+"-")
	_, _ = os.MkdirTemp("", "evcodexhome")
	_, _ = os.MkdirTemp("", "cycle1799-"+name+"-")
}
`,
		"pkg/b_test.go": `package pkg

import "os"

func helper() {
	_, _ = os.MkdirTemp("/tmp", "zzintest")
}
`,
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := gc.UnregisteredTempPatterns(root)
	if err != nil {
		t.Fatalf("UnregisteredTempPatterns(%s): %v", root, err)
	}

	for _, want := range []string{"zzunregistered-", "zzconcat-", "zzintest"} {
		if !slices.Contains(got, want) {
			t.Errorf("RED: pattern %q has no registered shape but the guard did not report it; got %v", want, got)
		}
	}
	for _, registered := range []string{"evcodexhome", "cycle1799-"} {
		if slices.Contains(got, registered) {
			t.Errorf("a registered pattern %q was reported as unregistered: %v", registered, got)
		}
	}
}

func TestC2631_002_GuardPassesWhenEveryPatternIsRegistered(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"pkg/a.go": `package pkg

import "os"

func Make() {
	_, _ = os.MkdirTemp("", "evcodexhome")
	_, _ = os.MkdirTemp("", "evcodeusage")
	_, _ = os.MkdirTemp("/tmp", "evtmux")
	_, _ = os.MkdirTemp("", "buildfloor-cover-*")
}
`,
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := gc.UnregisteredTempPatterns(root)
	if err != nil {
		t.Fatalf("UnregisteredTempPatterns(%s): %v", root, err)
	}
	if len(got) != 0 {
		t.Errorf("every pattern here is a shape the repository creates and the registry must hold, got unregistered %v", got)
	}
}

func TestC2631_003_EveryMkdirTempPatternOfTheRepositoryHasARegisteredShape(t *testing.T) {
	got, err := gc.UnregisteredTempPatterns(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatalf("scan the repository: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("RED: os.MkdirTemp patterns with no registered shape in go/internal/gc/tempdir.go: %v", got)
	}
}

func TestC2631_004_ADirWithALiveSocketStays(t *testing.T) {
	base := privateBase(t)
	live := staleDir(t, base, "TestLiveSocket1234567890")
	ln := listenOnSocket(t, live)
	defer ln.Close()
	setMtime(t, live, pastTheHorizon)

	rep := reapPastTheHorizon(base, noHolder)

	if !acsassert.PathExists(live) {
		t.Errorf("RED: a dir past the horizon with a listening socket was removed; report %+v", rep)
	}
	if rep.Entries != 0 {
		t.Errorf("report = %+v, want nothing reaped", rep)
	}
}

func TestC2631_005_ADirWithAStaleSocketFileGoes(t *testing.T) {
	base := privateBase(t)
	stale := staleDir(t, base, "TestStaleSocket1234567890")
	ln := listenOnSocket(t, stale)
	ln.SetUnlinkOnClose(false)
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if !acsassert.PathExists(filepath.Join(stale, "s")) {
		t.Fatal("the fixture needs a socket file with no listener")
	}
	setMtime(t, stale, pastTheHorizon)

	reapPastTheHorizon(base, noHolder)

	if acsassert.PathExists(stale) {
		t.Error("a dir whose socket has no listener must go: a dead socket file is no holder")
	}
}

func TestC2631_006_AnEvcodexhomeDirPastTheHorizonWithNoHolderGoes(t *testing.T) {
	base := privateBase(t)
	gone := staleDir(t, base, "evcodexhome3141592653")

	rep := reapPastTheHorizon(base, noHolder)

	if acsassert.PathExists(gone) {
		t.Errorf("RED: an evcodexhome dir past the horizon with no holder stayed; report %+v", rep)
	}
	if rep.Entries != 1 {
		t.Errorf("report = %+v, want exactly the one evcodexhome dir reaped", rep)
	}
}

func TestC2631_007_AnEvcodexhomeDirThatIsHeldOrFreshStays(t *testing.T) {
	base := privateBase(t)
	held := staleDir(t, base, "evcodexhome2718281828")
	fresh := filepath.Join(base, "evcodexhome1618033988")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	setMtime(t, fresh, insideTheHorizon)
	holdsOnlyTheOldOne := func(path string) (bool, error) { return path == held, nil }

	rep := reapPastTheHorizon(base, holdsOnlyTheOldOne)

	if !acsassert.PathExists(held) {
		t.Errorf("an evcodexhome dir with a live holder was removed; report %+v", rep)
	}
	if !acsassert.PathExists(fresh) {
		t.Errorf("an evcodexhome dir newer than the horizon was removed; report %+v", rep)
	}
	if rep.Entries != 0 || rep.Held != 1 {
		t.Errorf("report = %+v, want 0 reaped and 1 held", rep)
	}
}

func TestC2631_008_ADirOfNoRegisteredShapeStaysWhateverItsAge(t *testing.T) {
	base := privateBase(t)
	foreign := staleDir(t, base, "someones-notes-3141592653")
	lookalike := staleDir(t, base, "evcodexhomeKeepMe")

	reapPastTheHorizon(base, noHolder)

	for _, dir := range []string{foreign, lookalike} {
		if !acsassert.PathExists(dir) {
			t.Errorf("%s matches no registered shape and must stay", dir)
		}
	}
}
