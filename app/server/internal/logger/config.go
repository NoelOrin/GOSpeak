package logger

// ConfigLike 避免 logger 包直接依赖 config 包，防止循环引用。
type ConfigLike interface {
	IsProduction() bool
	GetLogLevel() string
	GetLogFormat() string
	GetLogOutput() string
	GetLogFile() string
	GetLogCaller() bool
}

// OptionsFrom 从任意带日志字段的配置构造 Options。
func OptionsFrom(level, format, output, file string, caller, production bool) Options {
	return Options{
		Level:        level,
		Format:       format,
		Output:       output,
		FilePath:     file,
		ReportCaller: caller,
		Production:   production,
	}
}
