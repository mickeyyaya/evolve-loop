package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type gateLane struct {
	root, scratch, binary, policy, home string
	gitdir, common                      string
}

func newGateLane(t *testing.T) gateLane {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	g := gateLane{
		root: filepath.Join(base, "lane"), scratch: filepath.Join(base, "scratch"), binary: filepath.Join(base, "evolve"),
		policy: filepath.Join(base, "policy.json"), home: filepath.Join(base, "home"),
		common: filepath.Join(base, "main.git"),
	}
	g.gitdir = filepath.Join(g.common, "worktrees", "lane")
	for _, d := range []string{g.root, g.scratch, g.gitdir, g.home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(g.root, ".git"), []byte("gitdir: "+g.gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.gitdir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return g
}

func (g gateLane) config() GateConfig {
	return GateConfig{Lane: g.root, Binary: g.binary, Policy: g.policy, Scratch: g.scratch, HomeDir: g.home, Tag: "gaterun-1"}
}

func TestGitEntries_ALinkedWorktreeNamesTheFileTheGitDirAndTheCommonDir(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	got := GitEntries(g.root)
	want := []GitEntry{{Path: filepath.Join(g.root, ".git")}, {Path: g.gitdir, Dir: true}, {Path: g.common, Dir: true}}
	if !slices.Equal(got, want) {
		t.Errorf("GitEntries = %v, want %v", got, want)
	}
}

func TestGitEntries_AGitDirectoryIsTheOnlyEntryAndAMissingOneGivesTheDefaultPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	canonical, _ := filepath.EvalSymlinks(root)
	if got := GitEntries(root); !slices.Equal(got, []GitEntry{{Path: filepath.Join(canonical, ".git"), Dir: true}}) {
		t.Errorf("GitEntries(dir) = %v", got)
	}
	bare := t.TempDir()
	canonicalBare, _ := filepath.EvalSymlinks(bare)
	if got := GitEntries(bare); !slices.Equal(got, []GitEntry{{Path: filepath.Join(canonicalBare, ".git")}}) {
		t.Errorf("GitEntries(no .git) = %v", got)
	}
}

func TestGitEntries_AGitFileWithNoGitdirLineOrACommondirThatIsMissingKeepsWhatItHas(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := GitEntries(root); len(got) != 1 {
		t.Errorf("GitEntries(junk pointer) = %v, want the file only", got)
	}
	gitdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := GitEntries(root); len(got) != 2 {
		t.Errorf("GitEntries(no commondir) = %v, want the file and the git dir", got)
	}
}

func TestGateProfile_TheOrderIsSystemSetThenGrantsThenGitDeniesThenTheRest(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	sbpl := GateProfile(g.config())
	order := []string{
		`(deny default)`,
		`(allow file-read* (subpath "/usr/lib"))`,
		`(allow file-read* (subpath "/System/Library"))`,
		`(allow file-read* (subpath "` + g.root + `"))`,
		`(allow file-read* (literal "` + g.binary + `"))`,
		`(allow file-read* (literal "` + filepath.Dir(g.binary) + `"))`,
		`(allow file-read* (literal "` + g.policy + `"))`,
		`(allow file-read* file-write* (subpath "` + g.scratch + `"))`,
		`(deny file-read* (literal "` + filepath.Join(g.root, ".git") + `"))`,
		`(deny file-read* (subpath "` + g.gitdir + `"))`,
		`(deny file-read* (subpath "` + g.common + `"))`,
		`(deny file-read* (subpath "` + filepath.Join(g.home, ".ssh") + `"))`,
		`(deny process-exec (require-not (require-all (signing-identifier "com.apple.sandbox-exec") (process-attribute is-platform-binary))))`,
		`(deny network*)`,
	}
	last := -1
	for _, want := range order {
		at := strings.Index(sbpl, want)
		if at < 0 || at < last {
			t.Fatalf("%q must come after the rule before it (at=%d last=%d):\n%s", want, at, last, sbpl)
		}
		last = at
	}
	for _, secret := range []string{".aws", ".gnupg", ".netrc", ".config/gh", ".docker/config.json", ".kube"} {
		if !strings.Contains(sbpl, filepath.Join(g.home, secret)) {
			t.Errorf("the secret %s must be denied:\n%s", secret, sbpl)
		}
	}
	if strings.Contains(sbpl, "with report") {
		t.Errorf("enforce must not report:\n%s", sbpl)
	}
}

func TestGateProfile_ReportModeAllowsEachDenyWithAReportAndTheTag(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	cfg := g.config()
	cfg.Report = true
	sbpl := GateProfile(cfg)
	for _, want := range []string{
		`(allow file-read* (literal "` + filepath.Join(g.root, ".git") + `") (with report) (with message "evolve-cell=gaterun-1 rule=gate.git inv=I0"))`,
		`(allow process-exec (require-not (require-all (signing-identifier "com.apple.sandbox-exec") (process-attribute is-platform-binary))) (with report) (with message "evolve-cell=gaterun-1 rule=gate.exec inv=I0"))`,
		`(allow network* (with report) (with message "evolve-cell=gaterun-1 rule=gate.network inv=I0"))`,
		`(allow default (with report) (with message "evolve-cell=gaterun-1 rule=gate.default inv=I0"))`,
		`rule=gate.secret inv=I0"))`,
	} {
		if !strings.Contains(sbpl, want) {
			t.Errorf("missing %s in:\n%s", want, sbpl)
		}
	}
	if strings.Contains(sbpl, "(deny ") {
		t.Errorf("report mode must deny nothing:\n%s", sbpl)
	}
}

func TestGateProfile_ReportModeDiffersFromEnforceOnlyInTheVerbOfEachDeny(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	cfg := g.config()
	enforce := GateProfile(cfg)
	cfg.Report = true
	report := GateProfile(cfg)
	if !strings.HasPrefix(enforce, "(version 1)\n(deny default)\n(import \"system.sb\")\n") {
		t.Errorf("enforce must start with the default deny:\n%s", enforce)
	}
	if !strings.HasPrefix(report, "(version 1)\n(allow default (with report) (with message \"evolve-cell=gaterun-1 rule=gate.default inv=I0\"))\n(import \"system.sb\")\n") {
		t.Errorf("report mode must report the default deny first, so that each later grant wins silently as it does at enforce:\n%s", report)
	}
	enforceLines, reportLines := strings.Split(enforce, "\n"), slices.DeleteFunc(strings.Split(report, "\n"), func(l string) bool { return strings.Contains(l, "rule="+RuleGateCanary+" ") })
	if len(enforceLines) != len(reportLines) {
		t.Fatalf("the two modes must have the same rules: %d against %d lines", len(enforceLines), len(reportLines))
	}
	for i, line := range enforceLines {
		denied := strings.HasPrefix(line, "(deny ")
		reported := strings.HasPrefix(reportLines[i], "(allow ") && strings.Contains(reportLines[i], "(with report)")
		if (denied && !reported) || (!denied && line != reportLines[i]) {
			t.Errorf("line %d: enforce %q, report %q: only a deny may change, and only to an allow with a report", i, line, reportLines[i])
		}
	}
}

func TestGateProfile_AGitDirectoryIsDeniedAsASubpathAndMaskedWithATmpfs(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	if err := os.Remove(filepath.Join(g.root, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(g.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(g.root, ".git")
	if sbpl := GateProfile(g.config()); !strings.Contains(sbpl, `(deny file-read* (subpath "`+gitDir+`"))`) {
		t.Errorf("a literal deny admits .git/HEAD through the lane grant; want a subpath deny:\n%s", sbpl)
	}
	argv := GateBwrapPrefix(g.config())
	at := slices.Index(argv, gitDir)
	if at < 1 || argv[at-1] != "--tmpfs" {
		t.Errorf("bwrap cannot bind /dev/null over a directory; want --tmpfs %s: %q", gitDir, argv)
	}
}

func TestGitEntries_ARelativeGitdirResolvesAgainstTheLane(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	rel, err := filepath.Rel(g.root, g.gitdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.root, ".git"), []byte("gitdir: "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []GitEntry{{Path: filepath.Join(g.root, ".git")}, {Path: g.gitdir, Dir: true}, {Path: g.common, Dir: true}}
	if got := GitEntries(g.root); !slices.Equal(got, want) {
		t.Errorf("GitEntries = %v, want %v: a relative gitdir is relative to the lane, not to the process", got, want)
	}
}

func TestGateProfile_ReportModeNamesEachRuleThatTheReportReaderMaps(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	cfg := g.config()
	cfg.Report = true
	sbpl := GateProfile(cfg)
	for _, rule := range []string{RuleGateGit, RuleGateExec, RuleGateSecret, RuleGateNetwork, RuleGateDefault} {
		if !strings.Contains(sbpl, " rule="+rule+" ") {
			t.Errorf("report mode must tag the rule %s:\n%s", rule, sbpl)
		}
	}
}

func TestGateProfile_ABlankHomeAddsNoSecretRule(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	cfg := g.config()
	cfg.HomeDir = ""
	if sbpl := GateProfile(cfg); strings.Contains(sbpl, "gate.secret") || strings.Contains(sbpl, ".ssh") {
		t.Errorf("no home, no secret rule:\n%s", sbpl)
	}
}

func TestGateBwrapPrefix_BindsOnlyTheBinaryItsLibrariesTheLaneAndTheScratch(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	cfg := g.config()
	cfg.Libraries = []string{"/lib/x86_64-linux-gnu/libc.so.6"}
	argv := GateBwrapPrefix(cfg)
	joined := strings.Join(argv, " ")
	for _, want := range []string{
		"--unshare-net", "--unshare-pid", "--cap-drop ALL",
		"--ro-bind " + g.binary + " " + g.binary,
		"--ro-bind /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libc.so.6",
		"--ro-bind " + g.root + " " + g.root,
		"--ro-bind " + g.policy + " " + g.policy,
		"--bind " + g.scratch + " " + g.scratch,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	for _, banned := range []string{"/usr /usr", "/bin /bin", "/etc /etc"} {
		if strings.Contains(joined, banned) {
			t.Errorf("the root must hold no %q: %q", banned, joined)
		}
	}
}

func TestGateBwrapPrefix_TheGitEntriesAreMaskedAfterEveryBind(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	argv := GateBwrapPrefix(g.config())
	lane := slices.Index(argv, g.root)
	file := slices.Index(argv, filepath.Join(g.root, ".git"))
	dir := slices.Index(argv, g.gitdir)
	common := slices.Index(argv, g.common)
	if lane < 0 || file < lane || dir < lane || common < lane {
		t.Fatalf("the masks must follow the lane bind: lane=%d file=%d dir=%d common=%d in %q", lane, file, dir, common, argv)
	}
	if argv[file-2] != "--ro-bind" || argv[file-1] != os.DevNull || argv[dir-1] != "--tmpfs" {
		t.Errorf("the .git file is masked with /dev/null and the dirs with tmpfs: %q", argv)
	}
	if home := slices.Index(argv, filepath.Join(g.home, ".ssh")); home >= 0 {
		t.Errorf("home is not bound, so a secret needs no mask: %q", argv)
	}
}

func TestParseLibraries_ReadsTheResolvedPathsOfLdd(t *testing.T) {
	t.Parallel()
	out := "\tlinux-vdso.so.1 (0x00007ffd)\n\tlibc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f)\n\t/lib64/ld-linux-x86-64.so.2 (0x00007f)\n\tlibgone.so => not found\n"
	got := ParseLibraries(out)
	want := []string{"/lib/x86_64-linux-gnu/libc.so.6", "/lib64/ld-linux-x86-64.so.2"}
	if !slices.Equal(got, want) {
		t.Errorf("ParseLibraries = %v, want %v", got, want)
	}
	if got := ParseLibraries("\tnot a dynamic executable\n"); len(got) != 0 {
		t.Errorf("a static binary has no libraries: %v", got)
	}
}

func TestGenerateSBPL_ExecDenyAllWritesTheOneRuleAfterTheExecGrant(t *testing.T) {
	t.Parallel()
	cfg := canonicalConfig()
	cfg.ExecDenyAll = true
	sbpl := GenerateSBPL(cfg)
	grant := strings.Index(sbpl, "(allow process-exec)\n")
	rule := strings.Index(sbpl, `(deny process-exec (require-not (require-all (signing-identifier "com.apple.sandbox-exec") (process-attribute is-platform-binary))))`)
	if rule < 0 || rule < grant {
		t.Fatalf("the exec-deny-all rule must follow the grant:\n%s", sbpl)
	}
	cfg.ExecDenyEnforce = false
	cfg.ExecDenyPaths = []string{"/opt/llm/claude"}
	if sbpl := GenerateSBPL(cfg); strings.Count(sbpl, `(deny process-exec (require-not (require-all`) != 1 {
		t.Errorf("the rule is written once:\n%s", sbpl)
	}
}

func TestBwrapPrefix_ExecDenyAllBindsOnlyTheExecBinaryAndItsLibraries(t *testing.T) {
	t.Parallel()
	argv := BwrapPrefix(Config{RepoRoot: "/repo", ExecDenyAll: true, ExecBinary: "/opt/evolve", ExecLibraries: []string{"/lib/libc.so.6"}})
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--ro-bind /opt/evolve /opt/evolve", "--ro-bind /lib/libc.so.6 /lib/libc.so.6", "--unshare-net"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	for _, banned := range []string{"/usr /usr", "/bin /bin", "/etc /etc", "/lib /lib "} {
		if strings.Contains(joined, banned) {
			t.Errorf("exec deny all must leave no %q in the root: %q", banned, joined)
		}
	}
	if joined := strings.Join(BwrapPrefix(Config{RepoRoot: "/repo", ExecDenyAll: true}), " "); strings.Contains(joined, "--ro-bind  ") {
		t.Errorf("a missing binary adds no bind: %q", joined)
	}
}

func TestGateProfile_ReportModeTagsTheTwoCanariesAfterTheScratchGrant(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	cfg := g.config()
	if strings.Contains(GateProfile(cfg), RuleGateCanary) {
		t.Error("enforce reads no reports, so it tags no canary")
	}
	cfg.Report = true
	sbpl := GateProfile(cfg)
	grant := strings.Index(sbpl, `(allow file-read* file-write* (subpath "`+g.scratch+`"))`)
	for _, name := range []string{CanaryStart, CanaryEnd} {
		rule := `(allow file-read* (literal "` + filepath.Join(g.scratch, name) + `") (with report) (with message "evolve-cell=gaterun-1 rule=gate.canary inv=I0"))`
		if at := strings.Index(sbpl, rule); at < 0 || at < grant {
			t.Errorf("the canary %s must be tagged after the scratch grant, or the grant wins and no report proves delivery:\n%s", name, sbpl)
		}
	}
}

func TestHomeSecrets_NamesEachCredentialStoreThatTheGateAndTheCellDeny(t *testing.T) {
	t.Parallel()
	for _, secret := range []string{".ssh", ".aws", ".gnupg", ".netrc", ".config/gh", ".docker/config.json", ".kube"} {
		if !slices.Contains(HomeSecrets, secret) {
			t.Errorf("HomeSecrets lost %s: both the gate profile and the bridge cell read this one list", secret)
		}
	}
}

func TestGateProfile_ASymlinkedSecretIsDeniedAtItsTarget(t *testing.T) {
	t.Parallel()
	g := newGateLane(t)
	target := filepath.Join(filepath.Dir(g.home), "dotfiles", "ssh")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(g.home, ".ssh")); err != nil {
		t.Fatal(err)
	}
	if sbpl := GateProfile(g.config()); !strings.Contains(sbpl, `(deny file-read* (subpath "`+target+`"))`) {
		t.Errorf("the sandbox matches the resolved path, so a symlinked ~/.ssh must be denied at %s:\n%s", target, sbpl)
	}
}
