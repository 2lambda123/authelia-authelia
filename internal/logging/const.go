package logging

import (
	"github.com/sirupsen/logrus"
)

// Log Format values.
const (
	FormatText = "text"
	FormatJSON = "json"
)

type LogLevel string

// Log Level values.
const (
	LevelTrace = "trace"
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

func (l LogLevel) Level() logrus.Level {
	switch l {
	case LevelError:
		return logrus.ErrorLevel
	case LevelWarn:
		return logrus.WarnLevel
	case LevelInfo:
		return logrus.InfoLevel
	case LevelDebug:
		return logrus.DebugLevel
	case LevelTrace:
		return logrus.TraceLevel
	default:
		return logrus.InfoLevel
	}
}

// Field names.
const (
	FieldRemoteIP   = "remote_ip"
	FieldMethod     = "method"
	FieldPath       = "path"
	FieldStatusCode = "status_code"
)
