//go:build acs

package cycle1303

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
)

func sentinel(verdict string) string {
	return fmt.Sprintf(`<!-- evolve-verdict: {"phase":"audit","verdict":%q,"schema_version":1} -->`, verdict)
}

func placeholderSentinel(verdict string) string {
	return fmt.Sprintf(`<!-- evolve-verdict: {"phase":"audit","verdict":%q,"schema_version":2,`+
		`"failure":{"class":"<failure class>","defects":["<one line per defect>"],`+
		`"evidence_paths":["<artifact path>"]}} -->`, verdict)
}

func makeRepo(t *testing.T, auditBody string) string {
	t.Helper()
	root := t.TempDir()
	auditPath := filepath.Join(root, ".evolve", "runs", "cycle-99", "audit-report.md")
	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"),
		[]byte(`{"name":"x","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(auditBody), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := fmt.Sprintf(
		`{"ts":%q,"cycle":99,"role":"auditor","kind":"agent_subprocess","model":"opus",`+
			`"exit_code":0,"artifact_path":%q,"artifact_sha256":"deadbeef",`+
			`"git_head":"none","tree_state_sha":"none"}`+"\n",
		time.Now().UTC().Format(time.RFC3339), auditPath)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "ledger.jsonl"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func preflight(t *testing.T, auditBody string, strict bool) (verdict string, err error) {
	t.Helper()
	res, err := releasepreflight.Run(releasepreflight.Options{
		Target:         "1.0.1",
		RepoRoot:       makeRepo(t, auditBody),
		SkipTests:      true,
		StrictPass:     strict,
		Stderr:         &strings.Builder{},
		Now:            time.Now,
		GitClean:       func(string) (bool, error) { return true, nil },
		CurrentBranch:  func(string) (string, error) { return "main", nil },
		GateTestRunner: func(string, string) error { return nil },
		CIConclusion: func(string) (releasepreflight.CIRunStatus, error) {
			return releasepreflight.CIRunStatus{}, nil
		},
	})
	return res.AuditVerdict, err
}

const realFailThenPlaceholderEcho = "# Audit — cycle 99\n\n" +
	"The change regresses the ship gate.\n\n" +
	sentinelFail + "\n\n" +
	"## Deliverable Contract (echoed from the prompt)\n\n" +
	placeholderPass + "\n"

const (
	sentinelFail    = `<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":1} -->`
	sentinelPass    = `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->`
	sentinelWarn    = `<!-- evolve-verdict: {"phase":"audit","verdict":"WARN","schema_version":1} -->`
	placeholderPass = `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":2,` +
		`"failure":{"class":"<failure class>","defects":["<one line per defect>"],` +
		`"evidence_paths":["<artifact path>"]}} -->`
	placeholderFail = `<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":2,` +
		`"failure":{"class":"<failure class>","defects":["<one line per defect>"]}} -->`
)

func TestC1303_001_placeholder_echo_cannot_override_real_fail(t *testing.T) {
	verdict, err := preflight(t, realFailThenPlaceholderEcho, false)
	if !errors.Is(err, releasepreflight.ErrCheckFailed) {
		t.Fatalf("a real FAIL sentinel trailed by a contract-example placeholder echo must BLOCK the "+
			"release (the placeholder is never a real agent verdict — sentinel.go:62-74); "+
			"Run returned verdict=%q err=%v, want ErrCheckFailed", verdict, err)
	}
	if s, ok := phasecontract.ParseVerdictSentinelFull(realFailThenPlaceholderEcho); !ok || s.Verdict != "FAIL" {
		t.Fatalf("fixture drift: ParseVerdictSentinelFull(fixture) = (%+v, %v), want verdict FAIL", s, ok)
	}
}

func TestC1303_002_sole_placeholder_echo_is_not_a_marker(t *testing.T) {
	body := "# Audit — cycle 99\n\nVerdict: PASS\n\nConfidence: 1.0\n\n" +
		"## Deliverable Contract (echoed from the prompt)\n\n" + placeholderFail + "\n"
	if s, ok := phasecontract.ParseVerdictSentinelFull(body); ok {
		t.Fatalf("fixture drift: the SSOT must find NO valid sentinel in a placeholder-only body, got %+v", s)
	}
	verdict, err := preflight(t, body, false)
	if err != nil {
		t.Fatalf("a body whose ONLY sentinel is a contract-example placeholder echo has no machine "+
			"marker, so the prose 'Verdict: PASS' must govern and the release must proceed; "+
			"Run err = %v", err)
	}
	if verdict != "PASS" {
		t.Errorf("AuditVerdict = %q, want PASS from the prose fallback", verdict)
	}
}

type oracleCase struct {
	name string
	body string
}

func TestC1303_003_release_verdict_matches_phasecontract_oracle(t *testing.T) {
	cases := []oracleCase{
		{"lone-pass", "# Audit\n\n" + sentinelPass + "\n"},
		{"lone-warn", "# Audit\n\n" + sentinelWarn + "\n"},
		{"lone-fail", "# Audit\n\n" + sentinelFail + "\n"},
		{"quoted-pass-then-real-fail", "# Audit\n\nPrior cycle said:\n" + sentinelPass + "\n\n" + sentinelFail + "\n"},
		{"real-fail-then-placeholder-pass", realFailThenPlaceholderEcho},
		{"real-pass-then-placeholder-fail", "# Audit\n\n" + sentinelPass + "\n\nContract example:\n" + placeholderFail + "\n"},
		{"malformed-then-real-pass", "# Audit\n\n<!-- evolve-verdict: {not json} -->\n\n" + sentinelPass + "\n"},
		{"empty-verdict-then-real-pass", "# Audit\n\n" +
			`<!-- evolve-verdict: {"phase":"audit","verdict":"","schema_version":1} -->` + "\n\n" + sentinelPass + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, ok := phasecontract.ParseVerdictSentinelFull(tc.body)
			if !ok {
				t.Skipf("no valid sentinel in fixture %q — nothing for the oracle to compare", tc.name)
			}
			verdict, err := preflight(t, tc.body, false)
			switch want.Verdict {
			case "PASS", "WARN":
				if err != nil {
					t.Fatalf("SSOT verdict %q must release; release-preflight blocked with %v "+
						"(the two parsers disagree — releasepreflight is not delegating)", want.Verdict, err)
				}
				if verdict != want.Verdict {
					t.Errorf("release-preflight acted on verdict %q, SSOT says %q", verdict, want.Verdict)
				}
			default:
				if !errors.Is(err, releasepreflight.ErrCheckFailed) {
					t.Fatalf("SSOT verdict %q must BLOCK; release-preflight returned verdict=%q err=%v "+
						"(the two parsers disagree — releasepreflight is not delegating)",
						want.Verdict, verdict, err)
				}
			}
		})
	}
}

func TestC1303_004_extract_verdict_behavior_preserved(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		strict    bool
		wantErr   bool
		wantVerd  string
		rationale string
	}{
		{"marker-pass", "# Audit\n\n" + sentinelPass + "\n", false, false, "PASS",
			"a lone PASS marker releases"},
		{"marker-warn-nonstrict", "# Audit\n\n" + sentinelWarn + "\n", false, false, "WARN",
			"WARN releases under the fluent posture"},
		{"marker-warn-strict", "# Audit\n\n" + sentinelWarn + "\n", true, true, "",
			"EVOLVE_RELEASE_STRICT_PASS rejects WARN"},
		{"marker-fail", "# Audit\n\n" + sentinelFail + "\n", false, true, "",
			"a FAIL marker blocks"},
		{"marker-fail-beats-later-prose-pass", "# Audit\n\n" + sentinelFail + "\n\nVerdict: PASS\n", false, true, "",
			"a present marker is authoritative — prose must not override it"},
		{"last-marker-wins", "# Audit\n\nPrior cycle:\n" + sentinelPass + "\n\n" + sentinelFail + "\n", false, true, "",
			"a quoted earlier PASS can never silence the report's own later FAIL"},
		{"prose-inline-pass", "# Audit\n\nVerdict: PASS\n\nConfidence: 1.0\n", false, false, "PASS",
			"no marker → inline prose fallback still works"},
		{"prose-heading-pass", "# Audit\n\n## Verdict\n\n**PASS**\n", false, false, "PASS",
			"no marker → heading prose fallback still works"},
		{"no-verdict-at-all", "# Audit\n\nNothing conclusive here.\n", false, true, "",
			"an artifact declaring no verdict blocks the release"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict, err := preflight(t, tc.body, tc.strict)
			if tc.wantErr {
				if !errors.Is(err, releasepreflight.ErrCheckFailed) {
					t.Fatalf("%s: want ErrCheckFailed, got verdict=%q err=%v", tc.rationale, verdict, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: want release, got err = %v", tc.rationale, err)
			}
			if verdict != tc.wantVerd {
				t.Errorf("%s: AuditVerdict = %q, want %q", tc.rationale, verdict, tc.wantVerd)
			}
		})
	}
}

func TestC1303_005_owning_package_suites_green(t *testing.T) {
	goDir := repoGoDir(t)
	for _, pkg := range []string{"./internal/releasepreflight", "./internal/phasecontract"} {
		cmd := exec.Command("go", "test", "-count=1", pkg)
		cmd.Dir = goDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("go test %s failed: %v\n%s", pkg, err, out)
		}
	}
}

func repoGoDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := dir; d != string(filepath.Separator); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatalf("no go.mod found walking up from %s", dir)
	return ""
}

var _ = []func(string) string{sentinel, placeholderSentinel}
