package phaseproto

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func benchRequest() core.PhaseRequest {
	return core.PhaseRequest{
		Cycle:       12345,
		ProjectRoot: "/path/to/project",
		Workspace:   "/path/to/workspace/cycle-12345",
		Worktree:    "/path/to/worktree/cycle-12345",
		GoalHash:    "deadbeefcafebabe",
		Context: map[string]string{
			"intent":  strings.Repeat("intent body ", 64),
			"history": strings.Repeat("history line ", 32),
		},
	}
}

func BenchmarkEncodeRequest(b *testing.B) {
	req := benchRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeRequest("corr-bench", req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeRequest(b *testing.B) {
	env, err := EncodeRequest("corr-bench", benchRequest())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DecodeRequest(env)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRoundTripEncodeDecodeRequest(b *testing.B) {
	req := benchRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env, err := EncodeRequest("corr-bench", req)
		if err != nil {
			b.Fatal(err)
		}
		_, err = DecodeRequest(env)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeResponse(b *testing.B) {
	resp := core.PhaseResponse{
		Phase:        "audit",
		Verdict:      core.VerdictPASS,
		ArtifactsDir: "/path/to/artifacts",
		NextPhase:    "ship",
		CostUSD:      0.42,
		DurationMS:   12345,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EncodeResponse("corr-bench", resp)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidate(b *testing.B) {
	env, err := EncodeRequest("corr-bench", benchRequest())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Validate(env); err != nil {
			b.Fatal(err)
		}
	}
}
