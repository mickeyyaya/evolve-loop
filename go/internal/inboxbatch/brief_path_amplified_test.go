package inboxbatch

import (
	"encoding/json"
	"testing"
)

func TestItemBriefPath_Amplified(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"string", `{"id":"a","brief_path":"/x/brief-1.txt"}`, "/x/brief-1.txt"},
		{"absent", `{"id":"a"}`, ""},
		{"empty", `{"id":"a","brief_path":""}`, ""},
		{"null", `{"id":"a","brief_path":null}`, ""},
		{"object", `{"id":"a","brief_path":{"k":"v"}}`, ""},
		{"number", `{"id":"a","brief_path":7}`, ""},
		{"array", `{"id":"a","brief_path":["p"]}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var it Item
			if err := json.Unmarshal([]byte(c.body), &it); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if it.BriefPath != c.want {
				t.Fatalf("BriefPath = %q, want %q", it.BriefPath, c.want)
			}
			if it.ID != "a" {
				t.Fatalf("ID = %q", it.ID)
			}
		})
	}
}

func TestItemBriefPath_RoundTripOmitsEmpty(t *testing.T) {
	out, err := json.Marshal(Item{ID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["brief_path"]; ok {
		t.Fatalf("empty brief_path serialised: %s", out)
	}
	out, _ = json.Marshal(Item{ID: "a", BriefPath: "/p"})
	var back Item
	if err := json.Unmarshal(out, &back); err != nil || back.BriefPath != "/p" {
		t.Fatalf("round trip: %v %q", err, back.BriefPath)
	}
}
