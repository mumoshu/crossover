package log

// Logger is the logging interface for Crossover
//
// Inspired from, even though the implementation here doesn't completely match the idea explained in:
// - https://dave.cheney.net/2017/01/23/the-package-level-logger-anti-pattern
// - https://dave.cheney.net/2015/11/05/lets-talk-about-logging
type Logger interface {
	Infof(string, ...interface{})
	Errorf(string, ...interface{})
}
