//go:build integration

package ship

import (
	"path/filepath"
	"strings"
	"testing"
)

// stalePinStateJSON writes state.json with a deliberately wrong pin under the
// current plugin version.
func stalePinStateJSON(t *testing.T, repo string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"),
		`{"expected_ship_sha":"`+strings.Repeat("a", 64)+`","expected_ship_version":"1.0.0"}`)
}

func TestRepair_SelfSHA_VerifiedRebuild_RepinsAndShips(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)
	stalePinStateJSON(t, repo)

	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\naudited edit\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: verified-rebuild repin"})
	if res.ExitCode != ExitOK {
		t.Fatalf("stale pin with verified rebuild must self-heal; got exit=%d err=%v logs=%v",
			res.ExitCode, err, res.Logs)
	}
	if res.RepairAttempted != "SELF_SHA_TAMPERED" {
		t.Errorf("RepairAttempted = %q, want SELF_SHA_TAMPERED", res.RepairAttempted)
	}
	if res.RepairOutcome == "" || res.RepairOutcome == "declined" {
		t.Errorf("RepairOutcome = %q, want a successful repin outcome", res.RepairOutcome)
	}

	// want is the sha of makeRepo's fixture binary content, as committed at HEAD.
	stMap, rerr := readStateMap(filepath.Join(repo, ".evolve", "state.json"))
	if rerr != nil {
		t.Fatalf("read state.json: %v", rerr)
	}
	want := sha256Hex([]byte("ship-binary-v1\n"))
	if got := stateString(stMap, "expected_ship_sha"); got != want {
		t.Errorf("expected_ship_sha = %q, want re-pinned %q", got, want)
	}

	if got, head := remoteHeadSHA(t, repo), headSHA(t, repo); got != head {
		t.Errorf("remote main = %s, want pushed HEAD %s", got, head)
	}
}

func TestRepair_SelfSHA_GenuineTamper_StillBlocks(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)
	stalePinStateJSON(t, repo)

	// Written on disk without committing, so it diverges from the HEAD blob.
	mustWrite(t, filepath.Join(repo, "ship-binary-fixture"), "ship-binary-v1\n# injected payload\n")

	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nedit\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "should refuse"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("genuine tamper must stay ExitIntegrity; got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	se := mustShipErr(t, err)
	if se.Code != "SELF_SHA_TAMPERED" {
		t.Errorf("Code = %s, want SELF_SHA_TAMPERED", se.Code)
	}
	if se.Debug["repair_outcome"] != "declined" {
		t.Errorf("Debug[repair_outcome] = %q, want declined (repair must be attempted + observable)", se.Debug["repair_outcome"])
	}
}

func TestRepair_SelfSHA_BinaryNotAtHEAD_StillBlocks(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)
	stalePinStateJSON(t, repo)

	// Written after the initial commit; never committed, so no blob at HEAD.
	loose := filepath.Join(repo, "loose-bin")
	mustWrite(t, loose, "loose binary content\n")

	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nedit\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{
		Class: ClassCycle, CommitMessage: "should refuse", ShipBinaryPath: loose,
	})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("untracked binary must stay ExitIntegrity; got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	se := mustShipErr(t, err)
	if se.Code != "SELF_SHA_TAMPERED" {
		t.Errorf("Code = %s, want SELF_SHA_TAMPERED", se.Code)
	}
}
