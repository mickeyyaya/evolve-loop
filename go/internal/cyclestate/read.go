package cyclestate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func ReadLastCycleNumber(path string) (int, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read state.json: %w", err)
	}
	var raw struct {
		LastCycleNumber *int `json:"lastCycleNumber"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return 0, false, fmt.Errorf("parse state.json: %w", err)
	}
	if raw.LastCycleNumber == nil {
		return 0, false, nil
	}
	return *raw.LastCycleNumber, true, nil
}

func ReadState(path string) (State, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, fmt.Errorf("read state.json: %w", err)
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, false, fmt.Errorf("parse state.json: %w", err)
	}
	return st, true, nil
}
