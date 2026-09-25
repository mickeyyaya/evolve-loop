// Package committedset owns the ONE answer to "which tasks did this cycle
// commit to?" and the on-disk shapes it is read from.
//
// It exists because that question had four independent readers that disagreed
// on live cycles: core.ContractTaskIDs (lane pin, else decision top_n, minus
// deferred), core.BoundTaskIDs (decision top_n only), inboxmover.CommittedIDs
// (top_n ∪ skip_shipped) and, briefly, the cycle dossier (top_n only). On
// runtime cycle-1621 the lane pin named two members while top_n named one, so
// a top_n-only reader under-reported the commitment — and 17 of the last 20
// runtime cycles carried a lane pin, so that was the DOMINANT shape, not an
// edge case. A record that reports a commitment it did not read is worse than
// one that reports none.
//
// This package is a LEAF (stdlib only) so every consumer can project from it:
// internal/core owns the orchestrator and internal/dossier cannot import core.
// It parses; the only policy it owns is the documented precedence (Committed)
// and what counts as triage ANSWERING for an id it did not commit
// (DispositionsFrom — F30).
package committedset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LanePinFile is the lane-identity pin written into the run workspace before
// any phase output exists; its todo_ids are the lane's assigned members.
const LanePinFile = "lane-scope.json"

// DecisionFile is triage's structured decision: the committed top_n and the
// deferred set.
const DecisionFile = "triage-decision.json"

// Committed returns the task ids this cycle is bound to, in order, and whether
// any binding was recorded at all.
//
// Precedence — the lane pin first, then triage's decision, minus deferrals:
//
//	lane pin present & non-empty → those ids (the lane's assignment is
//	    authoritative even when a resumed triage re-labels its working ids)
//	otherwise                    → the decision's top_n
//	either way                   → minus every id the decision deferred
//
// ok is false only when NEITHER artifact yields a binding: nothing was
// recorded, which is "unknown". A present decision that committed to nothing
// returns (empty non-nil slice, true) — an explicit empty commitment is a
// finding in its own right, and callers must be able to tell it from unknown.
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

// LanePinDoc is the pinned lane identity's wire shape: the todo ids assigned
// to this lane and the goal hash it was provisioned for. It lives HERE, and
// core aliases it (type LaneScope = committedset.LanePinDoc), so exactly ONE
// non-test file declares `json:"todo_ids"` — the invariant
// cycleoutcome's TestLaneScopeProjection_SingleWireShapeDeclaration enforces,
// whose own header prescribes this leaf-package shape. An alias carries no
// struct tag, so core's composite literals keep compiling.
type LanePinDoc struct {
	TodoIDs  []string `json:"todo_ids"`
	GoalHash string   `json:"goal_hash"`
}

// LanePin returns the lane's pinned member ids. An absent, malformed or empty
// pin returns nil so the caller falls through to the decision — never a silent
// empty commitment.
func LanePin(workspace string) []string {
	var pin LanePinDoc
	if !readJSON(workspace, LanePinFile, &pin) {
		return nil
	}
	return nonBlank(pin.TodoIDs)
}

// DecisionTopN returns the ids triage committed. decided is false when the
// decision artifact is absent or unreadable; a present decision with an empty
// top_n returns (empty non-nil slice, true).
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

// Deferred returns the ids triage explicitly carried to a later cycle. Only an
// explicit deferral removes a task from its acceptance contract, so an absent
// or unreadable decision defers nothing.
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

// Disposition is one id triage ANSWERED for without committing to it: the
// bucket it placed the id in and that bucket's evidence (a drop's reason, a
// skip's sha, an escalation's reason).
type Disposition struct {
	ID     string // the answered task id
	Bucket string // escalate_block | skip_rejected | skip_shipped | dropped
	Reason string // the bucket's evidence (never empty for an answer)
}

// Dispositions reads the workspace's decision through DispositionsFrom. An
// absent or unreadable decision answers for nothing.
func Dispositions(workspace string) []Disposition {
	body, err := os.ReadFile(filepath.Join(workspace, DecisionFile))
	if err != nil {
		return nil
	}
	return DispositionsFrom(body)
}

// DispositionsFrom returns every id a triage decision answered for, one entry
// per id, the most severe bucket first: escalate_block → skip_rejected →
// skip_shipped → dropped (an integrity escalation is never masked by a drop's
// reason). An answer is a statement a reader can act on (F30, cycle 1682), so
// every answer carries evidence: an escalation its reason, else its
// fail_count, else an explicit "escalated without a stated reason" (an
// escalation is still a disposition); a rejection triage's report of the
// rejected-dir match; a skip its sha; a drop its reason — a skip without a sha
// or a drop without a reason answers for nothing. A DEFERRAL is never an answer: cycle 1623's triage narrated a claim
// failure as a deferral, so a deferred id is still owed work. A malformed
// decision answers for nothing.
func DispositionsFrom(body []byte) []Disposition {
	var decision struct {
		EscalateBlock []struct {
			TaskID    string `json:"task_id"`
			Reason    string `json:"reason"`
			FailCount int    `json:"fail_count"`
		} `json:"escalate_block"`
		SkipRejected []struct {
			TaskID string `json:"task_id"`
		} `json:"skip_rejected"`
		SkipShipped []struct {
			TaskID string `json:"task_id"`
			GitSHA string `json:"git_sha"`
		} `json:"skip_shipped"`
		Dropped []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"dropped"`
	}
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

// readJSON decodes one workspace artifact. Absent, unreadable or malformed all
// report false: this package never fabricates a binding.
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
