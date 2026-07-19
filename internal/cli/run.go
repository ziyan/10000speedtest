package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/ziyan/10000speedtest/internal/speedtest"
)

// run is the urfave/cli action: it builds the test configuration from the flags
// and runs the requested stages.
func run(ctx context.Context, command *cli.Command) error {
	config := speedtest.Config{
		Server:       command.String("server"),
		Connections:  command.Int("connections"),
		Duration:     command.Duration("duration"),
		DownloadSize: command.Int("download-size"),
		UploadSize:   command.Int("upload-size"),
		Insecure:     command.Bool("insecure"),
		Chart:        command.Bool("chart"),
	}
	mode := command.String("mode")

	fmt.Printf("Server:      %s\n", config.Server)
	fmt.Printf("Connections: %d\n", config.Connections)
	fmt.Printf("Duration:    %s per stage\n\n", config.Duration)

	tester := speedtest.New(config)
	if mode == "download" || mode == "both" {
		tester.Download()
	}
	if mode == "upload" || mode == "both" {
		tester.Upload()
	}
	return nil
}
