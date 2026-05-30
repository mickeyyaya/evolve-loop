// Package commitgate is the Go home for the commit-gate tier. The behavioral
// coverage of the gate RUNNER lives in commit-gate-test.sh (bash, exercises
// commit-gate/commit-gate-runner.sh over ephemeral repos); the attestation's
// enforcement at commit time is covered by go/internal/phases/ship/commitgate_test.go.
// This doc.go exists so the directory is a valid Go package. See go/docs/testing.md.
package commitgate
