package verifyeval

import "testing"

// TestVerify_ResultContract names the verifyeval.Result type (returned by Verify
// but never named in a test) and pins the constructor's contract: a satisfied
// eval yields Verdict=="PASS", echoes the source Path, and records exactly one
// passing CommandResult per executed command.
func TestVerify_ResultContract(t *testing.T) {
	path := writeEval(t, "```bash\ngo test ./...\n```\n\n## Expected\n\nexit_code: 0\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"go test ./...": {exit: 0},
	}}

	got, err := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if err != nil {
		t.Fatalf("Verify err = %v", err)
	}

	want := Result{Path: path, Verdict: "PASS"}
	if got.Path != want.Path || got.Verdict != want.Verdict {
		t.Errorf("Verify() = {Path:%q Verdict:%q}, want {Path:%q Verdict:%q}",
			got.Path, got.Verdict, want.Path, want.Verdict)
	}
	if len(got.Commands) != 1 || !got.Commands[0].Passed {
		t.Errorf("Result.Commands = %+v, want exactly one passing CommandResult", got.Commands)
	}
}
