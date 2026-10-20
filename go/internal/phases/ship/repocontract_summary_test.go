package ship

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoContractGate_TheLoopLogCarriesOneSummaryLinePerRun(t *testing.T) {
	repo := repoDeclaring(t, evolveLoopGoMod)
	infra := packOutcome{err: errors.New("exit status 2")}
	for _, tc := range []struct {
		name     string
		outcomes []packOutcome
		verdict  string
	}{
		{"green", []packOutcome{greenPack()}, "green"},
		{"red", []packOutcome{redPack("internal/phasespec.TestSpecParity")}, "REPO_CONTRACT_GATE"},
		{"infra", []packOutcome{infra, infra}, "REPO_CONTRACT_INFRA"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			swapRepoContractTest(t, tc.outcomes...)
			ws := t.TempDir()
			var stderr bytes.Buffer

			_ = runRepoContractGate(context.Background(), "enforce", repo, ws, &stderr)

			scanPath := filepath.Join(ws, scanLogName)
			lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			want := "[ship] repo-contract gate: " + tc.verdict + "; full output: " + scanPath
			if len(lines) != 1 || lines[0] != want {
				t.Fatalf("the loop log must carry one summary line per gate run:\n got %q\nwant %q", stderr.String(), want)
			}
			body, err := os.ReadFile(scanPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, note := range []string{scanChatter, "[ship] repo-contract scanner pack:"} {
				if !strings.Contains(string(body), note) {
					t.Errorf("the scan log must carry %q, got %q", note, body)
				}
			}
		})
	}
}

func TestRepoContractGate_WithoutAScanLogTheNotesStayOnStderr(t *testing.T) {
	swapRepoContractTest(t, greenPack())
	var stderr bytes.Buffer

	if err := runRepoContractGate(context.Background(), "enforce", repoDeclaring(t, evolveLoopGoMod), "", &stderr); err != nil {
		t.Fatal(err)
	}

	for _, note := range []string{scanChatter, "[ship] repo-contract scanner pack:"} {
		if !strings.Contains(stderr.String(), note) {
			t.Errorf("with no scan log the gate must keep its notes on stderr; %q missing from %q", note, stderr.String())
		}
	}
}

func TestRepoContractSummary_AnErrorThatIsNotAShipErrorIsNamedError(t *testing.T) {
	t.Parallel()

	got := repoContractSummary(context.Canceled, gateTally{}, "scan.log")

	if got != "[ship] repo-contract gate: error; full output: scan.log" {
		t.Fatalf("got %q", got)
	}
}

func TestRepoContractGate_APassAfterAnInfraRetryIsNotAPlainGreen(t *testing.T) {
	repo := repoDeclaring(t, evolveLoopGoMod)
	swapRepoContractTest(t, packOutcome{err: errors.New("exit status 2")}, greenPack())
	ws := t.TempDir()
	var stderr bytes.Buffer

	if err := runRepoContractGate(context.Background(), "enforce", repo, ws, &stderr); err != nil {
		t.Fatal(err)
	}

	want := "[ship] repo-contract gate: green after an infra retry of the scanner pack; full output: " + filepath.Join(ws, scanLogName) + "\n"
	if stderr.String() != want {
		t.Fatalf("a green that needed an infra retry must say so:\n got %q\nwant %q", stderr.String(), want)
	}
}

func TestRepoContractGate_WithoutAScanLogAnInfraRetryStaysOnStderr(t *testing.T) {
	swapRepoContractTest(t, packOutcome{err: errors.New("exit status 2")}, greenPack())
	var stderr bytes.Buffer

	if err := runRepoContractGate(context.Background(), "enforce", repoDeclaring(t, evolveLoopGoMod), "", &stderr); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(stderr.String(), "retry GREEN") {
		t.Fatalf("with no scan log the retry note stays on stderr, got %q", stderr.String())
	}
}

func TestRepoContractGate_ASecondRunKeepsTheScanLogOfTheFirst(t *testing.T) {
	repo := repoDeclaring(t, evolveLoopGoMod)
	swapRepoContractTest(t, redPack("internal/phasespec.TestSpecParity"), greenPack())
	ws := t.TempDir()

	if err := runRepoContractGate(context.Background(), "enforce", repo, ws, &bytes.Buffer{}); err == nil {
		t.Fatal("the first run is red")
	}
	if err := runRepoContractGate(context.Background(), "enforce", repo, ws, &bytes.Buffer{}); err != nil {
		t.Fatalf("the second run is green: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(ws, scanLogName))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(body), "[ship] repo-contract gate: run started "); n != 2 {
		t.Fatalf("the scan log must hold both runs, got %d run headers:\n%s", n, body)
	}
	if n := strings.Count(string(body), scanChatter); n < 2 {
		t.Fatalf("the output of the red run must survive the next run, got %d copies:\n%s", n, body)
	}
}

func TestRepoContractGate_AnUnopenableScanLogKeepsTheNotesOnStderr(t *testing.T) {
	swapRepoContractTest(t, greenPack())
	ws := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(ws, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	if err := runRepoContractGate(context.Background(), "enforce", repoDeclaring(t, evolveLoopGoMod), ws, &stderr); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"scan log " + filepath.Join(ws, scanLogName) + " unavailable", scanChatter} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("an unopenable scan log falls back to stderr; %q missing from %q", want, stderr.String())
		}
	}
}
