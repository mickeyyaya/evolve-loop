// repair_selfsha_test.go — RED contract for repair-ladder mode #1
// (ADR-0039 §8): SELF_SHA_TAMPERED from a STALE TOFU pin.
//
// Cycles 246-248 incident: an operator manual ship (or rebuild) committed a
// new ship binary at HEAD without re-pinning expected_ship_sha (repinPostCycle
// only fires for --class cycle). The next cycle's ship then hit
// SELF_SHA_TAMPERED even though the running binary matched the binary blob
// COMMITTED AT HEAD — i.e. provably built from audited, committed source.
// The operator hand-edited state.json twice.
//
// The repair: when the running binary's SHA equals the SHA of the binary blob
// at git HEAD, the pin is stale — re-pin and re-run verifySelfSHA. When they
// differ, the binary genuinely diverges from committed source: the integrity
// BLOCK stands untouched (same trust boundary as repinPostCycle, which already
// pins from HEAD:go/evolve).
package ship

import (
	"path/filepath"
	"strings"
	"testing"
)

// stalePinStateJSON writes state.json with a deliberately wrong pin under the
// CURRENT plugin version — the exact signature of the cycles-246-248 incident.
func stalePinStateJSON(t *testing.T, repo string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, ".evolve", "state.json"),
		`{"expected_ship_sha":"`+strings.Repeat("a", 64)+`","expected_ship_version":"1.0.0"}`)
}

// TestRepair_SelfSHA_VerifiedRebuild_RepinsAndShips: stale pin + running
// binary identical to the blob committed at HEAD → ship must re-pin and
// proceed to a normal commit+push instead of dying ExitIntegrity.
func TestRepair_SelfSHA_VerifiedRebuild_RepinsAndShips(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)
	stalePinStateJSON(t, repo)

	// Audited change ready to ship.
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

	// The pin must now equal the sha of the binary as committed at HEAD
	// (== the running fixture binary content from makeRepo).
	stMap, rerr := readStateMap(filepath.Join(repo, ".evolve", "state.json"))
	if rerr != nil {
		t.Fatalf("read state.json: %v", rerr)
	}
	want := sha256Hex([]byte("ship-binary-v1\n"))
	if got := stateString(stMap, "expected_ship_sha"); got != want {
		t.Errorf("expected_ship_sha = %q, want re-pinned %q", got, want)
	}

	// And the ship actually landed: remote advanced to local HEAD.
	if got, head := remoteHeadSHA(t, repo), headSHA(t, repo); got != head {
		t.Errorf("remote main = %s, want pushed HEAD %s", got, head)
	}
}

// TestRepair_SelfSHA_GenuineTamper_StillBlocks (adversarial guard): stale pin
// AND a running binary that does NOT match the blob at HEAD (uncommitted
// modification) must remain an integrity BLOCK — the repair narrows the
// tampering definition, it must not waive it.
func TestRepair_SelfSHA_GenuineTamper_StillBlocks(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)
	stalePinStateJSON(t, repo)

	// Tamper the binary ON DISK without committing: diverges from HEAD blob.
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

// TestRepair_SelfSHA_BinaryNotAtHEAD_StillBlocks: when the ship binary is not
// a blob at HEAD at all (untracked path), there is nothing to verify the
// rebuild against — the integrity BLOCK stands.
func TestRepair_SelfSHA_BinaryNotAtHEAD_StillBlocks(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".claude-plugin", "plugin.json"), `{"version":"1.0.0"}`)
	addRemote(t, repo)
	stalePinStateJSON(t, repo)

	// Untracked binary: written AFTER the initial commit, never committed.
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
