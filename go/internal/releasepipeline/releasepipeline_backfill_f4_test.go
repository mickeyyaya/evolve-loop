package releasepipeline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPreflight_RunsThePreflightLibraryWithoutStrictPassAndForwardsItsArguments(t *testing.T) {
	orig := preflightLib
	t.Cleanup(func() { preflightLib = orig })
	boom := errors.New("preflight red")
	var got struct {
		root, target              string
		dryRun, skipTests, strict bool
	}
	preflightLib = func(repoRoot, target string, dryRun, skipTests, strictPass bool) error {
		got.root, got.target, got.dryRun, got.skipTests, got.strict = repoRoot, target, dryRun, skipTests, strictPass
		return boom
	}

	err := defaultPreflight("/repo", "1.2.3", true, false)

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the library's error passed through", err)
	}
	if got.root != "/repo" || got.target != "1.2.3" || !got.dryRun || got.skipTests || got.strict {
		t.Errorf("forwarded = %+v: the default preflight is non-strict and passes dry-run and skip-tests through unchanged", got)
	}
	preflightLib = func(string, string, bool, bool, bool) error { return nil }
	if err := defaultPreflight("/repo", "1.2.3", false, true); err != nil {
		t.Errorf("a green library result is returned as nil: %v", err)
	}
}

func TestConfigReleaseSinceLastBinaryRelease_ProbeFailureStopsTheWalk(t *testing.T) {
	boom := errors.New("git diff failed")
	calls := 0
	probe := func(older, newer string) (bool, error) {
		calls++
		if calls == 2 {
			return false, boom
		}
		return false, nil
	}

	got, err := configReleaseSinceLastBinaryRelease([]string{"v3", "v2", "v1"}, probe)

	if !errors.Is(err, boom) || got != (ReleaseClassification{}) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if calls != 2 {
		t.Errorf("the walk must stop at the failing pair, probed %d times", calls)
	}
}

func TestConfigReleaseSinceLastBinaryRelease_ReportsTheNewestTagAfterTheLastBinaryChange(t *testing.T) {
	changedBetween := map[string]bool{"v1->v2": true}
	probe := func(older, newer string) (bool, error) { return changedBetween[older+"->"+newer], nil }

	got, err := configReleaseSinceLastBinaryRelease([]string{"v3", "v2", "v1"}, probe)

	if err != nil || got.Class != ConfigRelease || got.SinceVersion != "v2" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	none, err := configReleaseSinceLastBinaryRelease([]string{"v3", "v2", "v1"}, func(string, string) (bool, error) { return false, nil })
	if err != nil || none.SinceVersion != "v1" {
		t.Fatalf("no binary change anywhere reports the earliest tag: %+v %v", none, err)
	}
	single, err := configReleaseSinceLastBinaryRelease([]string{"v9"}, probe)
	if err != nil || single.SinceVersion != "v9" {
		t.Fatalf("a single tag is its own baseline: %+v %v", single, err)
	}
}

func TestRepinExpectedShipSHA_EncodeFailureLeavesTheStateUntouched(t *testing.T) {
	orig := repinMarshalIndent
	t.Cleanup(func() { repinMarshalIndent = orig })
	repinMarshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("encoder down") }
	state := filepath.Join(t.TempDir(), "state.json")
	const before = `{"expected_ship_sha":"old","keep":1}`
	if err := os.WriteFile(state, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	repinExpectedShipSHA(state, "9.9.9", "newsha")

	if got, _ := os.ReadFile(state); string(got) != before {
		t.Errorf("state changed despite the encode failure: %s", got)
	}
	if _, err := os.Stat(state + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("no temp file may be left: %v", err)
	}
}

func TestRepinExpectedShipSHA_RepinsOnlyWhenTheBlobChanged(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(state, []byte(`{"expected_ship_sha":"old","keep":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	repinExpectedShipSHA(state, "9.9.9", "newsha")
	body, _ := os.ReadFile(state)
	for _, want := range []string{`"expected_ship_sha": "newsha"`, `"expected_ship_version": "9.9.9"`, `"keep": 1`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("state lacks %q: %s", want, body)
		}
	}

	if err := os.WriteFile(state, []byte(`{"expected_ship_sha":"same"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repinExpectedShipSHA(state, "9.9.9", "same")
	if got, _ := os.ReadFile(state); string(got) != `{"expected_ship_sha":"same"}` {
		t.Errorf("an unchanged blob must not rewrite the file: %s", got)
	}
	repinExpectedShipSHA(filepath.Join(t.TempDir(), "absent.json"), "9.9.9", "x")
	if err := os.WriteFile(state, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	repinExpectedShipSHA(state, "9.9.9", "x")
	if got, _ := os.ReadFile(state); string(got) != "{broken" {
		t.Errorf("a malformed state file is left alone: %s", got)
	}
}
