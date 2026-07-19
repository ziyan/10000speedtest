// Package logging wraps github.com/op/go-logging so every package can declare a
// module-scoped logger with `var log = logging.MustGetLogger("<package>")`.
package logging

import (
	"os"

	logging "github.com/op/go-logging"
)

var log = logging.MustGetLogger("logging") //nolint:unused

var format = logging.MustStringFormatter(
	"%{color}%{time:2006-01-02 15:04:05.000} %{module} [%{level}] %{message}%{color:reset}",
)

// Logger is the concrete logger type returned by MustGetLogger.
type Logger = logging.Logger

// MustGetLogger returns a logger scoped to the given module name.
func MustGetLogger(module string) *Logger {
	return logging.MustGetLogger(module)
}

// Setup configures the global logging backend at the given level (for example
// "info" or "debug"), writing formatted output to stderr.
func Setup(levelName string) {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	formatted := logging.NewBackendFormatter(backend, format)
	leveled := logging.AddModuleLevel(formatted)

	level, err := logging.LogLevel(levelName)
	if err != nil {
		level = logging.INFO
	}
	leveled.SetLevel(level, "")
	logging.SetBackend(leveled)
}
