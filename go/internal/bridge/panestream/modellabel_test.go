package panestream

import "testing"

const testModelLabelRegex = `^(?:esc to cancel|\? for shortcuts)?\s{2,}(?P<model>\S[^·]*·\s*\S+)\s*$`

func TestModelLabel_ReadsTheFooterLabelOfTheLastLine(t *testing.T) {
	cases := map[string]string{
		"idle.txt":         "Gemini 3.8 Flash · low",
		"editing-a.txt":    "Gemini 3.8 Flash · low",
		"answer.txt":       "Gemini 3.8 Flash · high",
		"parked-paste.txt": "Gemini 3.8 Flash · low",
		"generating-a.txt": "",
	}
	for name, want := range cases {
		if got := ModelLabel(testdataFrame(t, "agy-1.2.17/"+name), testModelLabelRegex); got != want {
			t.Errorf("%s: model label %q, want %q", name, got, want)
		}
	}
	if ModelLabel("? for shortcuts      Gemini · low", "") != "" {
		t.Error("an empty pattern reads no label")
	}
	if ModelLabel("x", `(?P<other>x)`) != "" {
		t.Error("a pattern without a model group reads no label")
	}
	if ModelLabel("\n\n", testModelLabelRegex) != "" {
		t.Error("a blank pane has no label")
	}
}
