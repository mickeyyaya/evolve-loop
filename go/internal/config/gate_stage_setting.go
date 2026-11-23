package config

import (
	"encoding/json"
	"fmt"
)

type GateStageSetting string

func (s *GateStageSetting) UnmarshalJSON(raw []byte) error {
	var word string
	if err := json.Unmarshal(raw, &word); err != nil {
		return fmt.Errorf("a gate stage is a JSON string: %w", err)
	}
	if _, ok := GateStage(word); !ok {
		return fmt.Errorf("stage %q is not off, shadow or enforce", word)
	}
	*s = GateStageSetting(word)
	return nil
}

func (s GateStageSetting) Stage() Stage {
	stage, _ := GateStage(string(s))
	return stage
}
