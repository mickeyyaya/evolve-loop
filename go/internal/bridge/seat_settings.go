package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const (
	seatAutoMemoryKey = "autoMemoryEnabled"
	seatBypassAckKey  = "skipDangerousModePermissionPrompt"
	seatStateName     = ".claude.json"
	seatOnboardedKey  = "hasCompletedOnboarding"
)

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
	out[seatBypassAckKey] = true
	body, marshalErr := json.MarshalIndent(out, "", "  ")
	if err := errors.Join(marshalErr, os.MkdirAll(dir, seatDirMode)); err != nil {
		return err
	}
	if err := atomicwrite.Durable(filepath.Join(dir, seatSettingsName), body, 0o600); err != nil {
		return err
	}
	return markSeatOnboarded(dir)
}

func markSeatOnboarded(dir string) error {
	path := filepath.Join(dir, seatStateName)
	state := map[string]any{}
	body, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return fmt.Errorf("read the seat state %s: %w", path, err)
	default:
		if err := json.Unmarshal(body, &state); err != nil {
			return fmt.Errorf("the seat state %s is not JSON, so the launch stops and leaves it as it is: %w", path, err)
		}
	}
	if state[seatOnboardedKey] == true {
		return nil
	}
	state[seatOnboardedKey] = true
	out, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		err = atomicwrite.Durable(path, out, 0o600)
	}
	return err
}
