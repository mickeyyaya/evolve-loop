//go:build acs

package cycle1410

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const pairingPkg = "./internal/phasecoherence"

func goTest(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	full := append([]string{"test", "-count=1"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return string(out), code
}

func isTracked(t *testing.T, root, rel string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", rel)
	return cmd.Run() == nil
}

func TestC1410_001_PairingGreenWithUntrackedMintedProfileStub(t *testing.T) {
	root := acsassert.RepoRoot(t)

	rel := filepath.Join(".evolve", "profiles", "acs-c1410-untracked-mint-probe.json")
	abs := filepath.Join(root, rel)
	if _, err := os.Stat(abs); err == nil {
		t.Fatalf("probe stub %s already exists — refusing to clobber pre-existing state", rel)
	}
	if err := os.WriteFile(abs, []byte(`{"name":"acs-c1410-untracked-mint-probe","note":"cycle-1410 ACS probe: runtime-minted stub shape, no paired persona"}`+"\n"), 0o644); err != nil {
		t.Fatalf("mint probe stub %s: %v", rel, err)
	}
	t.Cleanup(func() { _ = os.Remove(abs) })

	if isTracked(t, root, rel) {
		t.Fatalf("probe stub %s is tracked — predicate precondition broken (it must be untracked to exercise the trackedProfiles seam)", rel)
	}
	if _, err := os.Stat(filepath.Join(root, "agents", "evolve-acs-c1410-untracked-mint-probe.md")); err == nil {
		t.Fatalf("probe persona unexpectedly exists — predicate precondition broken")
	}

	out, code := goTest(t, root, "-run", `^TestRepoPersonaProfilePairing$`, pairingPkg)
	if code != 0 {
		t.Errorf("pairing gate RED with an untracked minted profile stub present (exit=%d) — the cd49274beab2 ship|gate-block storm is not fixed:\n%s", code, out)
	}
}

func TestC1410_002_TrackedProfileFixtureRegressionGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)

	out, code := goTest(t, root, "-v", "-run", `^TestTrackedProfiles_`, pairingPkg)
	if code != 0 {
		t.Errorf("tracked-profile fixture regression RED (exit=%d):\n%s", code, out)
	}
	for _, name := range []string{
		"TestTrackedProfiles_MintedStubIsRuntimeStateNotRepoConfig",
		"TestTrackedProfiles_NonRepoDirFailsLoudly",
	} {
		if !strings.Contains(out, "=== RUN   "+name) {
			t.Errorf("fixture %s did not execute — the PR #421 regression pin was deleted or renamed:\n%s", name, out)
		}
	}
}

type consumedRecord struct {
	ID           string `json:"id"`
	Notes        string `json:"notes"`
	Verification struct {
		Commit     string `json:"commit"`
		PR         any    `json:"pr"`
		VerifiedAt string `json:"verified_at"`
		Evidence   string `json:"evidence"`
	} `json:"verification"`
}

const priorConsumed = "pipeline-defect-pipeline-blocker-2026-08-05.json"

func findCd49274Record(t *testing.T, root string) (string, consumedRecord) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox", "consumed")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".json") || n == priorConsumed {
			continue
		}
		if !strings.Contains(n, "pipeline-defect-pipeline-blocker") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, n))
		if rerr != nil {
			t.Fatalf("read %s: %v", n, rerr)
		}
		var rec consumedRecord
		if jerr := json.Unmarshal(raw, &rec); jerr != nil {
			t.Fatalf("consumed record %s is not valid JSON: %v", n, jerr)
		}
		if strings.Contains(string(raw), "cd49274beab2") {
			return filepath.Join(dir, n), rec
		}
	}
	t.Fatalf("no consumed record for the cd49274beab2 incident found in %s (excluding the prior %s) — the pipeline-defect-pipeline-blocker item was not consumed with verification evidence", dir, priorConsumed)
	return "", consumedRecord{}
}

func TestC1410_003_ConsumedRecordCarriesVerificationEvidence(t *testing.T) {
	root := acsassert.RepoRoot(t)

	prior := filepath.Join(root, ".evolve", "inbox", "consumed", priorConsumed)
	if _, err := os.Stat(prior); err != nil {
		t.Fatalf("prior consumed record %s was removed or renamed — consuming this cycle's item must ADD a record, never clobber the 96f17cfe3dfe one: %v", priorConsumed, err)
	}

	path, rec := findCd49274Record(t, root)
	base := filepath.Base(path)

	if rec.ID != "pipeline-defect-pipeline-blocker" {
		t.Errorf("%s: id=%q, want \"pipeline-defect-pipeline-blocker\"", base, rec.ID)
	}
	if !strings.Contains(rec.Verification.Commit, "7a42d30b") {
		t.Errorf("%s: verification.commit=%q does not record the merged fix commit 7a42d30b", base, rec.Verification.Commit)
	}
	if strings.TrimSpace(rec.Verification.VerifiedAt) == "" {
		t.Errorf("%s: verification.verified_at is empty — the record must timestamp WHEN the live run happened", base)
	}
	ev := strings.TrimSpace(rec.Verification.Evidence)
	if ev == "" {
		t.Errorf("%s: verification.evidence is empty — record the four-suite pack run summary", base)
	}
	for _, placeholder := range []string{"TODO", "TBD", "n/a", "N/A", "FIXME", "<fill", "pending"} {
		if strings.Contains(ev, placeholder) {
			t.Errorf("%s: verification.evidence contains placeholder %q (%q) — an unverified claim must not be recorded as verification", base, placeholder, ev)
		}
	}
	if !strings.Contains(ev, "phasecoherence") {
		t.Errorf("%s: verification.evidence=%q does not name the phasecoherence suite — the pack run that proves the gate is green must be cited", base, ev)
	}
	if !strings.Contains(rec.Notes, "cd49274beab2") && !strings.Contains(rec.Verification.Evidence, "cd49274beab2") {
		t.Errorf("%s: neither notes nor verification.evidence names fingerprint cd49274beab2", base)
	}
}

func TestC1410_004_VerificationCommitIsMergedAncestorOfHead(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, rec := findCd49274Record(t, root)

	sha := strings.TrimSpace(rec.Verification.Commit)
	if sha == "" {
		t.Fatalf("verification.commit is empty — nothing to resolve")
	}
	sha = strings.Fields(sha)[0]

	show := exec.Command("git", "-C", root, "log", "--format=%H %s", "-1", sha)
	out, err := show.CombinedOutput()
	if err != nil {
		t.Fatalf("verification.commit %q does not resolve in this repo: %v\n%s", sha, err, out)
	}
	if !strings.Contains(string(out), "#421") {
		t.Errorf("commit %q does not reference PR #421 — wrong commit recorded:\n%s", sha, out)
	}

	anc := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", sha, "HEAD")
	if err := anc.Run(); err != nil {
		t.Errorf("commit %q is not an ancestor of HEAD — the fix is not actually merged into this lane's base: %v", sha, err)
	}
}
