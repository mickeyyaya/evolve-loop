package llmcalls

// apicover_named_test.go names and exercises the exported telemetry vocabulary
// and result types that callers consume as data. The broader behavioral suites
// cover the storage and aggregation paths in depth.

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
