package logs

import "log"

var logger *log.Logger

func Init(l *log.Logger) {
	logger = l
}

func Println(v ...interface{}) {
	logger.Println(v...)
}

func Printf(fmt string, v ...interface{}) {
	logger.Printf(fmt, v...)
}
