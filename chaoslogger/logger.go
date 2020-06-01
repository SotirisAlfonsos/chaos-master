package chaoslogger

import (
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

// AllowedLevel is a settable identifier for the minimum level a log entry
// must be have.
type AllowedLevel struct {
	s string
	o level.Option
}

// Set updates the value of the allowed level.
func (l *AllowedLevel) Set(s string) error {
	switch s {
	case "debug":
		l.o = level.AllowDebug()
	case "info":
		l.o = level.AllowInfo()
	case "warn":
		l.o = level.AllowWarn()
	case "error":
		l.o = level.AllowError()
	default:
		return errors.Errorf("unrecognized log level " + s)
	}

	l.s = s

	return nil
}

// New returns a new leveled oklog logger. Each logged line will be annotated
// with a timestamp. The output always goes to stderr.
func New(allowedLevel *AllowedLevel) log.Logger {
	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	l = level.NewFilter(l, allowedLevel.o)
	l = log.With(l, "ts", timestampFormat(), "caller", log.DefaultCaller)

	return l
}

func timestampFormat() log.Valuer {
	return log.TimestampFormat(
		func() time.Time { return time.Now().UTC() },
		"2006-01-02T15:04:05.000Z07:00",
	)
}
