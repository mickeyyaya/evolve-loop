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

	got := repoContractSummary(context.Canceled, nil, "scan.log")

	if got != "[ship] repo-contract gate: error; full output: scan.log" {
		t.Fatalf("got %q", got)
	}
}
