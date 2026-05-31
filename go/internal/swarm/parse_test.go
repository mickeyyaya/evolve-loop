package swarm

import (
	"strings"
	"testing"
)

const sampleArtifact = "# Swarm Plan — Cycle 7\n\n" +
	"Some prose explaining the partition.\n\n" +
	"```json\n" +
	`{"swarm_plan":{"task_id":"t1","mode":"writer","partitionable":true,` +
	`"integration_branch":"cycle-7-integration","workers":[` +
	`{"worker_id":"w0","cli":"claude","model":"sonnet","branch":"cycle-7-w0",` +
	`"target_files":["a.go"],"depends_on":[],"scope":"A","acceptance":["go test ./a"]},` +
	`{"worker_id":"w1","cli":"codex","model":"gpt-5.5","branch":"cycle-7-w1",` +
	`"target_files":["b.go"],"depends_on":["w0"],"scope":"B"}]}}` + "\n" +
	"```\n\n## Reflection\nnone\n"

func TestParsePlan_FencedBlock(t *testing.T) {
	plan, err := ParsePlan(sampleArtifact)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.Mode != ModeWriter || plan.TaskID != "t1" || !plan.Partitionable {
		t.Errorf("header fields wrong: %+v", plan)
	}
	if len(plan.Workers) != 2 {
		t.Fatalf("want 2 workers, got %d", len(plan.Workers))
	}
	if plan.Workers[1].CLI != "codex" || plan.Workers[1].DependsOn[0] != "w0" {
		t.Errorf("worker w1 wrong: %+v", plan.Workers[1])
	}
	// End-to-end: a valid disjoint writer plan must validate OK.
	if got := Validate(plan); !got.OK {
		t.Errorf("parsed plan should validate OK: %+v", got)
	}
}

func TestParsePlan_BareJSON(t *testing.T) {
	bare := `{"swarm_plan":{"mode":"reader","partitionable":true,` +
		`"workers":[{"worker_id":"w0"},{"worker_id":"w1"}]}}`
	plan, err := ParsePlan(bare)
	if err != nil {
		t.Fatalf("parse bare: %v", err)
	}
	if plan.Mode != ModeReader || len(plan.Workers) != 2 {
		t.Errorf("bare parse wrong: %+v", plan)
	}
}

func TestParsePlan_Errors(t *testing.T) {
	cases := []struct {
		name, in, wantErr string
	}{
		{"no json block", "# just prose, no fence", "no ```json"},
		{"unterminated", "```json\n{\"swarm_plan\":{}", "unterminated"},
		{"bad json", "```json\n{not json}\n```", "swarm-plan JSON"},
		{"missing mode", "```json\n{\"swarm_plan\":{\"partitionable\":true}}\n```", "missing required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParsePlan(tc.in); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want error %q, got %v", tc.wantErr, err)
			}
		})
	}
}
