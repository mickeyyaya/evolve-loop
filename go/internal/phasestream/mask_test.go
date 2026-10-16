package phasestream

import (
	"reflect"
	"testing"
)

func TestMaskStaleObservations_MasksOldEvictablePreservesRest(t *testing.T) {
	in := []Envelope{
		{Seq: 1, Kind: KindResult, Data: map[string]any{"is_error": false}},
		{Seq: 2, Kind: KindToolUse, Data: map[string]any{"name": "read", "input_excerpt": "OLD"}},
		{Seq: 3, Kind: KindToolResult, Data: map[string]any{"tool_use_id": "read", "excerpt": "O2"}},
		{Seq: 4, Kind: KindToolUse, Data: map[string]any{"name": "grep", "input_excerpt": "KEEP"}},
	}
	out := MaskStaleObservations(in, 1)
	if len(out) != len(in) {
		t.Fatalf("length changed: got %d want %d", len(out), len(in))
	}
	if m, _ := out[3].Data["masked"].(bool); m {
		t.Errorf("newest evictable (Seq 4) must not be masked: %v", out[3].Data)
	}
	if out[3].Data["input_excerpt"] != "KEEP" {
		t.Errorf("in-window content altered: %v", out[3].Data["input_excerpt"])
	}
	for _, i := range []int{1, 2} {
		if m, _ := out[i].Data["masked"].(bool); !m {
			t.Errorf("out-of-window evictable (idx %d) must be masked: %v", i, out[i].Data)
		}
	}
	if c := out[1].Data["input_excerpt"]; c == "OLD" {
		t.Errorf("masked envelope still carries original content: %v", c)
	}
	if m, _ := out[0].Data["masked"].(bool); m {
		t.Errorf("KindResult (verdict) must never be masked: %v", out[0].Data)
	}
	if in[1].Data["input_excerpt"] != "OLD" {
		t.Errorf("input envelope mutated in place: %v", in[1].Data["input_excerpt"])
	}
}

func TestMaskStaleObservations_WindowLEZeroPassthrough(t *testing.T) {
	in := []Envelope{
		{Seq: 1, Kind: KindToolUse, Data: map[string]any{"name": "read", "input_excerpt": "a"}},
		{Seq: 2, Kind: KindToolResult, Data: map[string]any{"tool_use_id": "read", "excerpt": "b"}},
	}
	for _, w := range []int{0, -5} {
		out := MaskStaleObservations(in, w)
		for i := range out {
			if !reflect.DeepEqual(out[i].Data, in[i].Data) {
				t.Errorf("windowTurns=%d altered observation %d: got %v want %v", w, i, out[i].Data, in[i].Data)
			}
		}
	}
}
