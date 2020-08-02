package log

import (
	"log"
	"os"
)

type stdLogger struct {
	info, err *log.Logger
}

var StdLogger = NewStdLogger(
	log.New(os.Stdout, "", log.LstdFlags),
	log.New(os.Stderr, "", log.LstdFlags),
)

func NewStdLogger(info, err *log.Logger) *stdLogger {
	return &stdLogger{
		info: info,
		err:  err,
	}
}

func (l *stdLogger) Infof(f string, args ...interface{}) {
	l.info.Printf(f, args...)
}

func (l *stdLogger) Errorf(f string, args ...interface{}) {
	l.err.Printf(f, args...)
}
