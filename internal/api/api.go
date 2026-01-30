package api

import (
	"polyagent-backend/internal/controller"
	"polyagent-backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SetupRouter 初始化全局路由配置
func SetupRouter(
	logger *zap.Logger,
	jwtSecret string,
	authCtrl *controller.AuthController,
	fundCtrl *controller.FundController,
	intentCtrl *controller.IntentController,
	investorCtrl *controller.InvestorController,
) *gin.Engine {
	r := gin.New()

	// 1. 注册全局中间件
	r.Use(middleware.LoggerMiddleware(logger))

	r.Use(middleware.JWTMiddleware(jwtSecret))

	r.Use(gin.Recovery()) // 异常捕获

	// 2. 基础 API 组
	v1 := r.Group("/api/v1")
	{
		// --- 公开接口 (不需要 JWT) ---
		auth := v1.Group("/auth")
		{
			auth.GET("/nonce", authCtrl.GetNonce) // 获取签名 Nonce
			auth.POST("/login", authCtrl.Login)   // 提交签名登录
		}

		// --- 受保护接口 (需要 JWT 校验) ---
		authorized := v1.Group("/")
		authorized.Use(middleware.JWTMiddleware(jwtSecret))
		{
			// 用户个人资料
			authorized.GET("/user/profile", authCtrl.GetProfile)
			authorized.POST("/user/apply-manager", authCtrl.ApplyManager)

			// 基金浏览 (投资人 & 经理共有)
			funds := authorized.Group("/funds")
			{
				funds.GET("", fundCtrl.List)       // 基金列表
				funds.GET("/:id", fundCtrl.Detail) // 基金详情
			}

			// 投资人私有接口
			investor := authorized.Group("/investor")
			investor.Use(middleware.RoleGuard("INVESTOR"))
			{
				investor.GET("/portfolio", investorCtrl.GetPortfolio) // 个人投资组合
				investor.GET("/history", investorCtrl.GetHistory)     // 申赎历史
				investor.GET("/rankings", investorCtrl.GetRankings)   // 收益榜单
			}

			// 基金经理私有接口 (核心非裁量执行模块)
			manager := authorized.Group("/manager")
			manager.Use(middleware.RoleGuard("MANAGER"))
			{
				manager.POST("/funds", fundCtrl.Create)            // 创建基金
				manager.GET("/my-funds", fundCtrl.ListManaged)     // 管理的基金列表
				manager.GET("/ai-pick", fundCtrl.GetAISuggestions) // AI 选品建议

				// 交易意图操作
				intents := manager.Group("/intents")
				{
					intents.POST("", intentCtrl.Submit) // 提交交易意图
					intents.GET("", intentCtrl.List)    // 意图执行追踪
				}
			}
		}
	}

	return r
}
