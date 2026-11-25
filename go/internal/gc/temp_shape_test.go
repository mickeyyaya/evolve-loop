package gc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestTempShape_UnregisteredPatternsAndRepositoryGuard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"a/unreg.go": `package a

import myos "os"

func f(x string) {
	_, _ = myos.MkdirTemp("", ("zzbad-"))
	_, _ = myos.MkdirTemp("", "zzpre-"+x+"-")
	_, _ = myos.MkdirTemp("", "a-"+"b-")
	_, _ = myos.MkdirTemp("", "evcodexhome")
	_, _ = myos.MkdirTemp("", x)
	_, _ = myos.MkdirTemp("", 123)
	_, _ = myos.MkdirTemp("", x == "a")
	_, _ = other.MkdirTemp("", "zzignored-other-")
	_, _ = makeFS().MkdirTemp("", "zzignored-call-")
}
`,
		"a/ok_test.go": `package a

import "os"

func g() {
	_, _ = os.MkdirTemp("", "buildfloor-cover-*")
	_, _ = os.MkdirTemp("", "evghconfig")
}
`,
	}
	for rel, body := range files {
		writeFile(t, filepath.Join(root, rel), body)
	}

	got, err := UnregisteredTempPatterns(root)
	if err != nil {
		t.Fatalf("UnregisteredTempPatterns(%s): %v", root, err)
	}
	want := []string{"a-b-", "zzbad-", "zzpre-"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnregisteredTempPatterns = %v, want %v", got, want)
	}

	pkgGot, err := UnregisteredTempPatterns(".")
	if err != nil {
		t.Fatalf("UnregisteredTempPatterns(internal/gc): %v", err)
	}
	if len(pkgGot) != 0 {
		t.Errorf("unregistered os.MkdirTemp patterns in internal/gc: %v", pkgGot)
	}

	if _, err := UnregisteredTempPatterns(filepath.Join(root, "missing")); err == nil {
		t.Error("expected error for missing root")
	}

	badParse := t.TempDir()
	writeFile(t, filepath.Join(badParse, "broken.go"), "package broken\nfunc (")
	if _, err := UnregisteredTempPatterns(badParse); err == nil {
		t.Error("expected error for invalid Go source")
	}
}

func TestTempShape_HasLiveSocketHandlesWalkErrors(t *testing.T) {
	t.Parallel()
	if hasLiveSocket(filepath.Join(t.TempDir(), "missing")) {
		t.Error("missing root reported live socket")
	}
}

func TestTempShape_LiveSocketStaysAndStaleSocketGoes(t *testing.T) {
	t.Parallel()
	base, err := os.MkdirTemp("/tmp", "cycle2631-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })

	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-48 * time.Hour)

	liveDir := filepath.Join(base, "evcodexhome1111111111")
	staleDir := filepath.Join(base, "evcodexhome2222222222")
	for _, d := range []string{liveDir, staleDir} {
		writeFile(t, filepath.Join(d, "payload"), "x")
	}

	liveLn, err := net.Listen("unix", filepath.Join(liveDir, "s"))
	if err != nil {
		t.Fatalf("listen live socket: %v", err)
	}
	defer func() { _ = liveLn.Close() }()

	staleLn, err := net.Listen("unix", filepath.Join(staleDir, "s"))
	if err != nil {
		t.Fatalf("listen stale socket: %v", err)
	}
	staleLn.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := staleLn.Close(); err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{liveDir, staleDir} {
		if err := os.Chtimes(d, stale, stale); err != nil {
			t.Fatal(err)
		}
	}

	chk := tempCheck{
		dir: base,
		now: func() time.Time { return now },
		held: func(context.Context) func(string) (bool, error) {
			return func(string) (bool, error) { return false, nil }
		},
	}
	items, err := chk.Scan(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	actions := map[string]string{}
	for _, it := range items {
		actions[filepath.Base(it.Subject)] = it.Action
	}
	if actions[filepath.Base(liveDir)] != string(tempActionSpareHeld) {
		t.Errorf("live socket dir action = %q, want %q", actions[filepath.Base(liveDir)], tempActionSpareHeld)
	}
	if actions[filepath.Base(staleDir)] != string(tempActionRemove) {
		t.Errorf("stale socket dir action = %q, want %q", actions[filepath.Base(staleDir)], tempActionRemove)
	}

	if err := chk.Apply(context.Background(), CheckContext{Policy: Policy{TempTTLHours: 24}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(liveDir); err != nil {
		t.Errorf("Apply removed live socket dir: %v", err)
	}
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Errorf("Apply left stale socket dir behind: %v", err)
	}
}

func TestTempShape_EvIsolatedAndLookalikes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-48 * time.Hour)
	fresh := now.Add(-1 * time.Hour)

	cases := map[string]struct {
		mtime time.Time
		gone  bool
	}{
		"evcodexhome1234567890":      {stale, true},
		"evcodeusage1234567890":      {stale, true},
		"evtmux1234567890":           {stale, true},
		"evghconfig1234567890":       {stale, true},
		"buildfloor-cover-987654321": {stale, true},
		"evcodexhome0000000000":      {fresh, false},
		"evcodexhomeKeepMe":          {stale, false},
	}
	for name, tc := range cases {
		p := filepath.Join(dir, name)
		writeFile(t, filepath.Join(p, "payload"), "x")
		if err := os.Chtimes(p, tc.mtime, tc.mtime); err != nil {
			t.Fatal(err)
		}
	}

	rep := ReapPipelineTemp(dir, now.Add(-24*time.Hour), os.RemoveAll, SparingHeld(func(string) (bool, error) { return false, nil }))
	if rep.Entries != 5 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v, want 5 entries and 0 errors", rep)
	}
	for name, tc := range cases {
		_, err := os.Stat(filepath.Join(dir, name))
		if gone := os.IsNotExist(err); gone != tc.gone {
			t.Errorf("%s: gone=%v, want %v", name, gone, tc.gone)
		}
	}
}
