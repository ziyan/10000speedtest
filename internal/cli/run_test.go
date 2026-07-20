package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ziyan/10000speedtest/internal/speedtest"
)

// TestStageFailure checks that a stage which ran but moved no data is reported,
// while stages that succeeded or did not run are not.
func TestStageFailure(t *testing.T) {
	moved := &speedtest.Result{Bytes: 100}
	empty := &speedtest.Result{Bytes: 0}

	if err := stageFailure(moved, moved); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := stageFailure(moved, nil); err != nil {
		t.Fatalf("expected success when a stage did not run, got %v", err)
	}
	if err := stageFailure(empty, moved); err == nil || !strings.Contains(err.Error(), "download") {
		t.Fatalf("expected a download failure, got %v", err)
	}
	if err := stageFailure(moved, empty); err == nil || !strings.Contains(err.Error(), "upload") {
		t.Fatalf("expected an upload failure, got %v", err)
	}
	if err := stageFailure(empty, empty); err == nil || !strings.Contains(err.Error(), "download and upload") {
		t.Fatalf("expected a combined failure, got %v", err)
	}
}

// TestResultJsonShape checks the JSON encoding: present stages are included,
// absent stages are omitted, and the field values come from the results.
func TestResultJsonShape(t *testing.T) {
	download := &speedtest.Result{
		Name:              "Download",
		Bytes:             1000,
		Elapsed:           2 * time.Second,
		MegabitsPerSecond: 4.0,
	}
	output := resultJson{
		Server:          "https://example",
		Connections:     8,
		DurationSeconds: 10,
		Download:        stageJsonFrom(download),
		Upload:          stageJsonFrom(nil),
	}

	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	text := string(encoded)

	if !strings.Contains(text, `"download"`) {
		t.Fatal("expected a download object")
	}
	if strings.Contains(text, `"upload"`) {
		t.Fatalf("expected upload to be omitted when the stage did not run, got %s", text)
	}
	if !strings.Contains(text, `"mbps": 4`) || !strings.Contains(text, `"bytes": 1000`) {
		t.Fatalf("expected download values in the JSON, got %s", text)
	}
}
