package logger

import (
	"io"
	"strings"

	"github.com/sirupsen/logrus"
)

// LevelWriter 把写入内容按固定级别打到 logger（gin.DefaultWriter/Recovery 使用）。
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
