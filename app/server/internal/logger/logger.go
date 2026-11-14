// Package logger 提供基于 logrus 的统一日志模块。
//
// 用法：
//
//	logger.Init(logger.OptionsFrom(cfg.LoggerOptions()))
//	logger.Info("server started")
//	logger.WithComponent("Signal").Infof("client connected: %s", id)
//	logger.WithFields(logger.Fields{"room": room}).Warn("room full")
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Fields 结构化字段别名，避免业务侧直接依赖 logrus 类型名。
type Fields = logrus.Fields

// Level 日志级别字符串。
type Level string

const (
	LevelTrace Level = "trace"
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
	LevelFatal Level = "fatal"
	LevelPanic Level = "panic"
)

// Format 输出格式。
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Options 日志初始化选项。
type Options struct {
	// Level: trace|debug|info|warn|error|fatal|panic；空则默认 info
	Level string
	// Format: text|json；空则生产 json、开发 text
	Format string
	// Output: stdout|stderr|file|both；空则 stdout
	Output string
	// FilePath: file/both 时的日志文件路径
	FilePath string
	// ReportCaller: 是否打印调用方
	ReportCaller bool
	// Production: 影响默认 format/level 与文本着色
	Production bool
}

var (
	mu     sync.RWMutex
	global *logrus.Logger
	closer io.Closer
)

func init() {
	// 启动前也可用默认 logger，避免业务 import 时 nil。
	global = newDefaultLogger()
}

func newDefaultLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(os.Stdout)
	l.SetLevel(logrus.InfoLevel)
	l.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
	return l
}

// Init 按选项初始化全局 logger，可重复调用（会关闭旧文件句柄）。
func Init(opts Options) error {
	opts = normalizeOptions(opts)

	l := logrus.New()
	level, err := parseLevel(opts.Level)
	if err != nil {
		return err
	}
	l.SetLevel(level)
	l.SetReportCaller(opts.ReportCaller)

	formatter, err := buildFormatter(opts)
	if err != nil {
		return err
	}
	l.SetFormatter(formatter)

	out, c, err := buildOutput(opts)
	if err != nil {
		return err
	}
	l.SetOutput(out)

	mu.Lock()
	if closer != nil {
		_ = closer.Close()
	}
	global = l
	closer = c
	mu.Unlock()

	// 标准库 log / gin 默认 writer 统一转发
	RedirectStdLog()
	return nil
}

// InitFromEnv 用环境变量风格字段初始化；供尚未拿到 Config 的入口使用。
func InitFromEnv(level, format, output, filePath string, production bool) error {
	return Init(Options{
		Level:      level,
		Format:     format,
		Output:     output,
		FilePath:   filePath,
		Production: production,
	})
}

// L 返回全局 *logrus.Logger。
func L() *logrus.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return global
}

// Entry 返回带字段的 entry。
func Entry() *logrus.Entry {
	return logrus.NewEntry(L())
}

// WithFields 附加结构化字段。
func WithFields(fields Fields) *logrus.Entry {
	return L().WithFields(fields)
}

// WithField 附加单个字段。
func WithField(key string, value interface{}) *logrus.Entry {
	return L().WithField(key, value)
}

// WithComponent 附加 component 字段，便于按模块过滤。
func WithComponent(name string) *logrus.Entry {
	return L().WithField("component", name)
}

// WithError 附加 error 字段。
func WithError(err error) *logrus.Entry {
	return L().WithError(err)
}

// Writer 返回 info 级别 writer，可供 gin/std 使用。
func Writer() *io.PipeWriter {
	return L().Writer()
}

// WriterLevel 返回指定级别 writer。
func WriterLevel(level logrus.Level) *io.PipeWriter {
	return L().WriterLevel(level)
}

// Close 关闭文件输出句柄（进程退出前可调用）。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if closer == nil {
		return nil
	}
	err := closer.Close()
	closer = nil
	return err
}

// --- 便捷方法 ---

func Trace(args ...interface{})                 { L().Trace(args...) }
func Tracef(format string, args ...interface{}) { L().Tracef(format, args...) }
func Debug(args ...interface{})                 { L().Debug(args...) }
func Debugf(format string, args ...interface{}) { L().Debugf(format, args...) }
func Info(args ...interface{})                  { L().Info(args...) }
func Infof(format string, args ...interface{})  { L().Infof(format, args...) }
func Warn(args ...interface{})                  { L().Warn(args...) }
func Warnf(format string, args ...interface{})  { L().Warnf(format, args...) }
func Error(args ...interface{})                 { L().Error(args...) }
func Errorf(format string, args ...interface{}) { L().Errorf(format, args...) }
func Fatal(args ...interface{})                 { L().Fatal(args...) }
func Fatalf(format string, args ...interface{}) { L().Fatalf(format, args...) }
func Panic(args ...interface{})                 { L().Panic(args...) }
func Panicf(format string, args ...interface{}) { L().Panicf(format, args...) }

func normalizeOptions(opts Options) Options {
	opts.Level = strings.ToLower(strings.TrimSpace(opts.Level))
	opts.Format = strings.ToLower(strings.TrimSpace(opts.Format))
	opts.Output = strings.ToLower(strings.TrimSpace(opts.Output))
	opts.FilePath = strings.TrimSpace(opts.FilePath)

	if opts.Level == "" {
		if opts.Production {
			opts.Level = string(LevelInfo)
		} else {
			opts.Level = string(LevelDebug)
		}
	}
	if opts.Format == "" {
		if opts.Production {
			opts.Format = string(FormatJSON)
		} else {
			opts.Format = string(FormatText)
		}
	}
	if opts.Output == "" {
		opts.Output = "stdout"
	}
	if (opts.Output == "file" || opts.Output == "both") && opts.FilePath == "" {
		opts.FilePath = "logs/app.log"
	}
	return opts
}

func parseLevel(raw string) (logrus.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "trace":
		return logrus.TraceLevel, nil
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn", "warning":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	case "fatal":
		return logrus.FatalLevel, nil
	case "panic":
		return logrus.PanicLevel, nil
	default:
		return logrus.InfoLevel, fmt.Errorf("invalid log level %q", raw)
	}
}

func buildFormatter(opts Options) (logrus.Formatter, error) {
	switch opts.Format {
	case string(FormatJSON):
		return &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
		}, nil
	case string(FormatText):
		return &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
			ForceColors:     !opts.Production && isTerminal(),
			DisableColors:   opts.Production,
		}, nil
	default:
		return nil, fmt.Errorf("invalid log format %q (text|json)", opts.Format)
	}
}

func buildOutput(opts Options) (io.Writer, io.Closer, error) {
	switch opts.Output {
	case "stdout", "":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	case "file":
		f, err := openLogFile(opts.FilePath)
		if err != nil {
			return nil, nil, err
		}
		return f, f, nil
	case "both":
		f, err := openLogFile(opts.FilePath)
		if err != nil {
			return nil, nil, err
		}
		return io.MultiWriter(os.Stdout, f), f, nil
	default:
		return nil, nil, fmt.Errorf("invalid log output %q (stdout|stderr|file|both)", opts.Output)
	}
}

func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return f, nil
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
