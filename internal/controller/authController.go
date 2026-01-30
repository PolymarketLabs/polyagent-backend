package controller

import (
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	BaseController
	// 这里通常会注入 Service 接口
	// AuthService service.AuthService
}

// 处理获取登录 Nonce 的请求
func (a *AuthController) GetNonce(c *gin.Context) {
	Success(c, "GetNonce Success")
}

// 处理用户登录请求
func (a *AuthController) Login(c *gin.Context) {
	Success(c, "Login Success")
}

// 获取用户个人资料
func (a *AuthController) GetProfile(c *gin.Context) {
	Success(c, "GetProfile Success")
}

// 处理用户申请成为基金经理的请求
func (a *AuthController) ApplyManager(c *gin.Context) {
	Success(c, "ApplyManager Success")
}
