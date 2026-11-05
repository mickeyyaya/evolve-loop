package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	RuleRunsUnrelated     = "read.runs.unrelated"
	RuleHomeSecret        = "read.home.secret"
	RuleRawOutput         = "read.raw.output"
	RuleRawOutputWrite    = "write.raw.output"
	RuleInputsWrite       = "write.inputs"
	RulePipelineFileWrite = "write.pipeline.file"
	RuleGrantIndex        = "grant.index"
	CellMessagePrefix     = "evolve-cell="
)

type ReadIsolation struct {
	Enforce       bool
	SecretEnforce bool
	Tag           string
	RunsDir       string
	Workspace     string
	Grants        []string
	ReportOnly    []string
	RawSuffixes   []string
	RawFiles      []string
	RawDirs       []string
	SecretDirs    []string
	SecretFiles   []string
	PinnedFiles   []string
}

func (ri ReadIsolation) inputsDir() string {
	return filepath.Join(ri.Workspace, "inputs")
}

func (ri ReadIsolation) secretsDenied() bool {
	return ri.Enforce || ri.SecretEnforce
}

func (ri ReadIsolation) secretScope() ReadIsolation {
	scope := ri
	scope.Enforce = ri.secretsDenied()
	return scope
}

func (ri ReadIsolation) guard(b *strings.Builder, op, filter, rule, invariant string) {
	message := fmt.Sprintf("%s%s rule=%s inv=%s", CellMessagePrefix, ri.Tag, rule, invariant)
	if ri.Enforce {
		fmt.Fprintf(b, "(deny %s %s (with message %q))\n", op, filter, message)
		return
	}
	fmt.Fprintf(b, "(allow %s %s (with report) (with message %q))\n", op, filter, message)
}

func writeReadIsolation(b *strings.Builder, ri ReadIsolation) {
	if ri.Workspace == "" {
		return
	}
	grantMessage := fmt.Sprintf("%s%s rule=%s", CellMessagePrefix, ri.Tag, RuleGrantIndex)
	writeGrantReports(b, ri.ReportOnly, grantMessage)
	if ri.RunsDir != "" {
		ri.guard(b, "file-read-data", fmt.Sprintf("(subpath %q)", ri.RunsDir), RuleRunsUnrelated, "I5")
	}
	secrets := ri.secretScope()
	for _, d := range ri.SecretDirs {
		secrets.guard(b, "file-read-data", fmt.Sprintf("(subpath %q)", d), RuleHomeSecret, "I5")
	}
	for _, f := range ri.SecretFiles {
		secrets.guard(b, "file-read-data", fmt.Sprintf("(literal %q)", f), RuleHomeSecret, "I5")
	}
	fmt.Fprintf(b, "(allow file-read* file-read-data (subpath %q))\n", ri.Workspace)
	writeGrantReports(b, ri.Grants, grantMessage)
	ri.writeRawGuards(b, "file-read-data", RuleRawOutput)
	ri.writeRawGuards(b, "file-write*", RuleRawOutputWrite)
	ri.guard(b, "file-write*", fmt.Sprintf("(subpath %q)", ri.inputsDir()), RuleInputsWrite, "I4")
	for _, f := range ri.PinnedFiles {
		if f != "" {
			ri.guard(b, "file-write*", fmt.Sprintf("(literal %q)", f), RulePipelineFileWrite, "I5")
		}
	}
}

func writeGrantReports(b *strings.Builder, paths []string, message string) {
	for _, p := range paths {
		if p != "" {
			fmt.Fprintf(b, "(allow file-read* file-read-data (literal %q) (with report) (with message %q))\n", p, message)
		}
	}
}

func (ri ReadIsolation) writeRawGuards(b *strings.Builder, op, rule string) {
	for _, d := range ri.RawDirs {
		ri.guard(b, op, fmt.Sprintf("(subpath %q)", d), rule, "I4")
	}
	if ri.RunsDir != "" && len(ri.RawSuffixes) > 0 {
		ri.guard(b, op, fmt.Sprintf(`(require-all (subpath %q) (regex #"%s"))`, ri.RunsDir, rawSuffixPattern(ri.RawSuffixes)), rule, "I4")
	}
}

func rawSuffixPattern(suffixes []string) string {
	quoted := make([]string, len(suffixes))
	for i, s := range suffixes {
		quoted[i] = regexp.QuoteMeta(s)
	}
	return "(" + strings.Join(quoted, "|") + ")$"
}

func bwrapReadIsolation(ri ReadIsolation) []string {
	if ri.Workspace == "" {
		return nil
	}
	if !ri.Enforce {
		return bwrapSecretMasks(ri)
	}
	var out []string
	if ri.RunsDir != "" {
		out = append(out, "--tmpfs", ri.RunsDir, "--bind", ri.Workspace, ri.Workspace)
	}
	for _, g := range ri.Grants {
		if g != "" {
			out = append(out, "--ro-bind-try", g, g)
		}
	}
	for _, f := range ri.RawFiles {
		out = append(out, "--ro-bind", os.DevNull, f)
	}
	for _, d := range ri.RawDirs {
		out = append(out, "--tmpfs", d)
	}
	out = append(out, bwrapSecretMasks(ri)...)
	for _, f := range append([]string{ri.inputsDir()}, ri.PinnedFiles...) {
		if f != "" {
			out = append(out, "--ro-bind-try", f, f)
		}
	}
	return out
}

func bwrapSecretMasks(ri ReadIsolation) []string {
	if !ri.secretsDenied() {
		return nil
	}
	var out []string
	for _, f := range ri.SecretFiles {
		out = append(out, "--ro-bind", os.DevNull, f)
	}
	for _, d := range ri.SecretDirs {
		out = append(out, "--tmpfs", d)
	}
	return out
}
