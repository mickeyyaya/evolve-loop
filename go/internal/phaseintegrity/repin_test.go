package phaseintegrity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func seedStateJSON(t *testing.T, dir string, extra map[string]any) string {
	t.Helper()
	p := filepath.Join(dir, "state.json")
	state := map[string]any{
		"expected_ship_sha":     "OLD_SHA",
		"expected_ship_version": "21.0.0",
		"lastCycleNumber":       42,
	}
	for k, v := range extra {
		state[k] = v
	}
	b, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readState(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("torn/invalid state.json: %v\n%s", err, b)
	}
	return s
}

var trueProv = func(string) bool { return true }
var falseProv = func(string) bool { return false }

func TestRepinShipSHA_ProvenanceVerified_Repins(t *testing.T) {
	p := seedStateJSON(t, t.TempDir(), nil)
	res, err := RepinShipSHA(p, "NEW_SHA", "abc123", "21.1.1", trueProv, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Repinned || res.OldSHA != "OLD_SHA" || res.NewSHA != "NEW_SHA" || res.Authorized != "provenance" {
		t.Fatalf("unexpected result: %+v", res)
	}
	s := readState(t, p)
	if s["expected_ship_sha"] != "NEW_SHA" || s["expected_ship_version"] != "21.1.1" {
		t.Errorf("pin not updated: %+v", s)
	}
}

func TestRepinShipSHA_NoProvenance_NoAuth_Refuses(t *testing.T) {
	p := seedStateJSON(t, t.TempDir(), nil)
	res, err := RepinShipSHA(p, "NEW_SHA", "evilcommit", "21.1.1", falseProv, false)
	if err == nil {
		t.Fatal("expected refusal when provenance unverified and not operator-authorized")
	}
	if res.Repinned {
		t.Fatal("must not re-pin on refusal")
	}
	if s := readState(t, p); s["expected_ship_sha"] != "OLD_SHA" {
		t.Errorf("pin must be UNCHANGED on refusal: %+v", s)
	}
}

func TestRepinShipSHA_OperatorAuthorized_RepinsDespiteNoProvenance(t *testing.T) {
	p := seedStateJSON(t, t.TempDir(), nil)
	res, err := RepinShipSHA(p, "NEW_SHA", "", "21.1.1", falseProv, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Repinned || res.Authorized != "operator" {
		t.Fatalf("operator override should re-pin: %+v", res)
	}
}

func TestRepinShipSHA_EmptyRunningSHA_Errors(t *testing.T) {
	p := seedStateJSON(t, t.TempDir(), nil)
	if _, err := RepinShipSHA(p, "", "abc", "v", trueProv, true); err == nil {
		t.Fatal("must refuse an empty running sha")
	}
}

func TestRepinShipSHA_PreservesOtherStateKeys(t *testing.T) {
	p := seedStateJSON(t, t.TempDir(), map[string]any{"keepMe": "survive"})
	if _, err := RepinShipSHA(p, "NEW_SHA", "abc", "v", trueProv, false); err != nil {
		t.Fatal(err)
	}
	s := readState(t, p)
	if s["keepMe"] != "survive" || s["lastCycleNumber"] != float64(42) {
		t.Errorf("lost an unrelated state key: %+v", s)
	}
}

func TestRepinShipSHA_RejectsRelativeStatePath(t *testing.T) {
	if _, err := RepinShipSHA("relative/state.json", "SHA", "abc", "v", trueProv, true); err == nil {
		t.Fatal("must reject a non-absolute statePath")
	}
}

func TestRepinShipSHA_EmptyPluginVer_LeavesVersionUntouched(t *testing.T) {
	p := seedStateJSON(t, t.TempDir(), nil)
	if _, err := RepinShipSHA(p, "NEW_SHA", "abc", "", trueProv, false); err != nil {
		t.Fatal(err)
	}
	if s := readState(t, p); s["expected_ship_version"] != "21.0.0" {
		t.Errorf("empty pluginVer must leave expected_ship_version untouched, got %v", s["expected_ship_version"])
	}
}

// Concurrent re-pins (e.g. competing resume attempts / fleet) on the shared
// state.json must serialize under flock: no torn JSON, no lost unrelated keys,
// and the final pin is the (single) written value. Run with -race.
func TestRepinShipSHA_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	p := seedStateJSON(t, t.TempDir(), map[string]any{"keepMe": "survive"})
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := RepinShipSHA(p, "CONVERGED_SHA", "abc", "21.1.1", trueProv, false); err != nil {
				t.Errorf("concurrent re-pin: %v", err)
			}
		}()
	}
	wg.Wait()
	s := readState(t, p) // also asserts JSON not torn
	if s["expected_ship_sha"] != "CONVERGED_SHA" || s["keepMe"] != "survive" {
		t.Errorf("concurrent re-pin left bad state: %+v", s)
	}
}
