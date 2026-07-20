package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ziyan/10000speedtest/internal/speedtest"
)

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
