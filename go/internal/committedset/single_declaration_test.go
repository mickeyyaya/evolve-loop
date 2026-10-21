package committedset

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDecisionBuckets_TheOneDeclarationOfTheDroppedAndSkipShippedWireTags(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	decls := map[string][]string{}
	tags := []string{`json:"dropped"`, `json:"skip_shipped"`}
	err = filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "acs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		for _, tag := range tags {
			if strings.Contains(string(body), tag) {
				decls[tag] = append(decls[tag], filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range tags {
		slices.Sort(decls[tag])
		if want := []string{"internal/committedset/committedset.go"}; !reflect.DeepEqual(decls[tag], want) {
			t.Errorf("%s is declared in %v, want only %v: every reader and the writer of triage-decision.json share DecisionBuckets", tag, decls[tag], want)
		}
	}
}

func TestDecisionBuckets_ReadsEveryAnsweringBucketOfTheDocument(t *testing.T) {
	t.Parallel()
	doc := `{"escalate_block":[{"task_id":"e","reason":"r","fail_count":3}],"skip_rejected":[{"task_id":"x"}],"skip_shipped":[{"task_id":"s","git_sha":"abc"}],"dropped":[{"id":"d","reason":"duplicate"}]}`
	var got DecisionBuckets
	if err := json.Unmarshal([]byte(doc), &got); err != nil {
		t.Fatal(err)
	}
	if got.SkipShipped[0] != (SkipShipped{TaskID: "s", GitSHA: "abc"}) || got.Dropped[0] != (Dropped{ID: "d", Reason: "duplicate"}) ||
		got.EscalateBlock[0].FailCount != 3 || got.SkipRejected[0].TaskID != "x" {
		t.Fatalf("buckets = %+v", got)
	}
}

func TestDroppedBucket_AndSkipShippedBucket_WriteOnlyTheirOwnTag(t *testing.T) {
	t.Parallel()
	dropped, err := json.Marshal(DroppedBucket{Dropped: []Dropped{{ID: "d"}}})
	if err != nil || string(dropped) != `{"dropped":[{"id":"d"}]}` {
		t.Fatalf("DroppedBucket = %s (%v): the writer of triage-decision.json embeds it, so it must emit only dropped[], with an empty reason left out", dropped, err)
	}
	shipped, err := json.Marshal(SkipShippedBucket{SkipShipped: []SkipShipped{{TaskID: "s"}}})
	if err != nil || string(shipped) != `{"skip_shipped":[{"task_id":"s"}]}` {
		t.Fatalf("SkipShippedBucket = %s (%v)", shipped, err)
	}
}
