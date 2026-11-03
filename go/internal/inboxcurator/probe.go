package inboxcurator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const ProposeQuarantine ProposalKind = "quarantine"

const probePolicyPath = ".evolve/policy.json"

var ErrAlreadyDone = errors.New("every probe of the item passes on the tree, so its work is done")

type RepoTree struct {
	FS      fs.FS
	HasTest func(string) bool
}

type ProbeResult struct {
	Probe  inboxbatch.Probe
	Pass   bool
	Detail string
}

func (r ProbeResult) String() string {
	s := string(r.Probe.Kind) + " " + r.Probe.Target
	if r.Probe.Value != "" {
		s += " " + strconv.Quote(r.Probe.Value)
	}
	if r.Pass {
		return s + ": pass"
	}
	return s + ": fail (" + r.Detail + ")"
}

type probeShape struct {
	target func(string) bool
	what   string
	value  bool
	check  func(RepoTree, inboxbatch.Probe) ProbeResult
}

var (
	testProbeRE   = regexp.MustCompile(`^(?:Test|Benchmark|Fuzz)[A-Za-z0-9_]*$`)
	symbolProbeRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\.[A-Za-z_][A-Za-z0-9_]*$`)
	policyKeyRE   = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*$`)
)

var probeShapes = map[inboxbatch.ProbeKind]probeShape{
	inboxbatch.ProbeTestExists:   {testProbeRE.MatchString, "a Test, Benchmark or Fuzz name", false, RepoTree.testExists},
	inboxbatch.ProbeSymbolExists: {symbolProbeRE.MatchString, "a pkg.Name", false, RepoTree.symbolExists},
	inboxbatch.ProbeTextAbsent:   {isRepoPath, "a repository path", true, RepoTree.textAbsent},
	inboxbatch.ProbePolicyValue:  {policyKeyRE.MatchString, "a dotted policy key", true, RepoTree.policyValue},
}

func isRepoPath(p string) bool { return p != "." && fs.ValidPath(p) }

func CheckProbes(probes []inboxbatch.Probe) error {
	for i, p := range probes {
		if err := checkProbe(p); err != nil {
			return fmt.Errorf("%w: probe %d: %v", ErrIntake, i+1, err)
		}
	}
	return nil
}

func checkProbe(p inboxbatch.Probe) error {
	shape, known := probeShapes[p.Kind]
	switch {
	case !known:
		return fmt.Errorf("kind %q is not one of %v", p.Kind, inboxbatch.ProbeKinds())
	case !shape.target(p.Target):
		return fmt.Errorf("%s target %q is not %s", p.Kind, p.Target, shape.what)
	case shape.value && p.Value == "":
		return fmt.Errorf("%s needs a value", p.Kind)
	case !shape.value && p.Value != "":
		return fmt.Errorf("%s takes no value", p.Kind)
	}
	return nil
}

func (t RepoTree) Check(p inboxbatch.Probe) ProbeResult {
	shape, known := probeShapes[p.Kind]
	if !known {
		return verdict(p, false, fmt.Sprintf("kind %q is not known", p.Kind))
	}
	return shape.check(t, p)
}

func ProbesPass(probes []inboxbatch.Probe, tree RepoTree) ([]ProbeResult, bool) {
	results := make([]ProbeResult, len(probes))
	all := len(probes) > 0
	for i, p := range probes {
		results[i] = tree.Check(p)
		all = all && results[i].Pass
	}
	return results, all
}

func (t RepoTree) Exists(p string) bool {
	name := path.Clean(p)
	if !fs.ValidPath(name) {
		return true
	}
	_, err := fs.Stat(t.FS, name)
	return !errors.Is(err, fs.ErrNotExist)
}

func (t RepoTree) gone(paths []string) []string {
	var out []string
	for _, p := range paths {
		if !t.Exists(p) {
			out = append(out, p)
		}
	}
	return out
}

func verdict(p inboxbatch.Probe, pass bool, why string) ProbeResult {
	if pass {
		return ProbeResult{Probe: p, Pass: true}
	}
	return ProbeResult{Probe: p, Detail: why}
}

func (t RepoTree) testExists(p inboxbatch.Probe) ProbeResult {
	if t.HasTest != nil {
		return verdict(p, t.HasTest(p.Target), "no test of that name")
	}
	names, err := indexTestsIn(t.FS, "go")
	if err != nil {
		return verdict(p, false, err.Error())
	}
	return verdict(p, names[p.Target], "no test of that name")
}

func (t RepoTree) symbolExists(p inboxbatch.Probe) ProbeResult {
	pkg, name, _ := strings.Cut(p.Target, ".")
	found := false
	err := walkGoFiles(t.FS, "go", isSourceFile, func(file string, body []byte) error {
		clause, err := parser.ParseFile(token.NewFileSet(), file, body, parser.PackageClauseOnly)
		if err != nil || clause.Name.Name != pkg {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, body, parser.SkipObjectResolution)
		found = found || (err == nil && declares(parsed, name))
		return err
	})
	if err != nil {
		return verdict(p, false, err.Error())
	}
	return verdict(p, found, "no top-level declaration of that name in the package")
}

func isSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func declares(f *ast.File, name string) bool {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name == name {
				return true
			}
		case *ast.GenDecl:
			if slices.ContainsFunc(d.Specs, func(s ast.Spec) bool { return specDeclares(s, name) }) {
				return true
			}
		}
	}
	return false
}

func specDeclares(s ast.Spec, name string) bool {
	switch s := s.(type) {
	case *ast.TypeSpec:
		return s.Name.Name == name
	case *ast.ValueSpec:
		return slices.ContainsFunc(s.Names, func(id *ast.Ident) bool { return id.Name == name })
	}
	return false
}

func (t RepoTree) textAbsent(p inboxbatch.Probe) ProbeResult {
	body, err := fs.ReadFile(t.FS, p.Target)
	if err != nil {
		return verdict(p, false, err.Error())
	}
	return verdict(p, !bytes.Contains(body, []byte(p.Value)), "the text is present")
}

func (t RepoTree) policyValue(p inboxbatch.Probe) ProbeResult {
	raw, err := fs.ReadFile(t.FS, probePolicyPath)
	if err != nil {
		return verdict(p, false, err.Error())
	}
	for _, seg := range strings.Split(p.Target, ".") {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return verdict(p, false, fmt.Sprintf("the value above %q is not an object: %v", seg, err))
		}
		next, set := obj[seg]
		if !set {
			return verdict(p, false, fmt.Sprintf("%q is not set", seg))
		}
		raw = next
	}
	got := leafText(raw)
	return verdict(p, got == p.Value, "the value is "+got)
}

func leafText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var buf bytes.Buffer
	_ = json.Compact(&buf, raw)
	return buf.String()
}

func judgeProbes(it inboxbatch.Item, tree RepoTree) error {
	if len(it.Probes) == 0 {
		return nil
	}
	declared := it.DeclaredPaths()
	if gone := tree.gone(declared); len(gone) == len(declared) {
		return fmt.Errorf("%w: no declared path is on the tree (%s); a probe item names at least one path that exists, so the sweep can tell a dead premise", ErrIntake, strings.Join(gone, ", "))
	}
	if results, all := ProbesPass(it.Probes, tree); all {
		return fmt.Errorf("%w: %w: %s", ErrIntake, ErrAlreadyDone, joinResults(results))
	}
	return nil
}

func joinResults(results []ProbeResult) string {
	lines := make([]string, len(results))
	for i, r := range results {
		lines[i] = r.String()
	}
	return strings.Join(lines, "; ")
}

func SweepProbes(open []inboxbatch.Item, tree RepoTree) []Proposal {
	var out []Proposal
	for _, it := range open {
		if p, ok := sweepProbeItem(it, tree); ok {
			out = append(out, p)
		}
	}
	return out
}

func sweepProbeItem(it inboxbatch.Item, tree RepoTree) (Proposal, bool) {
	if len(it.Probes) == 0 {
		return Proposal{}, false
	}
	p := Proposal{ID: it.ID, Path: it.Path, Trigger: TriggerSweep}
	declared := it.DeclaredPaths()
	if gone := tree.gone(declared); len(declared) > 0 && len(gone) == len(declared) {
		p.Kind, p.Evidence = ProposeQuarantine, []string{"paths gone: " + strings.Join(gone, ", ")}
		p.Reason = "every declared path is gone from the tree, so the premise of the item is dead; the operator judges the move"
		return p, true
	}
	results, all := ProbesPass(it.Probes, tree)
	if !all {
		return Proposal{}, false
	}
	p.Kind, p.Deterministic, p.Reason = ProposeConsume, true, "every probe of the item passes on the tree"
	for _, r := range results {
		p.Evidence = append(p.Evidence, "probe "+r.String())
	}
	return p, true
}
