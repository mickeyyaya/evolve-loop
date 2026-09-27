package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// errTest is a generic injected-failure sentinel for the matrix dep stubs.
var errTest = errors.New("injected verify failure")

// okDeps returns matrixDeps where every CLI install/projection and every binary
// subcommand succeeds — the all-green baseline each negative test perturbs.
func okDeps() matrixDeps {
	return matrixDeps{
		installClaude: func(srcDir, home string) error { return nil },
		projectTarget: func(srcDir, target string) error { return nil },
		geminiLayout:  func(srcDir string) error { return nil },
		binAnswers:    func(binPath, sub string) error { return nil },
	}
}

func resultFor(results []cliVerify, key string) (cliVerify, bool) {
	for _, r := range results {
		if r.CLI == key {
			return r, true
		}
	}
	return cliVerify{}, false
}

func TestVerifyReleaseCLIMatrix_AllPass(t *testing.T) {
	results := verifyReleaseCLIMatrix("/src", "/bin/evolve", okDeps())

	for _, cli := range releaseVerifyCLIs {
		r, ok := resultFor(results, cli)
		if !ok {
			t.Fatalf("no result for CLI %q — every supported CLI must be verified", cli)
		}
		if !r.OK {
			t.Errorf("CLI %q reported not-OK with all deps passing: %s", cli, r.Detail)
		}
	}
	if r, ok := resultFor(results, binaryRowKey); !ok || !r.OK {
		t.Errorf("binary-subcommand row missing or not-OK: %+v", r)
	}
}

func TestVerifyReleaseCLIMatrix_MissingSubcommand(t *testing.T) {
	d := okDeps()
	d.binAnswers = func(binPath, sub string) error {
		if sub == "loop" {
			return errTest
		}
		return nil
	}
	results := verifyReleaseCLIMatrix("/src", "/bin/evolve", d)

	r, ok := resultFor(results, binaryRowKey)
	if !ok || r.OK {
		t.Fatalf("binary row must be not-OK when a core subcommand is unanswered: %+v", r)
	}
	if !strings.Contains(r.Detail, "loop") {
		t.Errorf("binary row must name the missing subcommand 'loop', got: %s", r.Detail)
	}
	if c, _ := resultFor(results, "claude"); !c.OK {
		t.Errorf("claude install should still be OK; a binary fault must not bleed into a CLI row: %s", c.Detail)
	}
}

func TestVerifyReleaseCLIMatrix_OneCLIFails(t *testing.T) {
	d := okDeps()
	d.projectTarget = func(srcDir, target string) error {
		if target == "codex" {
			return errTest
		}
		return nil
	}
	results := verifyReleaseCLIMatrix("/src", "/bin/evolve", d)

	if c, _ := resultFor(results, "codex"); c.OK {
		t.Errorf("codex must be not-OK when its projection fails")
	}
	for _, other := range []string{"claude", "agy", "gemini"} {
		if c, _ := resultFor(results, other); !c.OK {
			t.Errorf("%s must stay OK when only codex fails (isolation), got: %s", other, c.Detail)
		}
	}
}

func TestVerifyReleaseCLIMatrix_AllFailVisible(t *testing.T) {
	d := matrixDeps{
		installClaude: func(string, string) error { return errTest },
		projectTarget: func(string, string) error { return errTest },
		geminiLayout:  func(string) error { return errTest },
		binAnswers:    func(string, string) error { return errTest },
	}
	results := verifyReleaseCLIMatrix("/src", "/bin/evolve", d)
	for _, cli := range releaseVerifyCLIs {
		if r, ok := resultFor(results, cli); !ok || r.OK {
			t.Errorf("CLI %q must be reported not-OK, got ok=%v", cli, r.OK)
		}
	}
}

func TestCoreSubcommandsRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, c := range commands {
		registered[c.Name] = true
		for _, a := range c.Aliases {
			registered[a] = true
		}
	}
	for _, sub := range coreSubcommands {
		if !registered[sub] {
			t.Errorf("coreSubcommands lists %q which is not a registered command — drift", sub)
		}
	}
}

// TestBinAnswersStartFailureIsLoud exercises the real default binAnswers, not a
// stub, so a genuinely missing binary is exercised.
func TestBinAnswersStartFailureIsLoud(t *testing.T) {
	d := defaultMatrixDeps()
	if err := d.binAnswers(filepath.Join(t.TempDir(), "does-not-exist-evolve"), "loop"); err == nil {
		t.Fatal("binAnswers must error when the binary cannot be started, got nil")
	}
}
