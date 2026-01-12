package utils

import "github.com/sirupsen/logrus"

const (
	debug   = "debug"
	warning = "warning"
	info    = "info"
	error_  = "error"
	fatal   = "fatal"
)

var Log *logrus.Logger

func InitLogger(logLevel string) *logrus.Logger {
	Log = logrus.New()

	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	switch logLevel {
	case debug:
		Log.SetLevel(logrus.DebugLevel)
	case warning:
		Log.SetLevel(logrus.WarnLevel)
	case info:
		Log.SetLevel(logrus.InfoLevel)
	case error_:
		Log.SetLevel(logrus.ErrorLevel)
	case fatal:
		Log.SetLevel(logrus.FatalLevel)
	default:
		Log.SetLevel(logrus.ErrorLevel)
	}

	return Log
}
