package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// --- 1. 统一响应格式 (Response Format) ---

// Response 基础响应结构
type Response struct {
	Code    int         `json:"code"`    // 业务状态码
	Data    interface{} `json:"data"`    // 数据内容
	Message string      `json:"message"` // 提示信息
}

// Success 成功响应封装
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    data,
		Message: "success",
	})
}

// Error 错误响应封装
func Error(c *gin.Context, httpCode int, businessCode int, msg string) {
	c.AbortWithStatusJSON(httpCode, Response{
		Code:    businessCode,
		Data:    nil,
		Message: msg,
	})
}

// --- 2. 基础控制器 (Base Controller) ---

type BaseController struct{}

// GetUserAddress 从 Context 获取中间件注入的地址
func (base *BaseController) GetUserAddress(c *gin.Context) string {
	addr, exists := c.Get("user_address")
	if !exists {
		return ""
	}
	return addr.(string)
}
