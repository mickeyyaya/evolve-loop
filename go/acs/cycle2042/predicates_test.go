//go:build acs

package cycle2042

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var epoch = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func deliverable(id string, rank int, deps ...string) integration.Deliverable {
	return integration.Deliverable{ID: id, InboxRank: rank, Deps: deps, DeliveredAt: epoch}
}

func idsOf(batch []integration.Deliverable) []string {
	out := make([]string, 0, len(batch))
	for _, d := range batch {
		out = append(out, d.ID)
	}
	return out
}

func permutations(n int) [][]int {
	indexes := make([]int, n)
	for i := range indexes {
		indexes[i] = i
	}
	var all [][]int
	var heap func(k int)
	heap = func(k int) {
		if k <= 1 {
			all = append(all, slices.Clone(indexes))
			return
		}
		for i := 0; i < k; i++ {
			heap(k - 1)
			if k%2 == 0 {
				indexes[i], indexes[k-1] = indexes[k-1], indexes[i]
			} else {
				indexes[0], indexes[k-1] = indexes[k-1], indexes[0]
			}
		}
	}
	heap(n)
	return all
}

func arranged(batch []integration.Deliverable, permutation []int) []integration.Deliverable {
	out := make([]integration.Deliverable, 0, len(batch))
	for _, i := range permutation {
		out = append(out, batch[i])
	}
	return out
}

func TestC2042_001_AConsumerNeverComposesBeforeItsProducerInAnyArrivalOrder(t *testing.T) {
	scenarios := []struct {
		name  string
		batch []integration.Deliverable
	}{
		{"consumer outranks its producer", []integration.Deliverable{deliverable("consumer", 1, "producer"), deliverable("producer", 9)}},
		{"chain with ranks reversed", []integration.Deliverable{deliverable("c", 1, "b"), deliverable("b", 5, "a"), deliverable("a", 9)}},
		{"diamond with ranks reversed", []integration.Deliverable{deliverable("d", 1, "b", "c"), deliverable("b", 4, "a"), deliverable("c", 5, "a"), deliverable("a", 9)}},
		{"consumer beside an unrelated better rank", []integration.Deliverable{deliverable("consumer", 1, "producer"), deliverable("producer", 9), deliverable("other", 5)}},
		{"two consumers of one producer", []integration.Deliverable{deliverable("x", 1, "p"), deliverable("y", 2, "p"), deliverable("p", 8), deliverable("z", 3)}},
	}
	for _, s := range scenarios {
		for _, permutation := range permutations(len(s.batch)) {
			ordered := idsOf(integration.OrderDeliverables(arranged(s.batch, permutation)))
			if len(ordered) != len(s.batch) {
				t.Fatalf("%s: ordered %v, want all %d deliverables", s.name, ordered, len(s.batch))
			}
			for _, consumer := range s.batch {
				for _, producer := range consumer.Deps {
					if slices.Index(ordered, producer) > slices.Index(ordered, consumer.ID) {
						t.Errorf("%s: arrival %v ordered %v, consumer %s before its producer %s", s.name, idsOf(arranged(s.batch, permutation)), ordered, consumer.ID, producer)
					}
				}
			}
		}
	}
}

func TestC2042_002_TwoDeliverablesWithNoEdgeKeepTheirOrderByRank(t *testing.T) {
	late := deliverable("late", 1)
	late.DeliveredAt = epoch.Add(time.Hour)
	late.Risky = true
	late.VerificationCost = 90
	early := deliverable("early", 2)
	early.UnblocksCount = 7
	scenarios := []struct {
		name  string
		batch []integration.Deliverable
		want  []string
	}{
		{"three ranks arrive scrambled", []integration.Deliverable{deliverable("c", 3), deliverable("a", 1), deliverable("b", 2)}, []string{"a", "b", "c"}},
		{"rank beats every later key", []integration.Deliverable{early, late}, []string{"late", "early"}},
		{"a lower integer rank is the higher priority", []integration.Deliverable{deliverable("rank10", 10), deliverable("rank2", 2)}, []string{"rank2", "rank10"}},
	}
	for _, s := range scenarios {
		for _, permutation := range permutations(len(s.batch)) {
			got := idsOf(integration.OrderDeliverables(arranged(s.batch, permutation)))
			if !slices.Equal(got, s.want) {
				t.Errorf("%s: arrival %v ordered %v, want %v", s.name, idsOf(arranged(s.batch, permutation)), got, s.want)
			}
		}
	}
}

func TestC2042_003_EachLaterKeyBreaksTheTieOfTheKeysBeforeIt(t *testing.T) {
	worseOnEveryLaterKey := func(winner integration.Deliverable) integration.Deliverable {
		winner.ID = "z"
		return winner
	}
	moreUnblocks := worseOnEveryLaterKey(deliverable("", 3))
	moreUnblocks.UnblocksCount = 4
	moreUnblocks.Risky = true
	moreUnblocks.VerificationCost = 50
	moreUnblocks.DeliveredAt = epoch.Add(time.Hour)
	fewerUnblocks := deliverable("a", 3)

	safe := worseOnEveryLaterKey(deliverable("", 3))
	safe.VerificationCost = 50
	safe.DeliveredAt = epoch.Add(time.Hour)
	risky := deliverable("a", 3)
	risky.Risky = true

	cheap := worseOnEveryLaterKey(deliverable("", 3))
	cheap.DeliveredAt = epoch.Add(time.Hour)
	costly := deliverable("a", 3)
	costly.VerificationCost = 20

	delivered := worseOnEveryLaterKey(deliverable("", 3))
	delivered.DeliveredAt = epoch.Add(-time.Hour)
	deliveredLater := deliverable("a", 3)

	scenarios := []struct {
		name  string
		batch []integration.Deliverable
		want  []string
	}{
		{"more unblocked items first", []integration.Deliverable{fewerUnblocks, moreUnblocks}, []string{"z", "a"}},
		{"low risk before high risk", []integration.Deliverable{risky, safe}, []string{"z", "a"}},
		{"low verification cost before high cost", []integration.Deliverable{costly, cheap}, []string{"z", "a"}},
		{"earlier delivery first", []integration.Deliverable{deliveredLater, delivered}, []string{"z", "a"}},
		{"identifier last", []integration.Deliverable{deliverable("b", 3), deliverable("a", 3), deliverable("c", 3)}, []string{"a", "b", "c"}},
	}
	for _, s := range scenarios {
		for _, permutation := range permutations(len(s.batch)) {
			got := idsOf(integration.OrderDeliverables(arranged(s.batch, permutation)))
			if !slices.Equal(got, s.want) {
				t.Errorf("%s: arrival %v ordered %v, want %v", s.name, idsOf(arranged(s.batch, permutation)), got, s.want)
			}
		}
	}
}

func TestC2042_004_TheSameInputAlwaysGivesTheSameOrder(t *testing.T) {
	tied := func(id string, deps ...string) integration.Deliverable {
		return deliverable(id, 2, deps...)
	}
	risky := tied("risky")
	risky.Risky = true
	unblocker := tied("unblocker")
	unblocker.UnblocksCount = 3
	batch := []integration.Deliverable{tied("a"), tied("b", "a"), risky, unblocker, deliverable("first", 1), tied("tail", "first", "b")}

	reference := idsOf(integration.OrderDeliverables(batch))
	for _, permutation := range permutations(len(batch)) {
		got := idsOf(integration.OrderDeliverables(arranged(batch, permutation)))
		if !slices.Equal(got, reference) {
			t.Fatalf("arrival %v ordered %v, want the order %v of every other arrival", idsOf(arranged(batch, permutation)), got, reference)
		}
	}
	for repeat := 0; repeat < 25; repeat++ {
		if got := idsOf(integration.OrderDeliverables(batch)); !slices.Equal(got, reference) {
			t.Fatalf("repeat %d ordered %v, want %v", repeat, got, reference)
		}
	}
}

func TestC2042_005_EmptySingleUnknownDepAndCyclicBatchesAreOrderedCleanly(t *testing.T) {
	if got := integration.OrderDeliverables(nil); len(got) != 0 {
		t.Errorf("nil batch ordered %v, want nothing", idsOf(got))
	}
	if got := integration.OrderDeliverables([]integration.Deliverable{}); len(got) != 0 {
		t.Errorf("empty batch ordered %v, want nothing", idsOf(got))
	}
	if got := idsOf(integration.OrderDeliverables([]integration.Deliverable{deliverable("only", 1)})); !slices.Equal(got, []string{"only"}) {
		t.Errorf("single batch ordered %v, want [only]", got)
	}
	dangling := []integration.Deliverable{deliverable("waits", 1, "landed-in-an-earlier-batch"), deliverable("free", 2)}
	if got := idsOf(integration.OrderDeliverables(dangling)); !slices.Equal(got, []string{"waits", "free"}) {
		t.Errorf("a dep outside the batch ordered %v, want [waits free] by rank", got)
	}
	cyclic := []integration.Deliverable{deliverable("a", 1, "b"), deliverable("b", 2, "a"), deliverable("self", 3, "self"), deliverable("free", 4)}
	var reference []string
	for _, permutation := range permutations(len(cyclic)) {
		got := idsOf(integration.OrderDeliverables(arranged(cyclic, permutation)))
		sorted := slices.Clone(got)
		slices.Sort(sorted)
		if !slices.Equal(sorted, []string{"a", "b", "free", "self"}) {
			t.Fatalf("cyclic batch ordered %v, want every deliverable exactly once", got)
		}
		if reference == nil {
			reference = got
		}
		if !slices.Equal(got, reference) {
			t.Fatalf("cyclic batch arrival %v ordered %v, want the order %v of every other arrival", idsOf(arranged(cyclic, permutation)), got, reference)
		}
	}
}

func TestC2042_006_TheOrderLeavesItsInputUntouchedAndSharesNoStorageWithIt(t *testing.T) {
	batch := []integration.Deliverable{deliverable("c", 3, "a"), deliverable("a", 9), deliverable("b", 1)}
	before := make([]integration.Deliverable, len(batch))
	for i, d := range batch {
		d.Deps = slices.Clone(d.Deps)
		before[i] = d
	}
	ordered := integration.OrderDeliverables(batch)
	if !reflect.DeepEqual(batch, before) {
		t.Errorf("input became %v after ordering, want %v", batch, before)
	}
	ordered[0].ID = "overwritten"
	if !reflect.DeepEqual(batch, before) {
		t.Errorf("writing to the result changed the input to %v, want %v", batch, before)
	}
}

// acs-predicate: config-check
func TestC2042_007_TheIntegrationPackageIsEnrolledInTheStrictCoverAndApicoverGates(t *testing.T) {
	root := acsassert.RepoRoot(t)
	strict, err := os.ReadFile(filepath.Join(root, "go", ".cover-strict"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(strict), "\n"), "./internal/integration 100") {
		t.Errorf("RED: go/.cover-strict has no exact line \"./internal/integration 100\"")
	}
	enforce, err := os.ReadFile(filepath.Join(root, "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(enforce), "\n"), "./internal/integration") {
		t.Errorf("RED: go/.apicover-enforce has no exact line ./internal/integration")
	}
	for _, rel := range []string{"go/.cover-strict", "go/internal/integration/order.go"} {
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("RED: %s is not git-tracked", rel)
		}
	}
}

func goIn(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.WaitDelay = 10 * time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return out.String(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return out.String(), exit.ExitCode()
	}
	t.Fatalf("go %v: %v", args, err)
	return "", -1
}

func TestC2042_008_TheIntegrationPackageMeasuresOneHundredPercentWithOrderDeliverablesCovered(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "integration.cover")
	if out, code := goIn(t, goDir, "test", "-count=1", "-timeout", "240s", "-coverprofile="+profile, "./internal/integration"); code != 0 {
		t.Fatalf("go test ./internal/integration exit %d:\n%s", code, out)
	}
	funcs, code := goIn(t, goDir, "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover exit %d:\n%s", code, funcs)
	}
	orderLines := 0
	for _, line := range strings.Split(funcs, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		percent := fields[len(fields)-1]
		if percent != "100.0%" {
			t.Errorf("RED: %q is below 100%% statement coverage", strings.TrimSpace(line))
		}
		if strings.Contains(line, "order.go") && strings.Contains(line, "OrderDeliverables") {
			orderLines++
		}
	}
	if orderLines != 1 {
		t.Errorf("RED: the cover profile has %d OrderDeliverables lines in order.go, want 1", orderLines)
	}
	funcFile := filepath.Join(t.TempDir(), "integration.cover.func")
	if err := os.WriteFile(funcFile, []byte(funcs), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := goIn(t, goDir, "run", "./cmd/apicover", "-enforce", "-cover", funcFile, filepath.Join(goDir, "internal", "integration"))
	if code != 0 {
		t.Errorf("RED: apicover -enforce exit %d:\n%s", code, out)
	}
}
