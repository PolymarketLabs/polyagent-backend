package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoggerMiddleware 使用 zap 记录结构化日志
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 结束时间
		end := time.Now()
		latency := end.Sub(start)

		// 状态码
		status := c.Writer.Status()

		// 尝试从 Context 中获取 JWT 解析出的用户地址 (在 jwt_middleware.go 中设置)
		userAddress, _ := c.Get("user_address")
		if userAddress == nil {
			userAddress = "anonymous"
		}

		// 记录结构化日志
		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.String("user_address", userAddress.(string)),
			zap.Duration("latency", latency),
			zap.String("time", end.Format(time.RFC3339)),
		}

		// 如果有错误信息，也一并记录
		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				logger.Error(e, fields...)
			}
		} else {
			if status >= 500 {
				logger.Error("server error", fields...)
			} else if status >= 400 {
				logger.Warn("client error", fields...)
			} else {
				logger.Info("request success", fields...)
			}
		}
	}
}

// InitLogger 初始化 zap 日志实例
func InitLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout", "./logs/polyagent.log"} // 同时输出到控制台和文件
	return config.Build()
}
