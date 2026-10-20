package policy

import (
	"strings"
	"testing"
)

func TestPRMergeMethod_DefaultsAndRejects(t *testing.T) {
	cases := []struct {
		name, json, want string
		wantErr          bool
	}{
		{"absent block defaults to merge", `{}`, "merge", false},
		{"empty field defaults to merge", `{"pr":{}}`, "merge", false},
		{"merge", `{"pr":{"merge_method":"merge"}}`, "merge", false},
		{"squash", `{"pr":{"merge_method":"squash"}}`, "squash", false},
		{"rebase", `{"pr":{"merge_method":"rebase"}}`, "rebase", false},
		{"unknown word fails closed", `{"pr":{"merge_method":"fast-forward"}}`, "", true},
		{"case-sensitive", `{"pr":{"merge_method":"Squash"}}`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := loadCIWatchPolicy(t, c.json).PRMergeMethod()
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("PRMergeMethod() = %q, %v; want %q, err=%v", got, err, c.want, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "pr.merge_method") {
				t.Errorf("the error must name the key: %v", err)
			}
		})
	}
	if got, err := (Policy{PR: &PRPolicy{MergeMethod: "squash"}}).PRMergeMethod(); err != nil || got != "squash" {
		t.Errorf("a literal PRPolicy must resolve: %q, %v", got, err)
	}
}

func TestPRMergeWindowStage_DefaultsOffUntilTheOperatorEnablesIt(t *testing.T) {
	cases := []struct {
		name, json, want string
		wantErr          bool
	}{
		{"absent block is off", `{}`, "off", false},
		{"absent window is off", `{"pr":{"merge_method":"squash"}}`, "off", false},
		{"empty stage is off", `{"pr":{"merge_window":{}}}`, "off", false},
		{"off", `{"pr":{"merge_window":{"stage":"off"}}}`, "off", false},
		{"on", `{"pr":{"merge_window":{"stage":"on"}}}`, "on", false},
		{"unknown word fails closed", `{"pr":{"merge_window":{"stage":"enforce"}}}`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := loadCIWatchPolicy(t, c.json).PRMergeWindowStage()
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("PRMergeWindowStage() = %q, %v; want %q, err=%v", got, err, c.want, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "pr.merge_window.stage") {
				t.Errorf("the error must name the key: %v", err)
			}
		})
	}
}
