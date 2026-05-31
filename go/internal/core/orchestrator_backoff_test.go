package core

import (
	"testing"
	"time"
)

func TestResolveRetryBackoffBase(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want int
	}{
		{
			name: "empty env",
			env:  nil,
			want: 5,
		},
		{
			name: "unset val",
			env:  map[string]string{},
			want: 5,
		},
		{
			name: "valid val",
			env:  map[string]string{"EVOLVE_RETRY_BACKOFF_BASE_S": "10"},
			want: 10,
		},
		{
			name: "invalid val",
			env:  map[string]string{"EVOLVE_RETRY_BACKOFF_BASE_S": "abc"},
			want: 5,
		},
		{
			name: "negative val",
			env:  map[string]string{"EVOLVE_RETRY_BACKOFF_BASE_S": "-3"},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveRetryBackoffBase(tt.env); got != tt.want {
				t.Errorf("resolveRetryBackoffBase() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExecuteRetryBackoff_ZeroBaseDisables(t *testing.T) {
	env := map[string]string{"EVOLVE_RETRY_BACKOFF_BASE_S": "0"}
	start := time.Now()
	executeRetryBackoff(1, env) // would sleep if enabled
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("expected no sleep for zero base, slept for %v", duration)
	}
}

func TestExecuteRetryBackoff_AppliedOnAttempt2(t *testing.T) {
	// attempt = 1 in loop means we finished attempt 1 and are about to start attempt 2.
	// Since we sleep inside the loop before continuing to next attempt,
	// when attempt = 1, nextAttempt = 2.
	// We want to test that nextAttempt >= 2 triggers backoff.
	// Let's use a base of 1 second to test a quick 1-second sleep.
	env := map[string]string{"EVOLVE_RETRY_BACKOFF_BASE_S": "1"}

	// 1. nextAttempt = 1 (attempt = 0) -> no sleep (it should skip since nextAttempt < 2)
	start := time.Now()
	executeRetryBackoff(0, env)
	duration := time.Since(start)
	if duration > 100*time.Millisecond {
		t.Errorf("expected no sleep for nextAttempt < 2, slept for %v", duration)
	}

	// 2. nextAttempt = 2 (attempt = 1) -> should sleep for base * 2^(2-2) = 1 * 1 = 1 second
	start = time.Now()
	executeRetryBackoff(1, env)
	duration = time.Since(start)
	if duration < 900*time.Millisecond || duration > 1500*time.Millisecond {
		t.Errorf("expected sleep around 1s for attempt 2, slept for %v", duration)
	}
}
