package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func lostLandingVerdict() string { return VerdictWARN }

// detectLostLanding reports a system-class signal when a cycle claims a
// shipping verdict but its own ship phase produced no binding; nil means
// there is nothing to report.
func detectLostLanding(workspace, finalVerdict string) *SystemFailureSignal {
	if workspace == "" || !IsShippingVerdict(finalVerdict) {
		return nil
	}
	if _, err := os.Stat(filepath.Join(workspace, "ship-binding.json")); err == nil {
		return nil
	}
	code, class, msg, ok := readShipError(workspace)
	if !ok {
		return nil
	}
	return &SystemFailureSignal{
		Category: "landing-lost",
		Level:    "system",
		Halt:     false,
		Evidence: fmt.Sprintf(
			"cycle reported %s but ship produced no commit: ship-error %s (%s) with no ship-binding.json in %s — the work was completed and then discarded. %s",
			finalVerdict, code, class, workspace, msg),
	}
}

// readShipError treats an absent or unreadable ship-error.json as "ship
// never ran" — the conservative reading, since inventing a landing loss
// from a missing file would fire on every pre-ship cycle.
func readShipError(workspace string) (code, class, msg string, ok bool) {
	b, err := os.ReadFile(filepath.Join(workspace, "ship-error.json"))
	if err != nil {
		return "", "", "", false
	}
	var se struct {
		Code    string `json:"code"`
		Class   string `json:"class"`
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &se) != nil || se.Code == "" {
		return "", "", "", false
	}
	return se.Code, se.Class, se.Message, true
}
