package inboxmover

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type ConsumedRecord struct {
	At         string `json:"at,omitempty"`
	Via        string `json:"via,omitempty"`
	Cycle      string `json:"cycle,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Note       string `json:"note,omitempty"`
	Reason     string `json:"reason,omitempty"`
	MergedInto string `json:"merged_into,omitempty"`
	Verdict    string `json:"verdict,omitempty"`
	By         string `json:"by,omitempty"`
	Landing    string `json:"landing,omitempty"`
}

type consumedWire struct {
	ConsumedRecord
	Cycle any `json:"cycle"`
}

func ParseConsumedRecord(item []byte) (ConsumedRecord, bool, error) {
	var doc struct {
		Consumed json.RawMessage `json:"consumed"`
	}
	if err := json.Unmarshal(item, &doc); err != nil {
		return ConsumedRecord{}, false, err
	}
	raw := bytes.TrimSpace(doc.Consumed)
	if len(raw) == 0 || string(raw) == "null" {
		return ConsumedRecord{}, false, nil
	}
	var at string
	if json.Unmarshal(raw, &at) == nil {
		return ConsumedRecord{At: at}, true, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var wire consumedWire
	if err := dec.Decode(&wire); err != nil {
		return ConsumedRecord{}, false, fmt.Errorf("inboxmover: consumed must be an object or string: %w", err)
	}
	cycle, err := cycleText(wire.Cycle)
	if err != nil {
		return ConsumedRecord{}, false, err
	}
	wire.ConsumedRecord.Cycle = cycle
	return wire.ConsumedRecord, true, nil
}

func cycleText(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	case json.Number:
		if _, err := strconv.ParseInt(t.String(), 10, 64); err != nil {
			return "", fmt.Errorf("inboxmover: cycle must be an integer or label, got %s", t)
		}
		return t.String(), nil
	}
	return "", fmt.Errorf("inboxmover: cycle must be an integer or label, got %v", v)
}
