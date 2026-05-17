package logger

import (
	"os"

	"github.com/op/go-logging"
)

var (
	log       *logging.Logger
	debugMode bool
)

func InitLogger(name string, debug bool) {
	debugMode = debug

	logging.SetFormatter(logging.MustStringFormatter(buildFormat(debug)))
	logging.SetBackend(logging.NewLogBackend(os.Stdout, "", 0))

	level := logging.INFO
	if debug {
		level = logging.DEBUG
	}
	logging.SetLevel(level, name)

	log = logging.MustGetLogger(name)
	log.ExtraCalldepth = 1
}

func buildFormat(debug bool) string {
	base := `%{color}[%{time:2006-01-02 15:04:05}][%{level:.4s}][%{id:04x}]%{color:reset} %{message}`
	if debug {
		return base + ` | {%{shortpkg}.%{longfunc}}`
	}
	return base
}

func Logger() *logging.Logger { return log }

func Info(format string, args ...interface{})    { log.Infof(format, args...) }
func Infof(format string, args ...interface{})   { log.Infof(format, args...) }
func Notice(format string, args ...interface{})  { log.Noticef(format, args...) }
func Noticef(format string, args ...interface{}) { log.Noticef(format, args...) }
func Warning(format string, args ...interface{}) { log.Warningf(format, args...) }
func Warningf(format string, args ...interface{}) { log.Warningf(format, args...) }
func Warn(format string, args ...interface{})    { log.Warningf(format, args...) }
func Warnf(format string, args ...interface{})   { log.Warningf(format, args...) }
func Error(format string, args ...interface{})   { log.Errorf(format, args...) }
func Errorf(format string, args ...interface{})  { log.Errorf(format, args...) }
func Panic(args ...interface{})                  { log.Panic(args...) }
func Panicf(format string, args ...interface{})  { log.Panicf(format, args...) }
func Fatal(args ...interface{})                  { log.Fatal(args...) }
func Fatalf(format string, args ...interface{})  { log.Fatalf(format, args...) }

func Debug(format string, args ...interface{}) {
	if debugMode {
		log.Debugf(format, args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if debugMode {
		log.Debugf(format, args...)
	}
}

func Temp(format string, args ...interface{}) {}
