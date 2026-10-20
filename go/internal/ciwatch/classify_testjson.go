package ciwatch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	testJSONArtifactPrefix = "coverage-"
	testJSONFile           = "test-integration.json"
	testJSONLineCap        = 64 << 20
	artifactPageCap        = 100
)

var failEventMark = []byte(`"Action":"fail"`)

type testEvent struct {
	Action  string
	Package string
	Test    string
}

func readTestJSONFailures(r io.Reader) ([]FailingTest, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, testJSONLineCap)
	failedPkgs := map[string]bool{}
	var tests []FailingTest
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, failEventMark) {
			continue
		}
		var e testEvent
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("test json: %w", err)
		}
		switch {
		case e.Test == "":
			failedPkgs[e.Package] = true
		case !strings.Contains(e.Test, "/"):
			tests = append(tests, FailingTest{Package: e.Package, Test: e.Test})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("test json: %w", err)
	}
	return failuresOfFailedPackages(failedPkgs, tests), nil
}

func failuresOfFailedPackages(failedPkgs map[string]bool, tests []FailingTest) []FailingTest {
	out := []FailingTest{}
	named := map[string]bool{}
	for _, t := range tests {
		if failedPkgs[t.Package] {
			out = append(out, t)
			named[t.Package] = true
		}
	}
	for pkg := range failedPkgs {
		if !named[pkg] {
			out = append(out, FailingTest{Package: pkg})
		}
	}
	return out
}

func (s ghClassifySource) testJSONFailures(ctx context.Context, st RunStatus) ([]FailingTest, bool, error) {
	if st.Attempt > 1 {
		return nil, false, nil
	}
	id := strconv.FormatInt(st.RunID, 10)
	names, err := s.testJSONArtifacts(ctx, id)
	if err != nil {
		return nil, false, err
	}
	dir, err := os.MkdirTemp("", "ciwatch-testjson-")
	if err != nil {
		return nil, false, fmt.Errorf("test json temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	var all []FailingTest
	found := false
	for _, name := range names {
		dest := filepath.Join(dir, name)
		if _, err := s.gh(ctx, "run", "download", id, "--name", name, "--dir", dest); err != nil {
			return nil, false, err
		}
		rows, ok, err := readTestJSONFile(filepath.Join(dest, testJSONFile))
		if err != nil {
			return nil, false, fmt.Errorf("artifact %s: %w", name, err)
		}
		found = found || ok
		for _, row := range rows {
			row.Jobs = []string{name}
			all = append(all, row)
		}
	}
	return all, found, nil
}

func (s ghClassifySource) testJSONArtifacts(ctx context.Context, id string) ([]string, error) {
	var doc struct {
		Total     int `json:"total_count"`
		Artifacts []struct {
			Name    string `json:"name"`
			Expired bool   `json:"expired"`
		} `json:"artifacts"`
	}
	if err := s.ghJSON(ctx, &doc, "api", "repos/{owner}/{repo}/actions/runs/"+id+"/artifacts?per_page="+strconv.Itoa(artifactPageCap)); err != nil {
		return nil, err
	}
	if doc.Total > len(doc.Artifacts) {
		return nil, fmt.Errorf("ciwatch: run %s lists %d artifacts, over the %d read in one page", id, doc.Total, len(doc.Artifacts))
	}
	var names []string
	for _, a := range doc.Artifacts {
		if !a.Expired && strings.HasPrefix(a.Name, testJSONArtifactPrefix) {
			names = append(names, a.Name)
		}
	}
	return names, nil
}

func readTestJSONFile(path string) ([]FailingTest, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	rows, err := readTestJSONFailures(f)
	return rows, err == nil, err
}
