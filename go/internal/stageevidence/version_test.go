package stageevidence_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	fixturePolicy  = `{"regression_tia":{"stage":"shadow","window":100},"other":{"knob":1}}`
	deciderClosure = "decider"
)

type fixtureModule struct {
	dir    string
	policy string
}

func newFixtureModule(t *testing.T) *fixtureModule {
	t.Helper()
	m := &fixtureModule{dir: t.TempDir(), policy: fixturePolicy}
	m.write(t, "go.mod", "module fixture\n\ngo 1.23\n")
	m.write(t, "decider/decider.go", "package decider\n\nimport \"fixture/dep\"\n\nfunc Decide() int { return dep.Value }\n")
	m.write(t, "dep/dep.go", "package dep\n\nconst Value = 1\n")
	m.write(t, "unrelated/unrelated.go", "package unrelated\n\nconst Other = 1\n")
	return m
}

func (m *fixtureModule) write(t *testing.T, rel, content string) {
	t.Helper()
	path := filepath.Join(m.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (m *fixtureModule) version(t *testing.T) string {
	t.Helper()
	v, err := stageevidence.DecisionVersion(m.dir, []string{deciderClosure}, []byte(m.policy), "regression_tia")
	if err != nil {
		t.Fatalf("DecisionVersion: %v", err)
	}
	if v == "" {
		t.Fatal("DecisionVersion returned an empty digest")
	}
	return v
}

func TestVersionDigestStableUnderUnrelatedChange(t *testing.T) {
	t.Parallel()
	m := newFixtureModule(t)
	before := m.version(t)

	m.write(t, "unrelated/unrelated.go", "package unrelated\n\nconst Other = 2\n")
	m.write(t, "unrelated/extra.go", "package unrelated\n\nconst More = 3\n")
	m.policy = `{"regression_tia":{"stage":"shadow","window":100},"other":{"knob":99,"added":true}}`
	after := m.version(t)

	if before != after {
		t.Errorf("an edit outside the closure and the subtree changed the digest: %s -> %s", before, after)
	}
	chain := stageevidence.Chain{Records: linked([]stageevidence.Record{
		decision(1, 1, before, map[string]string{"would_stop": "false"}),
		decision(2, 2, before, map[string]string{"would_stop": "false"}),
	})}
	counted := stageevidence.Score(chain, after, strictPromotion, stageevidence.DefaultScorer())
	if counted.Decisions != 2 {
		t.Errorf("evidence recorded before the unrelated edit counts %d decisions, want 2", counted.Decisions)
	}
}

func TestVersionDigestChangesWithDeciderClosure(t *testing.T) {
	t.Parallel()
	base := newFixtureModule(t).version(t)
	edits := []struct {
		name  string
		apply func(t *testing.T, m *fixtureModule)
	}{
		{"decider source", func(t *testing.T, m *fixtureModule) {
			m.write(t, "decider/decider.go", "package decider\n\nimport \"fixture/dep\"\n\nfunc Decide() int { return dep.Value + 1 }\n")
		}},
		{"a module-local import of the decider", func(t *testing.T, m *fixtureModule) {
			m.write(t, "dep/dep.go", "package dep\n\nconst Value = 2\n")
		}},
		{"the policy subtree", func(t *testing.T, m *fixtureModule) {
			m.policy = `{"regression_tia":{"stage":"shadow","window":200},"other":{"knob":1}}`
		}},
		{"the toolchain line of go.mod", func(t *testing.T, m *fixtureModule) {
			m.write(t, "go.mod", "module fixture\n\ngo 1.22\n")
		}},
	}
	for _, e := range edits {
		m := newFixtureModule(t)
		e.apply(t, m)
		if got := m.version(t); got == base {
			t.Errorf("an edit of %s left the digest unchanged", e.name)
		}
	}
}

func TestVersionDigestOfAnAbsentSubtreeIsDefinedAndDiffersFromAPresentOne(t *testing.T) {
	t.Parallel()
	m := newFixtureModule(t)
	present := m.version(t)
	m.policy = `{}`
	absent := m.version(t)
	if absent == present {
		t.Error("an absent subtree gave the digest of a present one")
	}
	if _, err := stageevidence.DecisionVersion(m.dir, []string{deciderClosure}, []byte(`{not json`), "regression_tia"); err == nil {
		t.Error("a malformed policy gave a digest instead of an error")
	}
	if _, err := stageevidence.DecisionVersion(m.dir, []string{"no/such/package"}, []byte(`{}`), "regression_tia"); err == nil {
		t.Error("a closure package that does not exist gave a digest instead of an error")
	}
}
