package llmcalls

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestFallbackVocabulary_Named(t *testing.T) {
	t.Parallel()

	rows := Aggregate([]Record{{SchemaVersion: SchemaVersion, Phase: "build"}})
	if len(rows) != 1 {
		t.Fatalf("Aggregate rows = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.DispatchSource != DispatchUnknown {
		t.Errorf("DispatchSource = %q, want %q", got.DispatchSource, DispatchUnknown)
	}
	if got.MeasurementSource != SourceUnknown {
		t.Errorf("MeasurementSource = %q, want %q", got.MeasurementSource, SourceUnknown)
	}
	if got.TimingScope != TimingLegacyUnknown {
		t.Errorf("TimingScope = %q, want %q", got.TimingScope, TimingLegacyUnknown)
	}
	if got.Outcome != OutcomeUnknown {
		t.Errorf("Outcome = %q, want %q", got.Outcome, OutcomeUnknown)
	}
}

func TestDispatchVocabulary_Named(t *testing.T) {
	t.Parallel()

	got := []string{
		DispatchREPL,
		DispatchPositional,
		DispatchCLIDefault,
		DispatchResumed,
		DispatchNotStarted,
	}
	want := []string{
		"repl",
		"positional",
		"cli_default",
		"resumed_session",
		"not_started",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatch vocabulary = %q, want %q", got, want)
	}
}

func TestStoreResultTypes_Named(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.ndjson")
	destination := filepath.Join(dir, "destination.ndjson")
	record := Record{SchemaVersion: SchemaVersion, CallID: "named-result", Phase: "build"}
	if err := Append(source, record); err != nil {
		t.Fatalf("Append source: %v", err)
	}

	var readResult ReadResult
	var err error
	readResult, err = Read(source)
	if err != nil {
		t.Fatalf("Read source: %v", err)
	}
	if len(readResult.Records) != 1 || readResult.Skipped != 0 {
		t.Fatalf("ReadResult = %+v, want one record and no skips", readResult)
	}

	var importResult ImportResult
	importResult, err = Import(destination, source)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	wantImport := ImportResult{Imported: 1}
	if importResult != wantImport {
		t.Fatalf("ImportResult = %+v, want %+v", importResult, wantImport)
	}
}

func TestImportTagged_Named(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "source.ndjson")
	destination := filepath.Join(dir, "destination.ndjson")
	if err := Append(source, Record{SchemaVersion: SchemaVersion, CallID: "tagged", Phase: "build"}); err != nil {
		t.Fatalf("Append source: %v", err)
	}
	result, err := ImportTagged(destination, source, "run-1", "profile-1")
	if err != nil {
		t.Fatalf("ImportTagged: %v", err)
	}
	if result != (ImportResult{Imported: 1}) {
		t.Fatalf("ImportResult = %+v, want one imported", result)
	}
	again, err := ImportTagged(destination, source, "run-2", "profile-2")
	if err != nil || again != (ImportResult{Duplicates: 1}) {
		t.Fatalf("second ImportTagged = %+v, %v; want one duplicate", again, err)
	}
	read, err := Read(destination)
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("Read = %+v, %v", read, err)
	}
	if got := read.Records[0]; got.RunID != "run-1" || got.Profile != "profile-1" {
		t.Fatalf("tags = %q/%q, want run-1/profile-1", got.RunID, got.Profile)
	}
}
