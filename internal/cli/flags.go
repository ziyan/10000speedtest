package cli

import (
	"time"

	"github.com/urfave/cli/v3"
)

var flags = []cli.Flag{
	&cli.StringFlag{
		Name:  "server",
		Value: "https://gz.10000gd.tech:12348",
		Usage: "base URL of the speed-test server",
	},
	&cli.StringFlag{
		Name:  "mode",
		Value: "both",
		Usage: "which test to run: download, upload, or both",
	},
	&cli.IntFlag{
		Name:  "connections",
		Value: 8,
		Usage: "number of parallel connections",
	},
	&cli.DurationFlag{
		Name:  "duration",
		Value: 10 * time.Second,
		Usage: "duration of each test stage",
	},
	&cli.IntFlag{
		Name:  "download-size",
		Value: 20,
		Usage: "size in MiB requested per download connection (path /shmfile/<N>)",
	},
	&cli.IntFlag{
		Name:  "upload-size",
		Value: 20,
		Usage: "size in MiB posted per upload connection",
	},
	&cli.BoolFlag{
		Name:  "insecure",
		Value: true,
		Usage: "skip TLS certificate verification (the test port serves a non-matching cert)",
	},
	&cli.BoolFlag{
		Name:  "chart",
		Value: true,
		Usage: "draw a live colored bar chart (falls back to a single line when output is not a terminal)",
	},
	&cli.BoolFlag{
		Name:  "json",
		Value: false,
		Usage: "print only the final results as JSON (no live progress)",
	},
	&cli.StringFlag{
		Name:  "interface",
		Value: "",
		Usage: "bind connections to a network interface name or source IP (needs matching source routing)",
	},
	&cli.StringFlag{
		Name:  "log-level",
		Value: "info",
		Usage: "log level (debug, info, notice, warning, error)",
	},
}
