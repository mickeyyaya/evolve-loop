package bridge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const seatAutoMemoryKey = "autoMemoryEnabled"

var seatCarriedSettings = map[string]bool{
	"model":                 true,
	"theme":                 true,
	"verbose":               true,
	"editorMode":            true,
	"effortLevel":           true,
	"alwaysThinkingEnabled": true,
	"includeCoAuthoredBy":   true,
	"outputStyle":           true,
	"cleanupPeriodDays":     true,
}

func carriedUserSettings(home string) map[string]any {
	out := map[string]any{}
	body, err := os.ReadFile(filepath.Join(home, ".claude", seatSettingsName))
	if err != nil {
		return out
	}
	var user map[string]any
	if json.Unmarshal(body, &user) != nil {
		return out
	}
	for k, v := range user {
		if seatCarriedSettings[k] {
			out[k] = v
		}
	}
	return out
}

func provisionSeatSettings(home, dir string) error {
	out := carriedUserSettings(home)
	out[seatAutoMemoryKey] = false
	body, marshalErr := json.MarshalIndent(out, "", "  ")
	if err := errors.Join(marshalErr, os.MkdirAll(dir, seatDirMode)); err != nil {
		return err
	}
	return atomicwrite.Durable(filepath.Join(dir, seatSettingsName), body, 0o600)
}
