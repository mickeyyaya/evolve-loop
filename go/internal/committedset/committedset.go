// Package committedset owns the ONE answer to "which tasks did this cycle
// commit to?" and the on-disk shapes it is read from.
// See docs/architecture/packages/internal-committedset.md.
package committedset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const LanePinFile = "lane-scope.json"

const DecisionFile = "triage-decision.json"

func Committed(workspace string) ([]string, bool) {
	pin := LanePin(workspace)
	topN, decided := DecisionTopN(workspace)
	if len(pin) == 0 && !decided {
		return nil, false
	}
	ids := pin
	if len(ids) == 0 {
		ids = topN
	}
	deferred := Deferred(workspace)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !contains(deferred, id) {
			out = append(out, id)
		}
	}
	return out, true
}

type LanePinDoc struct {
	TodoIDs  []string `json:"todo_ids"`
	GoalHash string   `json:"goal_hash"`
}

func LanePin(workspace string) []string {
	var pin LanePinDoc
	if !readJSON(workspace, LanePinFile, &pin) {
		return nil
	}
	return nonBlank(pin.TodoIDs)
}

func DecisionTopN(workspace string) (ids []string, decided bool) {
	var decision struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if !readJSON(workspace, DecisionFile, &decision) {
		return nil, false
	}
	out := make([]string, 0, len(decision.TopN))
	for _, t := range decision.TopN {
		if id := strings.TrimSpace(t.ID); id != "" {
			out = append(out, id)
		}
	}
	return out, true
}

func Deferred(workspace string) []string {
	var decision struct {
		Deferred []struct {
			ID string `json:"id"`
		} `json:"deferred"`
	}
	if !readJSON(workspace, DecisionFile, &decision) {
		return nil
	}
	out := make([]string, 0, len(decision.Deferred))
	for _, t := range decision.Deferred {
		if id := strings.TrimSpace(t.ID); id != "" {
			out = append(out, id)
		}
	}
	return out
}

type Disposition struct {
	ID     string
	Bucket string
	Reason string
}

func Dispositions(workspace string) []Disposition {
	body, err := os.ReadFile(filepath.Join(workspace, DecisionFile))
	if err != nil {
		return nil
	}
	return DispositionsFrom(body)
}

func Unanswered(workspace string) []string {
	answered := map[string]bool{}
	for _, d := range Dispositions(workspace) {
		answered[d.ID] = true
	}
	var owed []string
	for _, id := range LanePin(workspace) {
		if !answered[id] {
			owed = append(owed, id)
		}
	}
	return owed
}

func DispositionsFrom(body []byte) []Disposition {
	var decision DecisionBuckets
	if json.Unmarshal(body, &decision) != nil {
		return nil
	}
	var out []Disposition
	seen := map[string]bool{}
	add := func(id, bucket, evidence string) {
		if id = strings.TrimSpace(id); id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, Disposition{ID: id, Bucket: bucket, Reason: strings.TrimSpace(evidence)})
	}
	for _, e := range decision.EscalateBlock {
		evidence := strings.TrimSpace(e.Reason)
		switch {
		case evidence != "":
		case e.FailCount > 0:
			evidence = "fail_count " + strconv.Itoa(e.FailCount)
		default:
			evidence = "escalated without a stated reason"
		}
		add(e.TaskID, "escalate_block", evidence)
	}
	for _, e := range decision.SkipRejected {
		add(e.TaskID, "skip_rejected", "triage reported an earlier rejection (inbox/rejected)")
	}
	for _, e := range decision.SkipShipped {
		if sha := strings.TrimSpace(e.GitSHA); sha != "" {
			add(e.TaskID, "skip_shipped", "git_sha "+sha)
		}
	}
	for _, e := range decision.Dropped {
		if strings.TrimSpace(e.Reason) != "" {
			add(e.ID, "dropped", e.Reason)
		}
	}
	return out
}

type Dropped struct {
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

type SkipShipped struct {
	TaskID string `json:"task_id"`
	GitSHA string `json:"git_sha,omitempty"`
}

type DroppedBucket struct {
	Dropped []Dropped `json:"dropped"`
}

type SkipShippedBucket struct {
	SkipShipped []SkipShipped `json:"skip_shipped"`
}

type DecisionBuckets struct {
	EscalateBlock []struct {
		TaskID    string `json:"task_id"`
		Reason    string `json:"reason"`
		FailCount int    `json:"fail_count"`
	} `json:"escalate_block"`
	SkipRejected []struct {
		TaskID string `json:"task_id"`
	} `json:"skip_rejected"`
	SkipShippedBucket
	DroppedBucket
}

func readJSON(workspace, name string, into any) bool {
	body, err := os.ReadFile(filepath.Join(workspace, name))
	if err != nil {
		return false
	}
	return json.Unmarshal(body, into) == nil
}

func nonBlank(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
