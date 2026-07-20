// Package cli wires the command-line interface for 10000speedtest on top of
// github.com/urfave/cli/v3.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/ziyan/10000speedtest/internal/logging"
)

var log = logging.MustGetLogger("cli")

// Run parses arguments and executes the speed test, exiting non-zero on error.
func Run(version, commit string, arguments []string) {
	command := &cli.Command{
		Name:    "10000speedtest",
		Usage:   "Measure download/upload speed against the China Telecom Guangdong speed-test servers",
		Version: fmt.Sprintf("%s+%s", version, commit),
		Flags:   flags,
		Before: func(ctx context.Context, command *cli.Command) (context.Context, error) {
			logging.Setup(command.String("log-level"))
			log.Debugf("10000speedtest %s+%s", version, commit)
			return ctx, nil
		},
		Action: run,
	}

	if err := command.Run(context.Background(), arguments); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
