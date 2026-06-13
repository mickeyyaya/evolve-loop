package aggregator

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAmplifiedAggregate_ReadFileSeamCoversEveryWorkerInOrder(t *testing.T) {
	dir := t.TempDir()
	workers := []string{
		fileWithContent(t, dir, "w1.md", "alpha"),
		fileWithContent(t, dir, "w2.md", "bravo"),
		fileWithContent(t, dir, "w3.md", "charlie"),
	}
	output := filepath.Join(dir, "scout-report.md")

	var calls []string
	var stderr bytes.Buffer
	rc := Aggregate(Inputs{
		Phase:   "scout",
		Output:  output,
		Workers: workers,
		ReadFile: func(path string) ([]byte, error) {
			calls = append(calls, path)
			return os.ReadFile(path)
		},
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("Aggregate() exit = %d, want %d; stderr=%s", rc, ExitOK, stderr.String())
	}
	wantCalls := append(append([]string{}, workers...), workers...)
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("ReadFile calls = %#v, want preflight+merge calls %#v", calls, wantCalls)
	}
	body := readFile(t, output)
	for _, want := range []string{"alpha", "bravo", "charlie"} {
		if !strings.Contains(body, want) {
			t.Fatalf("merged output missing %q:\n%s", want, body)
		}
	}
}

func TestAmplifiedAggregate_ReadFileFailureDoesNotLeavePartialOutput(t *testing.T) {
	dir := t.TempDir()
	workers := []string{
		fileWithContent(t, dir, "w1.md", "safe worker"),
		fileWithContent(t, dir, "w2.md", "unreadable through seam"),
	}
	output := filepath.Join(dir, "scout-report.md")
	const original = "pre-existing output must survive"
	if err := os.WriteFile(output, []byte(original), 0o644); err != nil {
		t.Fatalf("write original output: %v", err)
	}

	var stderr bytes.Buffer
	rc := Aggregate(Inputs{
		Phase:   "scout",
		Output:  output,
		Workers: workers,
		ReadFile: func(path string) ([]byte, error) {
			if path == workers[1] {
				return nil, errors.New("synthetic read failure")
			}
			return os.ReadFile(path)
		},
	}, &stderr)

	if rc != ExitUsageErr {
		t.Fatalf("Aggregate() exit = %d, want %d; stderr=%s", rc, ExitUsageErr, stderr.String())
	}
	if got := readFile(t, output); got != original {
		t.Fatalf("output was modified after failed preflight: got %q want %q", got, original)
	}
	if !strings.Contains(stderr.String(), "synthetic read failure") {
		t.Fatalf("stderr did not include read failure detail: %q", stderr.String())
	}
}

func TestAmplifiedAggregate_ReadFileSeamCanProvideVirtualContent(t *testing.T) {
	dir := t.TempDir()
	workers := []string{
		fileWithContent(t, dir, "w1.md", "disk content one"),
		fileWithContent(t, dir, "w2.md", "disk content two"),
	}
	output := filepath.Join(dir, "scout-report.md")
	virtual := map[string]string{
		workers[0]: "virtual alpha",
		workers[1]: "virtual bravo",
	}

	var stderr bytes.Buffer
	rc := Aggregate(Inputs{
		Phase:   "scout",
		Output:  output,
		Workers: workers,
		ReadFile: func(path string) ([]byte, error) {
			return []byte(virtual[path]), nil
		},
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("Aggregate() exit = %d, want %d; stderr=%s", rc, ExitOK, stderr.String())
	}
	body := readFile(t, output)
	if strings.Contains(body, "disk content") {
		t.Fatalf("Aggregate read from disk instead of the seam:\n%s", body)
	}
	for _, want := range []string{"virtual alpha", "virtual bravo"} {
		if !strings.Contains(body, want) {
			t.Fatalf("merged output missing virtual content %q:\n%s", want, body)
		}
	}
}

func fileWithContent(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write worker %s: %v", name, err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
