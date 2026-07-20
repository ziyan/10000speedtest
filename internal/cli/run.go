package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/ziyan/10000speedtest/internal/speedtest"
)

// run is the urfave/cli action: it builds the test configuration from the flags,
// runs the requested stages, and prints results as text or JSON.
func run(ctx context.Context, command *cli.Command) error {
	config := speedtest.Config{
		Server:       command.String("server"),
		Connections:  command.Int("connections"),
		Duration:     command.Duration("duration"),
		DownloadSize: command.Int("download-size"),
		UploadSize:   command.Int("upload-size"),
		Insecure:     command.Bool("insecure"),
		Chart:        command.Bool("chart"),
		JSONOutput:   command.Bool("json"),
	}
	mode := command.String("mode")

	if !config.JSONOutput {
		fmt.Printf("Server:      %s\n", config.Server)
		fmt.Printf("Connections: %d\n", config.Connections)
		fmt.Printf("Duration:    %s per stage\n\n", config.Duration)
	}

	tester := speedtest.New(config)
	var download, upload *speedtest.Result
	if mode == "download" || mode == "both" {
		result := tester.Download()
		download = &result
	}
	if mode == "upload" || mode == "both" {
		result := tester.Upload()
		upload = &result
	}

	if config.JSONOutput {
		if err := printJson(config, download, upload); err != nil {
			return err
		}
	}

	// A stage that ran but moved no data means the server was unreachable or
	// every request failed; report it so the process exits non-zero.
	return stageFailure(download, upload)
}

// stageFailure returns an error naming every stage that ran but transferred no
// data, or nil when the run succeeded.
func stageFailure(download, upload *speedtest.Result) error {
	var failed []string
	if download != nil && download.Bytes == 0 {
		failed = append(failed, "download")
	}
	if upload != nil && upload.Bytes == 0 {
		failed = append(failed, "upload")
	}
	if len(failed) == 0 {
		return nil
	}
	return fmt.Errorf("cli: %s transferred no data; check --server and connectivity (use --log-level debug for details)",
		strings.Join(failed, " and "))
}

// stageJson is the JSON shape of one stage's result.
type stageJson struct {
	Mbps    float64 `json:"mbps"`
	Bytes   int64   `json:"bytes"`
	Seconds float64 `json:"seconds"`
}

// resultJson is the JSON shape of a whole run.
type resultJson struct {
	Server          string     `json:"server"`
	Connections     int        `json:"connections"`
	DurationSeconds float64    `json:"durationSeconds"`
	Download        *stageJson `json:"download,omitempty"`
	Upload          *stageJson `json:"upload,omitempty"`
}

func printJson(config speedtest.Config, download, upload *speedtest.Result) error {
	output := resultJson{
		Server:          config.Server,
		Connections:     config.Connections,
		DurationSeconds: config.Duration.Seconds(),
		Download:        stageJsonFrom(download),
		Upload:          stageJsonFrom(upload),
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func stageJsonFrom(result *speedtest.Result) *stageJson {
	if result == nil {
		return nil
	}
	return &stageJson{
		Mbps:    result.MegabitsPerSecond,
		Bytes:   result.Bytes,
		Seconds: result.Elapsed.Seconds(),
	}
}
