package bridge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readSeatState(t *testing.T, dir string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, seatStateName))
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatalf("the seat state is not whole JSON: %v", err)
	}
	return state
}

func TestProvisionSeatSettings_AFreshSeatSkipsTheFirstRunOnboarding(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, ".evolve-seat")
	if err := provisionSeatSettings(home, dir); err != nil {
		t.Fatal(err)
	}
	if got := readSeatState(t, dir)[seatOnboardedKey]; got != true {
		t.Fatalf("%s = %v, want true, or each seat on the token stops at the first-run onboarding", seatOnboardedKey, got)
	}
	info, err := os.Stat(filepath.Join(dir, seatStateName))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the seat state must be mode 0600: %v %v", info, err)
	}
}

func TestProvisionSeatSettings_KeepsTheStateThatClaudeCodeWrote(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, ".evolve-seat")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, seatStateName), []byte(`{"userID":"u1","migrationVersion":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provisionSeatSettings(home, dir); err != nil {
		t.Fatal(err)
	}
	state := readSeatState(t, dir)
	if state["userID"] != "u1" || state["migrationVersion"] != float64(3) || state[seatOnboardedKey] != true {
		t.Fatalf("the state of Claude Code must be kept and the onboarding marked done: %v", state)
	}
}

func TestProvisionSeatSettings_AnOnboardedSeatStateIsNotRewritten(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, ".evolve-seat")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"hasCompletedOnboarding": true,  "userID": "u1"}`)
	path := filepath.Join(dir, seatStateName)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provisionSeatSettings(home, dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("an onboarded seat state must stay byte for byte, so a launch never races Claude Code for it: %q %v", got, err)
	}
}

func TestProvisionSeatSettings_AnUnreadableSeatStateRefusesTheLaunch(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, ".evolve-seat")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, seatStateName)
	if err := os.WriteFile(path, []byte(`{"userID":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provisionSeatSettings(home, dir); err == nil {
		t.Fatal("a seat state that is not JSON must refuse the launch, not be replaced")
	}
	if got, _ := os.ReadFile(path); string(got) != `{"userID":` {
		t.Fatalf("a refused launch must leave the seat state as it was: %q", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := provisionSeatSettings(home, dir); err == nil {
		t.Fatal("a seat state that cannot be read must refuse the launch")
	}
}

func TestProvisionSeatSettings_TheBypassAcknowledgementIsSetByThePipelineNotCarried(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"skipDangerousModePermissionPrompt": false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".evolve-seat")
	if err := provisionSeatSettings(home, dir); err != nil {
		t.Fatal(err)
	}
	if seatCarriedSettings[seatBypassAckKey] {
		t.Fatal("the user value of the bypass acknowledgement must never be carried")
	}
	if got := readSeatSettings(t, dir)[seatBypassAckKey]; got != true {
		t.Fatalf("%s = %v, want true: each seat launches in bypass mode, and a seat with no acknowledgement stops at the confirmation, whose default is exit", seatBypassAckKey, got)
	}
}
