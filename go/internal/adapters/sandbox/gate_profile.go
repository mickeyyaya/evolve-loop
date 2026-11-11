package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	RuleGateGit     = "gate.git"
	RuleGateExec    = "gate.exec"
	RuleGateSecret  = "gate.secret"
	RuleGateNetwork = "gate.network"
	RuleGateDefault = "gate.default"
	RuleGateCanary  = "gate.canary"
	CanaryStart     = ".gate-canary-start"
	CanaryEnd       = ".gate-canary-end"

	gateInvariant = "I0"
	execAllowNone = `(require-not (require-all (signing-identifier "com.apple.sandbox-exec") (process-attribute is-platform-binary)))`
)

var gateSystemReads = []string{
	"/usr/lib", "/System/Library", "/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld", "/private/var/db/dyld",
}

var HomeSecrets = []string{".ssh", ".aws", ".gnupg", ".netrc", ".config/gh", ".docker/config.json", ".kube"}

type GitEntry struct {
	Path string
	Dir  bool
}

func (e GitEntry) filter() string {
	if e.Dir {
		return fmt.Sprintf("(subpath %q)", e.Path)
	}
	return fmt.Sprintf("(literal %q)", e.Path)
}

type GateConfig struct {
	Lane      string
	Binary    string
	Policy    string
	Scratch   string
	HomeDir   string
	Tag       string
	Report    bool
	Libraries []string
}

func canonical(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

func GitEntries(lane string) []GitEntry {
	lane = canonical(lane)
	file := filepath.Join(lane, ".git")
	info, err := os.Stat(file)
	if err != nil || info.IsDir() {
		return []GitEntry{{Path: file, Dir: err == nil}}
	}
	entries := []GitEntry{{Path: file}}
	raw, _ := os.ReadFile(file)
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
	if !ok {
		return entries
	}
	gitdir = canonical(within(lane, strings.TrimSpace(gitdir)))
	entries = append(entries, GitEntry{Path: gitdir, Dir: true})
	common, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return entries
	}
	return append(entries, GitEntry{Path: canonical(within(gitdir, strings.TrimSpace(string(common)))), Dir: true})
}

func within(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func gateSecrets(home string) []string {
	if home == "" {
		return nil
	}
	out := make([]string, len(HomeSecrets))
	for i, rel := range HomeSecrets {
		out[i] = canonical(filepath.Join(home, rel))
	}
	return out
}

type gateWriter struct {
	b      strings.Builder
	report bool
	tag    string
}

func (w *gateWriter) rule(op, filter, rule string) {
	if w.report {
		fmt.Fprintf(&w.b, "(allow %s (with report) (with message %q))\n", strings.TrimSpace(op+" "+filter), fmt.Sprintf("%s%s rule=%s inv=%s", CellMessagePrefix, w.tag, rule, gateInvariant))
		return
	}
	fmt.Fprintf(&w.b, "(deny %s)\n", strings.TrimSpace(op+" "+filter))
}

func GateProfile(cfg GateConfig) string {
	w := &gateWriter{report: cfg.Report, tag: cfg.Tag}
	lane, scratch := canonical(cfg.Lane), canonical(cfg.Scratch)
	w.b.WriteString("(version 1)\n")
	w.rule("default", "", RuleGateDefault)
	w.b.WriteString("(import \"system.sb\")\n(allow process-exec)\n(allow process-fork)\n(allow signal (target same-sandbox))\n(allow sysctl-read)\n(allow mach-lookup)\n(allow ipc-posix-shm)\n(allow file-read-metadata)\n")
	for _, p := range gateSystemReads {
		fmt.Fprintf(&w.b, "(allow file-read* (subpath %q))\n", p)
	}
	fmt.Fprintf(&w.b, "(allow file-read* (subpath %q))\n", lane)
	fmt.Fprintf(&w.b, "(allow file-read* (literal %q))\n", canonical(cfg.Binary))
	fmt.Fprintf(&w.b, "(allow file-read* (literal %q))\n", filepath.Dir(canonical(cfg.Binary)))
	fmt.Fprintf(&w.b, "(allow file-read* (literal %q))\n", canonical(cfg.Policy))
	fmt.Fprintf(&w.b, "(allow file-read* file-write* (literal %q))\n", os.DevNull)
	fmt.Fprintf(&w.b, "(allow file-read* file-write* (subpath %q))\n", scratch)
	if w.report {
		for _, name := range []string{CanaryStart, CanaryEnd} {
			w.rule("file-read*", fmt.Sprintf("(literal %q)", filepath.Join(scratch, name)), RuleGateCanary)
		}
	}
	for _, entry := range GitEntries(cfg.Lane) {
		w.rule("file-read*", entry.filter(), RuleGateGit)
	}
	for _, secret := range gateSecrets(cfg.HomeDir) {
		w.rule("file-read*", fmt.Sprintf("(subpath %q)", secret), RuleGateSecret)
	}
	w.rule("process-exec", execAllowNone, RuleGateExec)
	w.rule("network*", "", RuleGateNetwork)
	return w.b.String()
}

func GateBwrapPrefix(cfg GateConfig) []string {
	lane := canonical(cfg.Lane)
	out := BwrapPrefix(Config{
		RepoRoot: lane, ReadOnlyRepo: true, WritePaths: []string{canonical(cfg.Scratch)},
		ExecDenyAll: true, ExecBinary: canonical(cfg.Binary), ExecLibraries: cfg.Libraries,
	})
	out = append(out, "--ro-bind", canonical(cfg.Policy), canonical(cfg.Policy), "--cap-drop", "ALL")
	for _, entry := range GitEntries(cfg.Lane) {
		if entry.Dir {
			out = append(out, "--tmpfs", entry.Path)
			continue
		}
		out = append(out, "--ro-bind", os.DevNull, entry.Path)
	}
	return out
}

func ParseLibraries(lddOutput string) []string {
	var out []string
	for _, line := range strings.Split(lddOutput, "\n") {
		line = strings.TrimSpace(line)
		if _, after, ok := strings.Cut(line, "=>"); ok {
			line = strings.TrimSpace(after)
		}
		if !strings.HasPrefix(line, "/") {
			continue
		}
		if path, _, _ := strings.Cut(line, " "); path != "" {
			out = append(out, path)
		}
	}
	return out
}
