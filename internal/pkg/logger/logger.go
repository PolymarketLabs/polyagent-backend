package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 封装 zap.Logger
type Logger struct {
	*zap.Logger
	sugar *zap.SugaredLogger
}

// NewLogger 创建生产环境日志
func NewLogger() *Logger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.StacktraceKey = "stacktrace"

	// 可选：输出到文件
	// config.OutputPaths = []string{"stdout", "/var/log/scheduler.log"}

	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		panic(err)
	}

	return &Logger{
		Logger: logger,
		sugar:  logger.Sugar(),
	}
}

// NewDevelopmentLogger 创建开发环境日志（更友好的格式）
func NewDevelopmentLogger() *Logger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	return &Logger{
		Logger: logger,
		sugar:  logger.Sugar(),
	}
}

// Sugar 获取 SugaredLogger（支持格式化的便捷方法）
func (l *Logger) Sugar() *zap.SugaredLogger {
	return l.sugar
}

// Sync 刷新缓冲区
func (l *Logger) Sync() {
	_ = l.Logger.Sync()
}

// With 添加字段
func (l *Logger) With(fields ...zap.Field) *Logger {
	iface := make([]interface{}, len(fields))
	for i, f := range fields {
		iface[i] = f
	}
	return &Logger{
		Logger: l.Logger.With(fields...),
		sugar:  l.sugar.With(iface...),
	}
}

// Named 创建命名子logger
func (l *Logger) Named(name string) *Logger {
	return &Logger{
		Logger: l.Logger.Named(name),
		sugar:  l.sugar.Named(name),
	}
}
