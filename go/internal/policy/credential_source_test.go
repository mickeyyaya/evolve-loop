package policy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestIsolationCredentialSourceStage_ReadsTheKeyAndDefaultsToOff(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]string{
		`{}`:                                     "off",
		`{"isolation":{}}`:                       "off",
		`{"isolation":{"credential_source":{}}}`: "off",
		`{"isolation":{"credential_source":{"stage":"off"}}}`:    "off",
		`{"isolation":{"credential_source":{"stage":"shadow"}}}`: "shadow",
	} {
		p, err := policy.Parse([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if got := p.IsolationCredentialSourceStage(); got != want {
			t.Errorf("%s: IsolationCredentialSourceStage = %q, want %q", body, got, want)
		}
	}
}

func TestCredentialSourcePolicy_RefusesEnforceAndAnUnknownStage(t *testing.T) {
	t.Parallel()
	for stage, want := range map[string]string{
		"enforce": `stage "enforce" lands after the operator answers D1-OP1`,
		"loud":    `stage "loud": the stages are off and shadow`,
	} {
		_, err := policy.Parse([]byte(`{"isolation":{"credential_source":{"stage":"` + stage + `"}}}`))
		if err == nil || !strings.Contains(err.Error(), "isolation.credential_source: ") || !strings.Contains(err.Error(), want) {
			t.Errorf("stage %s: err = %v, want %q", stage, err, want)
		}
	}
}

func TestCredentialSourceStageFor_ReadsThePolicyOfTheRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if got, err := policy.CredentialSourceStageFor(root); err != nil || got != "off" {
		t.Fatalf("no policy file: %q, %v; want off", got, err)
	}
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"isolation":{"credential_source":{"stage":"shadow"}}}`)
	if got, err := policy.CredentialSourceStageFor(root); err != nil || got != "shadow" {
		t.Fatalf("shadow policy: %q, %v", got, err)
	}
	write(`{"isolation":{"credential_source":{"stage":"enforce"}}}`)
	if got, err := policy.CredentialSourceStageFor(root); err == nil || got != "off" {
		t.Fatalf("a bad policy: %q, %v; want off and an error", got, err)
	}
}

func TestTheCheckedInPolicy_RunsTheCredentialSourceInShadow(t *testing.T) {
	t.Parallel()
	stage, err := policy.CredentialSourceStageFor(filepath.Join("..", "..", ".."))
	if err != nil || stage != policy.CredentialSourceStageShadow {
		t.Fatalf("isolation.credential_source.stage = %q, %v; want shadow", stage, err)
	}
}

func TestCredentialSourcePolicy_TheTypedBlockCarriesTheStage(t *testing.T) {
	t.Parallel()
	p := policy.Policy{Isolation: &policy.IsolationPolicy{CredentialSource: &policy.CredentialSourcePolicy{Stage: "shadow"}}}
	if got := p.IsolationCredentialSourceStage(); got != policy.CredentialSourceStageShadow {
		t.Fatalf("IsolationCredentialSourceStage = %q, want shadow", got)
	}
}
