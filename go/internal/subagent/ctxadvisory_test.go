package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return p
}

func TestCheckCtxAdvisory_EmitsAboveThreshold(t *testing.T) {
	p := writeProfile(t, `{"role":"tester","context_clear_trigger_tokens":150000}`)
	res, err := CheckCtxAdvisory(p, 200000)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !res.Emit {
		t.Fatalf("expected emit=true for tokens>threshold")
	}
	if res.Threshold != 150000 {
		t.Errorf("threshold=%d, want 150000", res.Threshold)
	}
	if !strings.Contains(res.Message, "~200000") || !strings.Contains(res.Message, "150000") {
		t.Errorf("message missing token + threshold: %q", res.Message)
	}
	if !strings.Contains(res.Message, "Tool-Result Hygiene") {
		t.Errorf("message missing hygiene reminder: %q", res.Message)
	}
}

func TestCheckCtxAdvisory_SuppressedAtOrBelowThreshold(t *testing.T) {
	p := writeProfile(t, `{"context_clear_trigger_tokens":50000}`)
	for _, tokens := range []int{0, 1, 50000} {
		res, err := CheckCtxAdvisory(p, tokens)
		if err != nil {
			t.Fatalf("tokens=%d: %v", tokens, err)
		}
		if res.Emit {
			t.Errorf("tokens=%d: emit=true, want false", tokens)
		}
	}
}

func TestCheckCtxAdvisory_NoThresholdSuppressed(t *testing.T) {
	p := writeProfile(t, `{"role":"x"}`)
	res, err := CheckCtxAdvisory(p, 9999999)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.Emit {
		t.Errorf("no threshold should suppress emit")
	}
	if res.Threshold != 0 {
		t.Errorf("threshold=%d, want 0", res.Threshold)
	}
}

func TestCheckCtxAdvisory_MalformedThresholdSuppressed(t *testing.T) {
	p := writeProfile(t, `{"context_clear_trigger_tokens":"oops"}`)
	res, err := CheckCtxAdvisory(p, 1)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.Emit {
		t.Errorf("string threshold should not emit")
	}
}

func TestCheckCtxAdvisory_MissingProfileReturnsError(t *testing.T) {
	_, err := CheckCtxAdvisory(filepath.Join(t.TempDir(), "absent.json"), 100)
	if err == nil {
		t.Fatalf("expected error for missing profile")
	}
	if !strings.Contains(err.Error(), "read profile") {
		t.Errorf("error message lacks read profile context: %v", err)
	}
}

func TestExtractInt(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		field string
		want  int
		ok    bool
	}{
		{"present", `{"n":42}`, "n", 42, true},
		{"with whitespace", `{"n" : 7}`, "n", 7, true},
		{"absent", `{"x":1}`, "n", 0, false},
		{"non-integer value", `{"n":"oops"}`, "n", 0, false},
		{"trailing zero", `{"n":0}`, "n", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractInt(tc.body, tc.field)
			if ok != tc.ok || got != tc.want {
				t.Errorf("got (%d, %v), want (%d, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}
