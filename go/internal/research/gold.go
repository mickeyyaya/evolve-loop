package research

import (
	"encoding/json"
	"fmt"
	"os"
)

type GoldQuery struct {
	ID       string   `json:"id"`
	Plan     string   `json:"plan"`
	Query    Query    `json:"query"`
	Relevant []string `json:"relevant"`
}

type GoldSet struct {
	Queries []GoldQuery `json:"queries"`
}

func LoadGoldSet(path string) (GoldSet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return GoldSet{}, fmt.Errorf("read gold set %s: %w", path, err)
	}
	var set GoldSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return GoldSet{}, fmt.Errorf("parse gold set %s: %w", path, err)
	}
	return set, nil
}
