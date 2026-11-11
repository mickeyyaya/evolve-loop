package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func readIsolationConfig(enforce bool) Config {
	cfg := canonicalConfig()
	cfg.DenyReadPaths = []string{"/repo/.evolve/runs/cycle-7/private"}
	cfg.ReadIsolation = ReadIsolation{
		Enforce: enforce, Tag: "d42", RunsDir: "/repo/.evolve/runs", Workspace: "/repo/.evolve/runs/cycle-7",
		Grants:      []string{"", "/repo/.evolve/runs/cycle-6/retrospective-report.md"},
		ReportOnly:  []string{"", "/repo/.evolve/instincts/lessons/by-item.yaml"},
		RawSuffixes: []string{"-stdout.log", "-final-pane.txt"},
		RawFiles:    []string{"/repo/.evolve/runs/cycle-7/scout-stdout.log"},
		RawDirs:     []string{"/repo/.evolve/runs/cycle-7/.bridge"},
		SecretDirs:  []string{"/home/u/.ssh"},
		SecretFiles: []string{"/home/u/.netrc"},
		PinnedFiles: []string{"", "/repo/.evolve/runs/cycle-7/lane-scope.json"},
	}
	return cfg
}

func ruleOrder(t *testing.T, sbpl string, rules ...string) {
	t.Helper()
	last := -1
	for _, r := range rules {
		at := strings.Index(sbpl, r)
		if at < 0 || at <= last {
			t.Fatalf("rule %s is missing or out of order (at=%d, previous=%d); a later SBPL rule wins:\n%s", r, at, last, sbpl)
		}
		last = at
	}
}

func TestGenerateSBPL_EnforceDeniesOtherRunsSecretsAndRawOutputAndGrantsTheIndex(t *testing.T) {
	sbpl := GenerateSBPL(readIsolationConfig(true))
	ruleOrder(t, sbpl,
		`(allow file-read* (subpath "/repo"))`,
		`(allow file-read* file-read-data (literal "/repo/.evolve/instincts/lessons/by-item.yaml") (with report) (with message "evolve-cell=d42 rule=grant.index"))`,
		`(deny file-read-data (subpath "/repo/.evolve/runs") (with message "evolve-cell=d42 rule=read.runs.unrelated inv=I5"))`,
		`(deny file-read-data (subpath "/home/u/.ssh") (with message "evolve-cell=d42 rule=read.home.secret inv=I5"))`,
		`(deny file-read-data (literal "/home/u/.netrc") (with message "evolve-cell=d42 rule=read.home.secret inv=I5"))`,
		`(allow file-read* file-read-data (subpath "/repo/.evolve/runs/cycle-7"))`,
		`(allow file-read* file-read-data (literal "/repo/.evolve/runs/cycle-6/retrospective-report.md") (with report) (with message "evolve-cell=d42 rule=grant.index"))`,
		`(deny file-read-data (subpath "/repo/.evolve/runs/cycle-7/.bridge") (with message "evolve-cell=d42 rule=read.raw.output inv=I4"))`,
		`(deny file-read-data (require-all (subpath "/repo/.evolve/runs") (regex #"(-stdout\.log|-final-pane\.txt)$")) (with message "evolve-cell=d42 rule=read.raw.output inv=I4"))`,
		`(deny file-write* (subpath "/repo/.evolve/runs/cycle-7/.bridge") (with message "evolve-cell=d42 rule=write.raw.output inv=I4"))`,
		`(deny file-write* (require-all (subpath "/repo/.evolve/runs") (regex #"(-stdout\.log|-final-pane\.txt)$")) (with message "evolve-cell=d42 rule=write.raw.output inv=I4"))`,
		`(deny file-write* (subpath "/repo/.evolve/runs/cycle-7/inputs") (with message "evolve-cell=d42 rule=write.inputs inv=I4"))`,
		`(deny file-write* (literal "/repo/.evolve/runs/cycle-7/lane-scope.json") (with message "evolve-cell=d42 rule=write.pipeline.file inv=I5"))`,
		`(deny file-read* (subpath "/repo/.evolve/runs/cycle-7/private"))`,
	)
	if !strings.Contains(sbpl, "rule="+RuleGrantIndex+`"))`) {
		t.Fatalf("each grant names the rule %s:\n%s", RuleGrantIndex, sbpl)
	}
	if strings.Contains(sbpl, `(literal "")`) {
		t.Fatalf("an empty grant must emit no rule:\n%s", sbpl)
	}
}

func TestGenerateSBPL_ShadowReportsEachGuardedReadAndDeniesNone(t *testing.T) {
	sbpl := GenerateSBPL(readIsolationConfig(false))
	for _, want := range []string{
		`(allow file-read-data (subpath "/repo/.evolve/runs") (with report) (with message "evolve-cell=d42 rule=read.runs.unrelated inv=I5"))`,
		`(allow file-write* (subpath "/repo/.evolve/runs/cycle-7/inputs") (with report) (with message "evolve-cell=d42 rule=write.inputs inv=I4"))`,
	} {
		if !strings.Contains(sbpl, want) {
			t.Fatalf("shadow must report %s:\n%s", want, sbpl)
		}
	}
	if strings.Contains(sbpl, "(deny file-read-data (subpath \"/repo/.evolve/runs") || strings.Contains(sbpl, "(deny file-write* (subpath \"/repo/.evolve/runs/cycle-7/inputs\")") {
		t.Fatalf("shadow must deny no run, raw or inputs rule:\n%s", sbpl)
	}
}

func TestReadIsolation_ShadowDeniesTheSecretClass(t *testing.T) {
	shadow := GenerateSBPL(readIsolationConfig(false))
	for _, want := range []string{
		`(deny file-read-data (subpath "/home/u/.ssh") (with message "evolve-cell=d42 rule=read.home.secret inv=I5"))`,
		`(deny file-read-data (literal "/home/u/.netrc") (with message "evolve-cell=d42 rule=read.home.secret inv=I5"))`,
	} {
		if !strings.Contains(shadow, want) {
			t.Fatalf("shadow must deny %s:\n%s", want, shadow)
		}
	}
	if strings.Contains(shadow, `(deny file-read-data (subpath "/repo/.evolve/runs")`) {
		t.Fatalf("the runs rule stays report-only at shadow:\n%s", shadow)
	}
	if !(ReadIsolation{Workspace: "/w"}).secretsDenied() || (ReadIsolation{}).secretsDenied() {
		t.Fatal("secretsDenied is true exactly when the cell has read isolation")
	}
}

func TestSandboxWrap_ShadowMasksTheSecretClassOnLinux(t *testing.T) {
	dir := t.TempDir()
	ssh, netrc := filepath.Join(dir, ".ssh"), filepath.Join(dir, ".netrc")
	absent := filepath.Join(dir, ".git-credentials")
	if err := os.Mkdir(ssh, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(netrc, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, enforce := range []bool{false, true} {
		cfg := readIsolationConfig(enforce)
		cfg.ReadIsolation.SecretDirs = []string{ssh}
		cfg.ReadIsolation.SecretFiles = []string{netrc, absent}
		joined := strings.Join(BwrapPrefix(cfg), " ")
		for _, want := range []string{"--tmpfs " + ssh, "--ro-bind /dev/null " + netrc} {
			if !strings.Contains(joined, want) {
				t.Fatalf("enforce=%v must emit %q:\n%s", enforce, want, joined)
			}
		}
		if strings.Contains(joined, absent) {
			t.Fatalf("enforce=%v: an absent entry gets no mask:\n%s", enforce, joined)
		}
		if got := strings.Contains(joined, "--tmpfs /repo/.evolve/runs "); got != enforce {
			t.Fatalf("enforce=%v: the runs mask is Enforce-only, got %v:\n%s", enforce, got, joined)
		}
	}
}

func TestGenerateSBPL_NoWorkspaceOrNoRunsDirLeavesOutTheirRules(t *testing.T) {
	cfg := readIsolationConfig(true)
	cfg.ReadIsolation.Workspace = ""
	if sbpl := GenerateSBPL(cfg); strings.Contains(sbpl, "evolve-cell=") {
		t.Fatalf("a cell without a workspace gets no read isolation:\n%s", sbpl)
	}
	cfg = readIsolationConfig(true)
	cfg.ReadIsolation.RunsDir, cfg.ReadIsolation.RawSuffixes = "", nil
	sbpl := GenerateSBPL(cfg)
	if strings.Contains(sbpl, "read.runs.unrelated") || strings.Contains(sbpl, "regex #\"(") {
		t.Fatalf("no runs directory means no runs deny and no raw pattern:\n%s", sbpl)
	}
	if !strings.Contains(sbpl, "rule=write.inputs") {
		t.Fatalf("the inputs write deny does not need the runs directory:\n%s", sbpl)
	}
}

func TestBwrapPrefix_EnforceHidesOtherRunsAndRebindsTheOwnWorkspaceAndTheIndex(t *testing.T) {
	home := t.TempDir()
	ssh, netrc := filepath.Join(home, ".ssh"), filepath.Join(home, ".netrc")
	if err := os.Mkdir(ssh, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(netrc, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := readIsolationConfig(true)
	cfg.ReadIsolation.SecretDirs, cfg.ReadIsolation.SecretFiles = []string{ssh}, []string{netrc}
	argv := BwrapPrefix(cfg)
	joined := strings.Join(argv, " ")
	for _, want := range []string{
		"--tmpfs /repo/.evolve/runs --bind /repo/.evolve/runs/cycle-7 /repo/.evolve/runs/cycle-7",
		"--ro-bind-try /repo/.evolve/runs/cycle-6/retrospective-report.md /repo/.evolve/runs/cycle-6/retrospective-report.md",
		"--ro-bind /dev/null /repo/.evolve/runs/cycle-7/scout-stdout.log",
		"--tmpfs /repo/.evolve/runs/cycle-7/.bridge",
		"--ro-bind-try /repo/.evolve/runs/cycle-7/inputs /repo/.evolve/runs/cycle-7/inputs",
		"--tmpfs " + ssh,
		"--ro-bind /dev/null " + netrc,
		"--ro-bind-try /repo/.evolve/runs/cycle-7/lane-scope.json /repo/.evolve/runs/cycle-7/lane-scope.json",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("enforce must emit %q:\n%s", want, joined)
		}
	}
	ruleOrder(t, joined, "--bind /repo /repo", "--tmpfs /repo/.evolve/runs", "--tmpfs /repo/.evolve/runs/cycle-7/private")
	if slices.Contains(argv, "") {
		t.Fatalf("an empty grant must emit no bind: %q", argv)
	}
}

func TestBwrapPrefix_ShadowOrNoWorkspaceBindsNothingForReadIsolation(t *testing.T) {
	for _, cfg := range []Config{readIsolationConfig(false), func() Config {
		c := readIsolationConfig(true)
		c.ReadIsolation.Workspace = ""
		return c
	}()} {
		if joined := strings.Join(BwrapPrefix(cfg), " "); strings.Contains(joined, ".ssh") || strings.Contains(joined, "--ro-bind-try") {
			t.Fatalf("no read isolation binds expected: %s", joined)
		}
	}
	cfg := readIsolationConfig(true)
	cfg.ReadIsolation.RunsDir = ""
	if joined := strings.Join(BwrapPrefix(cfg), " "); strings.Contains(joined, "--tmpfs /repo/.evolve/runs ") {
		t.Fatalf("no runs directory means no tmpfs over it: %s", joined)
	}
}

func TestReadIsolation_RuleIDsAreTheSignalRulesOfTheSpec(t *testing.T) {
	got := []string{RuleRunsUnrelated, RuleHomeSecret, RuleRawOutput, RuleRawOutputWrite, RuleInputsWrite, RulePipelineFileWrite, CellMessagePrefix}
	want := []string{"read.runs.unrelated", "read.home.secret", "read.raw.output", "write.raw.output", "write.inputs", "write.pipeline.file", "evolve-cell="}
	if !slices.Equal(got, want) {
		t.Fatalf("rule ids = %v, want %v (llm-isolation.md §15.2)", got, want)
	}
}
