package logger

import (
	"io"
	stdlog "log"
	"strings"

	"github.com/sirupsen/logrus"
)

// RedirectStdLog 将标准库 log 输出接到全局 logger（默认 info）。
// 兼容存量 log.Printf("[Signal] ...") 调用。
func RedirectStdLog() {
	stdlog.SetFlags(0)
	stdlog.SetOutput(&stdLogWriter{level: logrus.InfoLevel})
}

// RedirectStdLogLevel 按指定级别转发标准库 log。
func RedirectStdLogLevel(level logrus.Level) {
	stdlog.SetFlags(0)
	stdlog.SetOutput(&stdLogWriter{level: level})
}

type stdLogWriter struct {
	level logrus.Level
}

func (w *stdLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\r\n")
	if msg == "" {
		return len(p), nil
	}
	L().Log(w.level, msg)
	return len(p), nil
}

// LevelWriter 把写入内容按固定级别打到 logger。
type LevelWriter struct {
	Level logrus.Level
}

func (w LevelWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\r\n")
	if msg == "" {
		return len(p), nil
	}
	L().Log(w.Level, msg)
	return len(p), nil
}

// Ensure LevelWriter 实现 io.Writer
var _ io.Writer = LevelWriter{}
