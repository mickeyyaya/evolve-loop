package committedset

import (
	"encoding/json"
	"strings"
)

type LaneSpec struct {
	DependsOn      []string `json:"depends_on,omitempty"`
	Priority       int      `json:"priority,omitempty"`
	OutputContract string   `json:"output_contract,omitempty"`
}

func (s LaneSpec) Encode() string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func DecodeLaneSpec(raw string) (LaneSpec, error) {
	if strings.TrimSpace(raw) == "" {
		return LaneSpec{}, nil
	}
	var spec LaneSpec
	err := json.Unmarshal([]byte(raw), &spec)
	return spec, err
}

func (s LaneSpec) IsZero() bool {
	return len(s.DependsOn) == 0 && s.Priority == 0 && s.OutputContract == ""
}
