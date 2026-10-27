package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var contractCodeSpanRE = regexp.MustCompile("`([^`]+)`")

type contractDocRow struct {
	cells []string
}

func contractDocSection(t *testing.T, heading string) []contractDocRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(findRepoRoot(t), "docs", "architecture", "cli-contract.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, after, found := strings.Cut(string(body), "\n"+heading+"\n")
	if !found {
		t.Fatalf("cli-contract.md has no heading %q", heading)
	}
	var rows []contractDocRow
	for _, line := range strings.Split(after, "\n") {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			break
		}
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|---") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, contractDocRow{cells: cells})
	}
	if len(rows) < 2 {
		t.Fatalf("cli-contract.md %s has no table", heading)
	}
	return rows[1:]
}

func contractVerbPaths(cell string) []string {
	spans := contractCodeSpanRE.FindAllStringSubmatch(cell, -1)
	if len(spans) == 0 {
		return nil
	}
	first := strings.Fields(spans[0][1])
	paths := []string{strings.Join(first, " ")}
	prefix := strings.Join(first[:len(first)-1], " ")
	for _, s := range spans[1:] {
		paths = append(paths, strings.TrimSpace(prefix+" "+s[1]))
	}
	return paths
}

func TestCLIContract_DocMatchesTheRegistry(t *testing.T) {
	t.Parallel()
	inDoc := map[string]bool{}
	for _, row := range contractDocSection(t, "## 5. The core verbs") {
		class, jsonToday, rootToday, schemaCell := row.cells[1], row.cells[3], row.cells[4], row.cells[5]
		for _, path := range contractVerbPaths(row.cells[0]) {
			inDoc[path] = true
			v, ok := lookupCoreVerb(path)
			if !ok {
				t.Errorf("%s: §5 lists it, but coreVerbs has no row for it", path)
				continue
			}
			if v.class != class {
				t.Errorf("%s: §5 class %q, coreVerbs class %q", path, class, v.class)
			}
			if jsonToday != "yes" {
				t.Errorf("%s: §5 says json today %q; each core verb has --json", path, jsonToday)
			}
			if rootToday != "yes" {
				t.Errorf("%s: §5 says root today %q; each core verb has --project-root", path, rootToday)
			}
			if !strings.Contains(schemaCell, v.schema()) && !strings.Contains(schemaCell, strings.Replace(v.schema(), strings.Fields(path)[len(strings.Fields(path))-1], "<verb>", 1)) {
				t.Errorf("%s: §5 schema cell %q does not name %q", path, schemaCell, v.schema())
			}
			if lookupCommand(strings.Fields(path)[0]) == nil {
				t.Errorf("%s: the registry has no command %q", path, strings.Fields(path)[0])
			}
		}
	}
	for _, v := range coreVerbs {
		if !inDoc[v.path] {
			t.Errorf("%s: coreVerbs has it, but cli-contract.md §5 does not list it", v.path)
		}
	}
	for _, row := range contractDocSection(t, "## 6. Verbs to add") {
		for _, path := range contractVerbPaths(row.cells[0]) {
			if _, ok := lookupCoreVerb(strings.Join(strings.Fields(path)[:min(2, len(strings.Fields(path)))], " ")); ok {
				t.Errorf("%s: §6 lists it as a verb to add, but it is a core verb already", path)
			}
		}
	}
}

func TestCLIContract_DocPublishesTheExitTable(t *testing.T) {
	t.Parallel()
	inDoc := map[int]string{}
	for _, row := range contractDocSection(t, "## 3. Exit codes") {
		code, err := strconv.Atoi(row.cells[0])
		if err != nil {
			continue
		}
		inDoc[code] = strings.Trim(row.cells[1], "`")
	}
	for code, kind := range cliExitKinds {
		if inDoc[code] != kind {
			t.Errorf("exit %d: Go kind %q, cli-contract.md §3 %q", code, kind, inDoc[code])
		}
	}
	if len(inDoc) != len(cliExitKinds) {
		t.Errorf("cli-contract.md §3 has %d codes, Go has %d", len(inDoc), len(cliExitKinds))
	}
}

func TestContractVerbPaths_ExpandTheSubverbsOfOneCell(t *testing.T) {
	t.Parallel()
	got := contractVerbPaths("`inbox list`, `show`, `rank`")
	if want := []string{"inbox list", "inbox show", "inbox rank"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := contractVerbPaths("`gc`"); !slices.Equal(got, []string{"gc"}) {
		t.Errorf("got %v", got)
	}
	if got := contractVerbPaths("no span"); got != nil {
		t.Errorf("got %v", got)
	}
}
