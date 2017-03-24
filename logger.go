package tcpnetwork

import (
	"fmt"
	"log"
)

const (
	KLogLevelDebug = iota
	KLogLevelInfo
	KLogLevelWarn
	KLogLevelError
	KLogLevelFatal
)

var logLevelPrefix = []string{
	"[DBG] ",
	"[INF] ",
	"[WRN] ",
	"[ERR] ",
	"[FAL]",
}

//ILogger is an interface use for log message
type ILogger interface {
	Output(level int, calldepth int, f string) error
}

//	default logger
type defaultLogger struct {
}

func (d *defaultLogger) Output(level int, calldepth int, f string) error {
	text := logLevelPrefix[level] + "[tcpnetwork]" + f
	return log.Output(calldepth, text)
}

//global varibale
var myDefaultLogger defaultLogger
var myLogger ILogger = &myDefaultLogger

//Set the custom logger
func SetLogger(logger ILogger) {
	myLogger = logger
}

func _log(level int, calldepth int, f string) {
	myLogger.Output(level, calldepth, f)
}

func logDebug(f string, v ...interface{}) {
	_log(KLogLevelDebug, 2, fmt.Sprintf(f, v...))
}

func logInfo(f string, v ...interface{}) {
	_log(KLogLevelInfo, 2, fmt.Sprintf(f, v...))
}

func logWarn(f string, v ...interface{}) {
	_log(KLogLevelWarn, 2, fmt.Sprintf(f, v...))
}

func logError(f string, v ...interface{}) {
	_log(KLogLevelError, 2, fmt.Sprintf(f, v...))
}

func logFatal(f string, v ...interface{}) {
	_log(KLogLevelFatal, 2, fmt.Sprintf(f, v...))
}
