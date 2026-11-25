//go:build acs

package cycle2632

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evolveCmdPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	goldSetRel   = "go/internal/research/testdata/gold_set.json"
	corpusRel    = "go/internal/research/testdata/gold_corpus"
	baselineRel  = "go/internal/research/testdata/gold_baseline.json"
	researchRel  = "go/internal/research"
	tolerance    = 1e-4
)

var errRankerDown = errors.New("ranker down")

type scriptedKB struct {
	byTerm map[string][]string
	err    error
}

func (s scriptedKB) Lookup(_ context.Context, q research.Query) ([]research.Lesson, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []research.Lesson
	for _, id := range s.byTerm[q.FailureMode] {
		out = append(out, research.Lesson{ID: id})
	}
	return out, nil
}

func goldQuery(id, mode string, relevant ...string) research.GoldQuery {
	return research.GoldQuery{ID: id, Plan: "cycle-1000", Query: research.Query{FailureMode: mode}, Relevant: relevant}
}

func near(a, b float64) bool { return math.Abs(a-b) <= tolerance }

func TestC2632_001_ScoreRecallComputesRecallAtKAndMRR(t *testing.T) {
	kb := scriptedKB{byTerm: map[string][]string{
		"first":   {"a", "x"},
		"second":  {"x", "b"},
		"partial": {"x", "c"},
		"miss":    {"x", "y"},
	}}
	set := research.GoldSet{Queries: []research.GoldQuery{
		goldQuery("q1", "first", "a"),
		goldQuery("q2", "second", "b"),
		goldQuery("q3", "partial", "c", "d"),
		goldQuery("q4", "miss", "e"),
	}}
	got, err := research.ScoreRecall(context.Background(), kb, set, 5)
	if err != nil {
		t.Fatalf("ScoreRecall: %v", err)
	}
	if got.Queries != 4 || got.K != 5 {
		t.Errorf("Queries=%d K=%d, want 4 and 5", got.Queries, got.K)
	}
	if !near(got.Recall, 0.625) {
		t.Errorf("recall@5 = %v, want 0.625 (mean of 1, 1, 0.5, 0)", got.Recall)
	}
	if !near(got.MRR, 0.5) {
		t.Errorf("MRR = %v, want 0.5 (mean of 1, 0.5, 0.5, 0)", got.MRR)
	}
}

func TestC2632_002_ScoreRecallCutsAtKAndCountsADuplicateOnce(t *testing.T) {
	deep := []string{"x1", "x2", "x3", "x4", "x5", "hit", "x7"}
	kb := scriptedKB{byTerm: map[string][]string{
		"deep":      deep,
		"duplicate": {"x", "a", "a"},
	}}
	cutAtFive, err := research.ScoreRecall(context.Background(), kb, research.GoldSet{Queries: []research.GoldQuery{goldQuery("deep", "deep", "hit")}}, 5)
	if err != nil {
		t.Fatalf("ScoreRecall k=5: %v", err)
	}
	if cutAtFive.Recall != 0 || cutAtFive.MRR != 0 {
		t.Errorf("a hit at rank 6 scored recall=%v MRR=%v at k=5, want 0 and 0", cutAtFive.Recall, cutAtFive.MRR)
	}
	cutAtTen, err := research.ScoreRecall(context.Background(), kb, research.GoldSet{Queries: []research.GoldQuery{goldQuery("deep", "deep", "hit")}}, 10)
	if err != nil {
		t.Fatalf("ScoreRecall k=10: %v", err)
	}
	if cutAtTen.Recall != 1 || !near(cutAtTen.MRR, 1.0/6.0) {
		t.Errorf("a hit at rank 6 scored recall=%v MRR=%v at k=10, want 1 and 1/6", cutAtTen.Recall, cutAtTen.MRR)
	}
	dup, err := research.ScoreRecall(context.Background(), kb, research.GoldSet{Queries: []research.GoldQuery{goldQuery("dup", "duplicate", "a", "b")}}, 5)
	if err != nil {
		t.Fatalf("ScoreRecall duplicate: %v", err)
	}
	if !near(dup.Recall, 0.5) || !near(dup.MRR, 0.5) {
		t.Errorf("a repeated relevant id scored recall=%v MRR=%v, want 0.5 and 0.5", dup.Recall, dup.MRR)
	}
}

func TestC2632_003_ScoreRecallRefusesInputsThatCannotBeScored(t *testing.T) {
	healthy := scriptedKB{byTerm: map[string][]string{"m": {"a"}}}
	one := research.GoldSet{Queries: []research.GoldQuery{goldQuery("q1", "m", "a")}}
	cases := []struct {
		name string
		kb   research.KB
		set  research.GoldSet
		k    int
	}{
		{"empty gold set", healthy, research.GoldSet{}, 5},
		{"query with no relevant entry", healthy, research.GoldSet{Queries: []research.GoldQuery{goldQuery("q1", "m")}}, 5},
		{"zero k", healthy, one, 0},
		{"negative k", healthy, one, -1},
		{"ranker failure", scriptedKB{err: errRankerDown}, one, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := research.ScoreRecall(context.Background(), c.kb, c.set, c.k)
			if err == nil {
				t.Fatalf("ScoreRecall returned %+v with no error, want an error", got)
			}
		})
	}
	_, err := research.ScoreRecall(context.Background(), scriptedKB{err: errRankerDown}, one, 5)
	if !errors.Is(err, errRankerDown) {
		t.Errorf("a ranker failure must wrap the ranker error, got %v", err)
	}
}

func TestC2632_004_GoldSetRoundTripsThroughLoadGoldSet(t *testing.T) {
	want := research.GoldSet{Queries: []research.GoldQuery{{
		ID:   "rt-1",
		Plan: "cycle-1000",
		Query: research.Query{
			Source:      "build",
			FailureMode: "contract gate block",
			Consequence: "worktree predicate unsatisfied",
			Keywords:    []string{"escalation", "overlay"},
		},
		Relevant: []string{"lesson-a", "lesson-b"},
	}}}
	path := filepath.Join(t.TempDir(), "gold.json")
	writeJSON(t, path, want)
	got, err := research.LoadGoldSet(path)
	if err != nil {
		t.Fatalf("LoadGoldSet: %v", err)
	}
	if len(got.Queries) != 1 {
		t.Fatalf("loaded %d queries, want 1", len(got.Queries))
	}
	q := got.Queries[0]
	if q.ID != "rt-1" || q.Plan != "cycle-1000" || q.Query.Source != "build" || q.Query.FailureMode != "contract gate block" ||
		q.Query.Consequence != "worktree predicate unsatisfied" || strings.Join(q.Query.Keywords, ",") != "escalation,overlay" ||
		strings.Join(q.Relevant, ",") != "lesson-a,lesson-b" {
		t.Errorf("round trip changed the query: %+v", q)
	}
	if _, err := research.LoadGoldSet(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Errorf("LoadGoldSet of an absent file returned no error")
	}
	corrupt := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(corrupt, []byte(`{"queries": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := research.LoadGoldSet(corrupt); err == nil {
		t.Errorf("LoadGoldSet of a corrupt file returned no error")
	}
}

func TestC2632_005_CheckedInGoldSetHasThirtyLabelledQueriesFromPastPlans(t *testing.T) {
	root := acsassert.RepoRoot(t)
	set, err := research.LoadGoldSet(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set %s: %v", goldSetRel, err)
	}
	if len(set.Queries) < 30 {
		t.Fatalf("RED: gold set holds %d queries, want at least 30", len(set.Queries))
	}
	ids := map[string]bool{}
	plans := map[string]bool{}
	planPattern := regexp.MustCompile(`^cycle-[0-9]+$`)
	for _, q := range set.Queries {
		if q.ID == "" || ids[q.ID] {
			t.Errorf("query id %q is empty or repeated", q.ID)
		}
		ids[q.ID] = true
		if !planPattern.MatchString(q.Plan) {
			t.Errorf("query %s plan %q is not a cycle-<N> reference", q.ID, q.Plan)
			continue
		}
		plans[q.Plan] = true
		if !acsassert.FileExists(t, filepath.Join(root, "knowledge-base", "cycles", q.Plan+".md")) {
			t.Errorf("query %s names plan %s that has no dossier under knowledge-base/cycles", q.ID, q.Plan)
		}
		if strings.TrimSpace(q.Query.FailureMode+q.Query.Consequence+strings.Join(q.Query.Keywords, "")) == "" {
			t.Errorf("query %s has no failure mode, consequence or keyword to rank on", q.ID)
		}
		if len(q.Relevant) == 0 {
			t.Errorf("query %s has no relevant entry marked", q.ID)
		}
		seen := map[string]bool{}
		for _, r := range q.Relevant {
			if r == "" || seen[r] {
				t.Errorf("query %s lists relevant id %q empty or twice", q.ID, r)
			}
			seen[r] = true
		}
	}
	if len(plans) < 10 {
		t.Errorf("gold set draws on %d distinct plans, want at least 10", len(plans))
	}
}

func TestC2632_006_EveryRelevantIdResolvesInTheCheckedInCorpusAmongDistractors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	set, err := research.LoadGoldSet(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set %s: %v", goldSetRel, err)
	}
	corpus := corpusIDs(t, filepath.Join(root, corpusRel))
	relevant := map[string]bool{}
	for _, q := range set.Queries {
		for _, r := range q.Relevant {
			relevant[r] = true
			if !corpus[r] {
				t.Errorf("query %s marks %q relevant but the corpus snapshot has no such lesson", q.ID, r)
			}
		}
	}
	if len(corpus) < 50 || len(corpus) <= len(relevant) {
		t.Errorf("corpus snapshot holds %d lessons for %d relevant ids, want at least 50 and some distractors", len(corpus), len(relevant))
	}
}

func TestC2632_007_RecallScoreCommandPrintsRecallAtFiveAndMRRForFileKB(t *testing.T) {
	bin := buildEvolve(t)
	gold, corpus := fixtureGold(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "kb", "recall", "score", "--gold", gold, "--corpus", corpus, "--json")
	if err != nil || code != 0 {
		t.Fatalf("RED: evolve kb recall score exit %d (%v)\nstderr: %s", code, err, acsassert.OutputTail(stderr, 20))
	}
	var rep map[string]any
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("RED: --json output is not JSON: %v\n%s", err, acsassert.OutputTail(stdout, 20))
	}
	if rep["ranker"] != "FileKB.Lookup" {
		t.Errorf("ranker = %v, want FileKB.Lookup", rep["ranker"])
	}
	if !jsonNear(rep["queries"], 3) || !jsonNear(rep["k"], 5) {
		t.Errorf("queries=%v k=%v, want 3 and 5", rep["queries"], rep["k"])
	}
	if !jsonNear(rep["recall_at_5"], 2.0/3.0) {
		t.Errorf("recall_at_5 = %v, want 2/3 (two of three queries hit their lesson)", rep["recall_at_5"])
	}
	if !jsonNear(rep["mrr"], 2.0/3.0) {
		t.Errorf("mrr = %v, want 2/3 (two first-rank hits, one miss)", rep["mrr"])
	}

	text, _, code, err := acsassert.SubprocessOutput(bin, "kb", "recall", "score", "--gold", gold, "--corpus", corpus)
	if err != nil || code != 0 {
		t.Fatalf("RED: evolve kb recall score (text) exit %d (%v)", code, err)
	}
	for label, re := range map[string]*regexp.Regexp{"recall@5": regexp.MustCompile(`recall@5\D+([0-9]*\.?[0-9]+)`), "MRR": regexp.MustCompile(`MRR\D+([0-9]*\.?[0-9]+)`)} {
		m := re.FindStringSubmatch(text)
		if m == nil {
			t.Errorf("text output has no %s figure:\n%s", label, text)
			continue
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		if !near(v, 2.0/3.0) {
			t.Errorf("text %s = %v, want 2/3", label, v)
		}
	}
}

func TestC2632_008_RecallScoreCommandRefusesAnUnscorableGoldSet(t *testing.T) {
	bin := buildEvolve(t)
	_, corpus := fixtureGold(t)
	dir := t.TempDir()
	unlabelled := filepath.Join(dir, "unlabelled.json")
	writeJSON(t, unlabelled, research.GoldSet{Queries: []research.GoldQuery{goldQuery("q1", "contract gate block")}})
	for name, gold := range map[string]string{"absent gold file": filepath.Join(dir, "absent.json"), "query with no relevant entry": unlabelled} {
		stdout, _, code, _ := acsassert.SubprocessOutput(bin, "kb", "recall", "score", "--gold", gold, "--corpus", corpus)
		if code == 0 {
			t.Errorf("%s: exit 0, want a non-zero exit", name)
		}
		if strings.Contains(stdout, "recall@5") {
			t.Errorf("%s: printed a recall figure for a set that cannot be scored:\n%s", name, stdout)
		}
	}
}

func TestC2632_009_BaselineOfFileKBIsRecordedAndReproducedFromTheDefaultCommand(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var baseline struct {
		Ranker  string  `json:"ranker"`
		K       int     `json:"k"`
		Queries int     `json:"queries"`
		Recall  float64 `json:"recall_at_5"`
		MRR     float64 `json:"mrr"`
	}
	raw, err := os.ReadFile(filepath.Join(root, baselineRel))
	if err != nil {
		t.Fatalf("RED: baseline %s: %v", baselineRel, err)
	}
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatalf("RED: baseline is not JSON: %v", err)
	}
	set, err := research.LoadGoldSet(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set: %v", err)
	}
	if baseline.Ranker != "FileKB.Lookup" || baseline.K != 5 || baseline.Queries != len(set.Queries) {
		t.Errorf("baseline names ranker=%q k=%d queries=%d, want FileKB.Lookup, 5 and %d", baseline.Ranker, baseline.K, baseline.Queries, len(set.Queries))
	}
	if baseline.Recall <= 0 || baseline.Recall > 1 || baseline.MRR <= 0 || baseline.MRR > 1 {
		t.Errorf("baseline recall_at_5=%v mrr=%v, want both in (0, 1]: a zero figure shows labels that no query can reach", baseline.Recall, baseline.MRR)
	}

	bin := buildEvolve(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "kb", "recall", "score", "--project-root", root, "--json")
	if err != nil || code != 0 {
		t.Fatalf("RED: default evolve kb recall score exit %d (%v)\nstderr: %s", code, err, acsassert.OutputTail(stderr, 20))
	}
	var live map[string]any
	if err := json.Unmarshal([]byte(stdout), &live); err != nil {
		t.Fatalf("RED: default run is not JSON: %v", err)
	}
	if !jsonNear(live["recall_at_5"], baseline.Recall) || !jsonNear(live["mrr"], baseline.MRR) || !jsonNear(live["queries"], float64(baseline.Queries)) {
		t.Errorf("today's FileKB scores recall_at_5=%v mrr=%v queries=%v, but the recorded baseline is %v / %v / %d", live["recall_at_5"], live["mrr"], live["queries"], baseline.Recall, baseline.MRR, baseline.Queries)
	}
}

func TestC2632_010_GoldDataIsNotGitIgnored(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{goldSetRel, baselineRel, corpusRel} {
		if !acsassert.PathExists(filepath.Join(root, rel)) {
			t.Errorf("RED: %s is missing", rel)
			continue
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
			t.Errorf("%s is git-ignored (check-ignore exit %d): it would be dropped at ship", rel, code)
		}
	}
}

func TestC2632_011_EveryExportedResearchSymbolIsNamedInATest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, researchRel)
	syms, err := apicover.Enumerate(context.Background(), dir)
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	named, err := apicover.NamesReferencedInTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("NamesReferencedInTests: %v", err)
	}
	present := map[string]bool{}
	for _, s := range syms {
		present[s.Name] = true
		key := s.Name
		if s.Kind == apicover.KindMethod {
			key = key[strings.LastIndex(key, ".")+1:]
		}
		if !s.Ignored && !named[key] {
			t.Errorf("exported %s %s is named in no test of internal/research (apicover_named_test.go)", s.Kind, s.Name)
		}
	}
	for _, want := range []string{"GoldSet", "GoldQuery", "LoadGoldSet", "ScoreRecall", "RecallScore"} {
		if !present[want] {
			t.Errorf("RED: internal/research exports no %s", want)
		}
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	if _, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", bin, evolveCmdPkg); err != nil || code != 0 {
		t.Fatalf("RED: the evolve binary did not build: exit %d (%v)\n%s", code, err, acsassert.OutputTail(stderr, 30))
	}
	return bin
}

func fixtureGold(t *testing.T) (gold, corpus string) {
	t.Helper()
	dir := t.TempDir()
	corpus = filepath.Join(dir, "corpus")
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		t.Fatal(err)
	}
	lessons := map[string]string{
		"gate.yaml":     lessonYAML("fx-gate", "gatealpha", "gatealpha contract gate block in the build phase", "re-dispatch the build with the escalation overlay"),
		"stall.yaml":    lessonYAML("fx-stall", "stallbeta", "stallbeta bridge pane timeout during the tdd phase", "raise the pane idle limit"),
		"distract.yaml": lessonYAML("fx-other", "otherzeta", "otherzeta ledger seal drift", "reseal the ledger"),
	}
	for name, body := range lessons {
		if err := os.WriteFile(filepath.Join(corpus, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	set := research.GoldSet{Queries: []research.GoldQuery{
		{ID: "g1", Plan: "cycle-1000", Query: research.Query{FailureMode: "gatealpha contract gate"}, Relevant: []string{"fx-gate"}},
		{ID: "g2", Plan: "cycle-1001", Query: research.Query{Keywords: []string{"stallbeta", "pane", "timeout"}}, Relevant: []string{"fx-stall"}},
		{ID: "g3", Plan: "cycle-1002", Query: research.Query{FailureMode: "quokka"}, Relevant: []string{"fx-other"}},
	}}
	gold = filepath.Join(dir, "gold.json")
	writeJSON(t, gold, set)
	return gold, corpus
}

func lessonYAML(id, pattern, description, action string) string {
	return "- id: " + id + "\n  pattern: " + pattern + "\n  description: " + description + "\n  confidence: 0.8\n  source: fixture\n  type: failure-lesson\n  category: episodic\n  preventiveAction: " + action + "\n  failureContext:\n    failedStep: build\n    errorCategory: " + pattern + "\n    auditVerdict: FAIL\n"
}

func corpusIDs(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("RED: corpus snapshot %s: %v", corpusRel, err)
	}
	idLine := regexp.MustCompile(`(?m)^- id: (\S+)\s*$`)
	ids := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range idLine.FindAllStringSubmatch(string(raw), -1) {
			ids[m[1]] = true
		}
	}
	return ids
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func jsonNear(v any, want float64) bool {
	f, ok := v.(float64)
	return ok && near(f, want)
}

const (
	maxRunSharedWithTarget = 3
	minRunQuotedFromPlan   = 3
	minPlanCoverage        = 0.5
	minMultiLabelQueries   = 8
	minCrossPlanQueries    = 8
	minRationaleLength     = 40
	planBoundaryToken      = "|"
)

var wordPattern = regexp.MustCompile(`[a-z0-9]+`)

func wordsOf(s string) []string {
	return wordPattern.FindAllString(strings.ToLower(s), -1)
}

func longestSharedRun(a, b []string) int {
	best := 0
	prev := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] && a[i-1] != planBoundaryToken {
				cur[j] = prev[j-1] + 1
				best = max(best, cur[j])
			}
		}
		prev = cur
	}
	return best
}

func containsRun(haystack, needle []string) bool {
	return len(needle) > 0 && longestSharedRun(haystack, needle) == len(needle)
}

func copiesTarget(query research.Query, targetWords []string) bool {
	for _, seq := range [][]string{wordsOf(query.FailureMode), wordsOf(strings.Join(query.Keywords, " "))} {
		if longestSharedRun(seq, targetWords) > maxRunSharedWithTarget {
			return true
		}
		if len(seq) >= 3 && containsRun(targetWords, seq) {
			return true
		}
	}
	return false
}

func lessonPatterns(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("RED: corpus snapshot %s: %v", corpusRel, err)
	}
	row := regexp.MustCompile(`(?m)^- id: (\S+)\s*\n\s+pattern: "?([^"\n]*)"?\s*$`)
	patterns := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range row.FindAllStringSubmatch(string(raw), -1) {
			patterns[m[1]] = m[2]
		}
	}
	return patterns
}

func planContentWords(t *testing.T, root, plan string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "knowledge-base", "cycles", plan+".md"))
	if err != nil {
		t.Fatalf("plan dossier %s: %v", plan, err)
	}
	text := string(raw)
	var words []string
	if m := regexp.MustCompile(`(?m)^\*\*Goal:\*\*(.*)$`).FindStringSubmatch(text); m != nil {
		words = append(words, wordsOf(m[1])...)
	}
	words = append(words, planBoundaryToken)
	if m := regexp.MustCompile(`(?s)\n## Failure\n(.*?)(?:\n## |\z)`).FindStringSubmatch(text); m != nil {
		words = append(words, wordsOf(m[1])...)
	}
	return words
}

type labelledQuery struct {
	ID        string `json:"id"`
	Labeller  string `json:"labeller"`
	Rationale string `json:"rationale"`
}

func loadLabels(t *testing.T, root string) map[string]labelledQuery {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set %s: %v", goldSetRel, err)
	}
	var doc struct {
		Queries []labelledQuery `json:"queries"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("RED: gold set is not JSON: %v", err)
	}
	labels := map[string]labelledQuery{}
	for _, q := range doc.Queries {
		labels[q.ID] = q
	}
	return labels
}

func TestC2632_012_NoGoldQueryIsCopiedFromItsRelevantLessonsOwnPatternOrId(t *testing.T) {
	target := wordsOf("one-counter-two-policies-suppression-disarms-the-remedy")
	copied := research.Query{FailureMode: "one counter two policies suppression disarms", Keywords: []string{"one", "counter", "two", "policies", "suppression", "disarms"}}
	paraphrase := research.Query{FailureMode: "retry budget shared between suppression and remedy paths", Keywords: []string{"retry", "budget", "suppression", "remedy"}}
	if !copiesTarget(copied, target) {
		t.Fatalf("detector misses a query that is the first words of the pattern")
	}
	if copiesTarget(paraphrase, target) {
		t.Fatalf("detector flags a paraphrase that shares only single words with the pattern")
	}

	root := acsassert.RepoRoot(t)
	set, err := research.LoadGoldSet(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set %s: %v", goldSetRel, err)
	}
	patterns := lessonPatterns(t, filepath.Join(root, corpusRel))
	for _, q := range set.Queries {
		for _, id := range q.Relevant {
			for name, text := range map[string]string{"pattern": patterns[id], "id": id} {
				if copiesTarget(q.Query, wordsOf(text)) {
					t.Errorf("query %s (%q) is copied from the %s of its relevant lesson %s: the baseline would score self-retrieval", q.ID, q.Query.FailureMode, name, id)
				}
			}
		}
	}
}

func TestC2632_013_EveryGoldQueryQuotesItsPlanDossierText(t *testing.T) {
	root := acsassert.RepoRoot(t)
	set, err := research.LoadGoldSet(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set %s: %v", goldSetRel, err)
	}
	for _, q := range set.Queries {
		plan := planContentWords(t, root, q.Plan)
		query := wordsOf(q.Query.FailureMode)
		if run := longestSharedRun(query, plan); run < minRunQuotedFromPlan {
			t.Errorf("query %s (%q) shares a run of %d words with the goal and failure text of %s, want at least %d: it is not taken from the plan", q.ID, q.Query.FailureMode, run, q.Plan, minRunQuotedFromPlan)
		}
		inPlan := map[string]bool{}
		for _, w := range plan {
			inPlan[w] = true
		}
		var long, covered int
		for _, w := range append(query, wordsOf(strings.Join(q.Query.Keywords, " "))...) {
			if len(w) < 4 {
				continue
			}
			long++
			if inPlan[w] {
				covered++
			}
		}
		if long == 0 || float64(covered)/float64(long) < minPlanCoverage {
			t.Errorf("query %s: %d of %d content words occur in the %s goal and failure text, want at least %.0f%%", q.ID, covered, long, q.Plan, minPlanCoverage*100)
		}
	}
}

func TestC2632_014_RelevanceIsJudgedNotMappedFromTheCycleNumber(t *testing.T) {
	root := acsassert.RepoRoot(t)
	set, err := research.LoadGoldSet(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set %s: %v", goldSetRel, err)
	}
	var multi, cross int
	for _, q := range set.Queries {
		if len(q.Relevant) >= 2 {
			multi++
		}
		for _, id := range q.Relevant {
			if !strings.HasPrefix(id, q.Plan+"-") {
				cross++
				break
			}
		}
	}
	if multi < minMultiLabelQueries {
		t.Errorf("%d queries mark two or more lessons relevant, want at least %d: one label per query is the cycle-number mapping", multi, minMultiLabelQueries)
	}
	if cross < minCrossPlanQueries {
		t.Errorf("%d queries mark a lesson of another cycle relevant, want at least %d", cross, minCrossPlanQueries)
	}
}

func TestC2632_015_EveryGoldQueryRecordsItsLabellerAndRationale(t *testing.T) {
	root := acsassert.RepoRoot(t)
	set, err := research.LoadGoldSet(filepath.Join(root, goldSetRel))
	if err != nil {
		t.Fatalf("RED: checked-in gold set %s: %v", goldSetRel, err)
	}
	labels := loadLabels(t, root)
	for _, q := range set.Queries {
		l := labels[q.ID]
		if strings.TrimSpace(l.Labeller) == "" {
			t.Errorf("query %s records no labeller", q.ID)
		}
		if len(strings.TrimSpace(l.Rationale)) < minRationaleLength {
			t.Errorf("query %s records a rationale of %d characters, want at least %d", q.ID, len(strings.TrimSpace(l.Rationale)), minRationaleLength)
		}
		for _, id := range q.Relevant {
			if !strings.Contains(l.Rationale, id) {
				t.Errorf("query %s rationale does not justify relevant lesson %s", q.ID, id)
			}
		}
	}
}
