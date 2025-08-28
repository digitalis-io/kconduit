package logger

import (
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
			Log.SetOutput(os.Stderr) // Use stderr to not interfere with TUI
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
		Init("info", "")
	}
	return Log
}