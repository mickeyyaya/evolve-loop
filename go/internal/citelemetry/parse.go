package citelemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	testJSONFile = "test-integration.json"
	rusageFile   = "test-rusage.ndjson"
	samplesFile  = "runner-samples.ndjson"
)

type rawStep struct {
	Name        string `json:"name"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
}

type rawJob struct {
	rawStep
	Steps []rawStep `json:"steps"`
}

type jobsDocument struct {
	Jobs []rawJob `json:"jobs"`
}

func elapsed(from, to string) (float64, error) {
	if from == "" || to == "" {
		return 0, nil
	}
	start, err := time.Parse(time.RFC3339, from)
	if err != nil {
		return 0, err
	}
	end, err := time.Parse(time.RFC3339, to)
	if err != nil {
		return 0, err
	}
	return end.Sub(start).Seconds(), nil
}

func parseJobs(data []byte) ([]job, error) {
	var doc jobsDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("jobs document: %w", err)
	}
	jobs := make([]job, 0, len(doc.Jobs))
	for _, raw := range doc.Jobs {
		seconds, err := elapsed(raw.StartedAt, raw.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("job %q: %w", raw.Name, err)
		}
		parsed := job{name: raw.Name, seconds: seconds}
		for _, rs := range raw.Steps {
			stepSeconds, err := elapsed(rs.StartedAt, rs.CompletedAt)
			if err != nil {
				return nil, fmt.Errorf("job %q step %q: %w", raw.Name, rs.Name, err)
			}
			parsed.steps = append(parsed.steps, jobStep{name: rs.Name, seconds: stepSeconds})
		}
		jobs = append(jobs, parsed)
	}
	return jobs, nil
}

func decodeLines[T any](data []byte) ([]T, error) {
	var rows []T
	for n, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row T
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func loadArtifacts(dir string) (artifacts, error) {
	var a artifacts
	var err error
	if a.testJSON, err = os.ReadFile(filepath.Join(dir, testJSONFile)); err != nil {
		return artifacts{}, err
	}
	if a.rusage, err = readOptional(filepath.Join(dir, rusageFile)); err != nil {
		return artifacts{}, err
	}
	if a.samples, err = readOptional(filepath.Join(dir, samplesFile)); err != nil {
		return artifacts{}, err
	}
	return a, nil
}

func Run(jobsFile string, bindings []string) (Report, error) {
	data, err := os.ReadFile(jobsFile)
	if err != nil {
		return Report{}, err
	}
	jobs, err := parseJobs(data)
	if err != nil {
		return Report{}, err
	}
	bound := map[string]artifacts{}
	for _, binding := range bindings {
		name, dir, ok := strings.Cut(binding, "=")
		if !ok {
			return Report{}, fmt.Errorf("artifact binding %q is not <job name>=<directory>", binding)
		}
		if bound[name], err = loadArtifacts(dir); err != nil {
			return Report{}, err
		}
	}
	return build(input{jobs: jobs, artifacts: bound})
}
