// Command 10000speedtest measures download/upload throughput against the China
// Telecom Guangdong speed-test servers (the same backend used by the web page at
// https://10000.gd.cn/#/speed).
package main

import (
	"os"

	"github.com/ziyan/10000speedtest/internal/cli"
)

// version and commit are overridden at build time via -ldflags.
var (
	version = "0.3.0"
	commit  = "unknown"
)

func main() {
	cli.Run(version, commit, os.Args)
}
