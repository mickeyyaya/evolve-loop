//go:build integration

package llmcalls

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImport_ConcurrentProcessesDeduplicateDestination(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "durable.ndjson")
	source := filepath.Join(dir, "scratch.ndjson")
	gate := filepath.Join(dir, "start")
	valid, err := json.Marshal(testRecord("shared-call"))
	if err != nil {
		t.Fatal(err)
	}
	// The malformed prefix lengthens the source scan so independently scheduled
	// helpers exercise destination-lock contention. Correctness relies on the
	// flock transaction, not on a particular process reaching the scan first.
	sourceBody := strings.Repeat("not-json\n", 40_000) + string(valid) + "\n"
	if err := os.WriteFile(source, []byte(sourceBody), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	const processes = 6
	commands := make([]*exec.Cmd, 0, processes)
	for i := 0; i < processes; i++ {
		ready := filepath.Join(dir, fmt.Sprintf("ready-%d", i))
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImportProcessHelper$")
		cmd.Env = append(os.Environ(),
			"LLMCALLS_IMPORT_HELPER=1",
			"LLMCALLS_IMPORT_DEST="+destination,
			"LLMCALLS_IMPORT_SOURCE="+source,
			"LLMCALLS_IMPORT_GATE="+gate,
			"LLMCALLS_IMPORT_READY="+ready,
		)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
		t.Cleanup(func() {
			if cmd.ProcessState == nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for ready := 0; ready < processes; {
		ready = 0
		for index := 0; index < processes; index++ {
			if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("ready-%d", index))); err == nil {
				ready++
			}
		}
		if ready == processes {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("import helpers did not become ready: %v", ctx.Err())
		case <-ticker.C:
		}
	}
	if err := os.WriteFile(gate, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("import helper: %v (context: %v)", err, ctx.Err())
		}
	}
	got, err := Read(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 || got.Records[0].CallID != "shared-call" {
		t.Fatalf("concurrent process imports produced %+v", got)
	}
	if _, err := os.Stat(destination + ".lock"); err != nil {
		t.Fatalf("canonical destination lock missing: %v", err)
	}
}

func TestImportProcessHelper(t *testing.T) {
	if os.Getenv("LLMCALLS_IMPORT_HELPER") != "1" {
		return
	}
	ready := os.Getenv("LLMCALLS_IMPORT_READY")
	if err := os.WriteFile(ready, []byte("ready"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(os.Getenv("LLMCALLS_IMPORT_GATE")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for import gate")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := Import(os.Getenv("LLMCALLS_IMPORT_DEST"), os.Getenv("LLMCALLS_IMPORT_SOURCE")); err != nil {
		t.Fatal(err)
	}
}
