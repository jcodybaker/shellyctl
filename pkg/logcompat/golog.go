package logcompat

import (
	golog "log"
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var mdnsLog = regexp.MustCompile(`^\[(INFO|DBG|DEBUG|WARN|ERR|FATAL|PANIC)\] (\w+): (.*)`)
var vanillaLog = regexp.MustCompile(`^(\d\d\d\d/\d\d/\d\d \d\d:\d\d:\d\d(.\d\d\d\d\d\d)?) (.*)`)

var goLogFlagsWarning sync.Once

func (l *LogWriter) Write(b []byte) (int, error) {
	msg := b
	if golog.Flags() != golog.LstdFlags {
		goLogFlagsWarning.Do(func() {
			l.log.Warn().Msg("non-standard go log flags mean stdlib go logging will not be parsed to zerolog")
		})
		return l.log.Write(b)
	}
	var e *zerolog.Event = l.log.Log()

	tsMatches := vanillaLog.FindSubmatch(b)
	if len(tsMatches) < 3 {
		// Provided the golog flags matches we shouldn't get here because the ts should exist. But
		// if we do, just pass through.
		return l.log.Write(b)
	}
	msg = tsMatches[len(tsMatches)-1]

	if matches := mdnsLog.FindSubmatch(msg); len(matches) == 4 {
		switch string(matches[1]) {
		case "INFO":
			// NOTE: The mDNS library is too noisy, downgrade INFO msgs to DEBUG.
			e = l.log.Debug()
		case "ERR":
			e = l.log.Error()
		case "DBG", "DEBUG":
			e = l.log.Debug()
		case "WARN":
			e = l.log.Warn()
		case "FATAL":
			e = l.log.Fatal()
		case "PANIC":
			e = l.log.Panic()
		}
		if len(matches[2]) > 0 {
			e = e.Str("component", string(matches[2]))
		}
		msg = matches[len(matches)-1]
	}

	// It'd makes sense to do this TS first, but it seems we can't change the level of a log
	// once it's been created. So we parse the TS first, pickup the level if possible, and then
	// apply the TS.
	if t, err := time.Parse("2006/01/02 15:04:05.999999", string(tsMatches[1])); err == nil {
		e = e.Time(zerolog.TimestampFieldName, t)
	} else if t, err := time.Parse("2006/01/02 15:04:05", string(tsMatches[1])); err == nil {
		e = e.Time(zerolog.TimestampFieldName, t)
	}
	e.Msg(string(msg))
	return len(b), nil
}
