# Eval: phasecoherence-check-nil-guard-coverage

## Task
Add unit tests for four uncovered branches in `phasecoherence.Check` (currently 81.4%):
1. `opts.AgentsFS == nil` guard → returns error "missing AgentsFS"
2. Directory entry in `agents/` is silently skipped (no `IsDir` branch tested)
3. Persona file with no frontmatter (`fm == nil`) is skipped without error
4. `tools` frontmatter value that is NOT a `[]string` (e.g. a plain string) is skipped

## Acceptance Criteria

### AC1: nil AgentsFS returns an error
```bash
cd go && go test -v -run TestCoherence_NilAgentsFSErrors ./internal/phasecoherence/...
```
[code] must exit 0 and print `PASS`

### AC2: directory entry in agents/ is skipped cleanly
```bash
cd go && go test -v -run TestCoherence_DirectoryEntrySkipped ./internal/phasecoherence/...
```
[code] must exit 0 and print `PASS`

### AC3: persona with no frontmatter is skipped without error
```bash
cd go && go test -v -run TestCoherence_NilFrontmatterSkipped ./internal/phasecoherence/...
```
[code] must exit 0 and print `PASS`

### AC4: non-slice `tools` frontmatter value is skipped without error
```bash
cd go && go test -v -run TestCoherence_NonSliceToolsValSkipped ./internal/phasecoherence/...
```
[code] must exit 0 and print `PASS`

### AC5: coverage improvement on `phasecoherence/coherence.go`
```bash
cd go && go test -coverprofile=/tmp/cover_pc.out ./internal/phasecoherence/... && go tool cover -func=/tmp/cover_pc.out | grep "coherence.go"
```
[code] `Check` coverage must be ≥ 88% (up from 81.4%)

### AC6 (negative): nil AgentsFS error message names the field
```bash
cd go && go test -v -run TestCoherence_NilAgentsFSErrors ./internal/phasecoherence/... 2>&1 | grep -i "AgentsFS\|agents"
```
[code] must exit 0 (error message is descriptive); empty grep output is a FAIL

### AC7 (edge): all existing phasecoherence tests remain green
```bash
cd go && go test ./internal/phasecoherence/...
```
[code] must exit 0
