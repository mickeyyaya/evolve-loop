package looppreflight

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestNewDefaultBootTester_UnusableTempDirIsASetupFailureNotABridgeExit(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
	tester := newDefaultBootTester(t.TempDir(), io.Discard)

	out := tester(context.Background(), "claude-tmux", false)

	if out.RC != exitWorkspaceSetupFailed || !strings.Contains(out.Scrollback, "could not create boot workspace") {
		t.Fatalf("outcome = %+v", out)
	}
	if out.RC >= 0 {
		t.Errorf("the setup failure code must stay negative so it never collides with a bridge exit: %d", out.RC)
	}
}

func TestDefaultSelfUpdateEvidence_UnresolvableHomeIsAmbiguityForRegisteredUpdaters(t *testing.T) {
	t.Setenv("HOME", "")

	for _, bin := range []string{"claude", "codex"} {
		ok, evidence, err := defaultSelfUpdateEvidence(bin)
		if err == nil || ok || evidence != "" || !strings.Contains(err.Error(), "user home dir unresolvable") {
			t.Errorf("%s: ok=%v evidence=%q err=%v", bin, ok, evidence, err)
		}
	}
	if ok, _, err := defaultSelfUpdateEvidence("agy"); ok || err != nil {
		t.Errorf("an unregistered binary needs no home lookup: ok=%v err=%v", ok, err)
	}
}

func TestDefaultSelfUpdateEvidence_UnreadableClaudeSettingsAreAmbiguityNotAbsence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"autoUpdates":true}`), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(settings, 0o644) })
	if f, err := os.Open(settings); err == nil {
		f.Close()
		t.Skip("file permissions are not enforced for this user")
	}

	ok, evidence, err := defaultSelfUpdateEvidence("claude")

	if err == nil || ok || evidence != "" || !strings.Contains(err.Error(), "claude settings unreadable") {
		t.Fatalf("ok=%v evidence=%q err=%v", ok, evidence, err)
	}
}

func TestRun_LLMCLIStatus_ProbeErrorHaltsNamingTheBinary(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProbeCLI = func(bin string) (doctor.Result, error) {
		return doctor.Result{}, errors.New("exec format error")
	}

	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}

	c := findCheck(t, r, "llm-cli-status")
	if c.Level != LevelHalt || !strings.Contains(c.Detail, `CLI "claude": probe error: exec format error`) {
		t.Fatalf("level=%s detail=%q", c.Level, c.Detail)
	}
}

func TestRun_VersionFreeze_DriversSharingOneBinaryAreCheckedOnce(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileLister = func() ([]string, error) { return []string{"builder", "auditor"}, nil }
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		if name == "auditor" {
			return profiles.Profile{Name: name, CLI: "agy-claude-tmux"}, nil
		}
		return profiles.Profile{Name: name, CLI: "agy-tmux"}, nil
	}
	var asked []string
	opts.SelfUpdateEvidence = func(bin string) (bool, string, error) {
		asked = append(asked, bin)
		return false, "", nil
	}

	if _, err := Run(opts); err != nil {
		t.Fatal(err)
	}

	if len(asked) != 1 || asked[0] != "agy" {
		t.Fatalf("self-update evidence asked for %v; two drivers of one binary must be checked once", asked)
	}
}

func TestDefaultSelfUpdateEvidence_BinaryWithoutAnyUpdaterRegistrationHasNoEvidence(t *testing.T) {
	t.Setenv("HOME", "")

	ok, evidence, err := defaultSelfUpdateEvidence("a-binary-no-manifest-or-registry-names")

	if ok || evidence != "" || err != nil {
		t.Fatalf("ok=%v evidence=%q err=%v: an unregistered binary is neither risky nor ambiguous", ok, evidence, err)
	}
}
