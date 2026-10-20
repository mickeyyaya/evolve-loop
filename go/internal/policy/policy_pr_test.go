package policy

import (
	"strings"
	"testing"
	"time"
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

func TestPRMergeWindowStage_ALiteralMergeWindowPolicyResolvesToItsStage(t *testing.T) {
	cases := []struct {
		name   string
		policy Policy
		want   string
	}{
		{"no window block resolves to MergeWindowOff", Policy{PR: &PRPolicy{}}, MergeWindowOff},
		{"a MergeWindowPolicy at MergeWindowOn resolves to on", Policy{PR: &PRPolicy{MergeWindow: &MergeWindowPolicy{Stage: MergeWindowOn}}}, MergeWindowOn},
		{"a MergeWindowPolicy at MergeWindowOff resolves to off", Policy{PR: &PRPolicy{MergeWindow: &MergeWindowPolicy{Stage: MergeWindowOff}}}, MergeWindowOff},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := c.policy.PRMergeWindowStage(); err != nil || got != c.want {
				t.Fatalf("PRMergeWindowStage() = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestPREarlyBoundaryAfter_DefaultsToOneHourAndRefusesANonPositiveBound(t *testing.T) {
	cases := []struct {
		name, json string
		want       time.Duration
		wantErr    bool
	}{
		{"absent block is the compiled hour", `{}`, time.Hour, false},
		{"absent key is the compiled hour", `{"pr":{"merge_window":{"stage":"on"}}}`, time.Hour, false},
		{"a set bound", `{"pr":{"merge_window":{"early_boundary_after_s":1800}}}`, 30 * time.Minute, false},
		{"zero fails closed", `{"pr":{"merge_window":{"early_boundary_after_s":0}}}`, 0, true},
		{"negative fails closed", `{"pr":{"merge_window":{"early_boundary_after_s":-5}}}`, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := loadCIWatchPolicy(t, c.json).PREarlyBoundaryAfter()
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("PREarlyBoundaryAfter() = %v, %v; want %v, err=%v", got, err, c.want, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "pr.merge_window.early_boundary_after_s") {
				t.Errorf("the error must name the key: %v", err)
			}
		})
	}
	if DefaultEarlyBoundaryAfterS != 3600 {
		t.Errorf("DefaultEarlyBoundaryAfterS = %d, want the 1 h of plan Q2", DefaultEarlyBoundaryAfterS)
	}
}
