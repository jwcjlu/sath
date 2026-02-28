package obs

import (
	"log"
	"os"
)

// Logger 是一个简单的结构化日志封装，当前基于标准库 log 实现。
// 未来可以替换为 zap/logrus 等更专业的实现。
type Logger struct {
	inner *log.Logger
}

var defaultLogger = NewLogger()

func NewLogger() *Logger {
	return &Logger{
		inner: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func L() *Logger {
	return defaultLogger
}

func (l *Logger) Info(msg string) {
	l.inner.Printf(`{"level":"info","msg":%q}`, msg)
}

func (l *Logger) Error(msg string) {
	l.inner.Printf(`{"level":"error","msg":%q}`, msg)
}
