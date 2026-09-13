// Package logger configures kconduit's shared logrus logger.
//
// Init sets up the global Log instance once per process (guarded by
// sync.Once) with the requested level and, optionally, a log file; callers
// elsewhere in kconduit use the package-level Log variable directly rather
// than passing a logger through every call.
package logger

import (
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	// Log is the global logger instance
	Log  *logrus.Logger
	once sync.Once
)

// Init initializes the global logger with the specified configuration
func Init(level, logFile string) error {
	var err error
	once.Do(func() {
		Log = logrus.New()

		// Set log level
		var logLevel logrus.Level
		logLevel, err = logrus.ParseLevel(level)
		if err != nil {
			return
		}
		Log.SetLevel(logLevel)

		// Set output
		if logFile != "" {
			var f *os.File
			f, err = os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return
			}
			Log.SetOutput(f)
		} else {
			// When no log file is specified, discard logs to avoid interfering with TUI
			// Unless we're in debug mode where we might want to see the output
			if logLevel == logrus.DebugLevel {
				// In debug mode without a file, create a debug log file
				var f *os.File
				f, err = os.OpenFile("kconduit-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					Log.SetOutput(io.Discard)
				} else {
					Log.SetOutput(f)
				}
			} else {
				// Discard all logs when no file is specified and not in debug mode
				Log.SetOutput(io.Discard)
			}
		}

		// Set formatter
		Log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	})
	return err
}

// Get returns the global logger instance, initializing with defaults if needed
func Get() *logrus.Logger {
	if Log == nil {
		// Initialize with info level and no output file (will discard logs)
		_ = Init("info", "")
	}
	return Log
}

// Fields is an alias for logrus.Fields for convenience
type Fields = logrus.Fields
