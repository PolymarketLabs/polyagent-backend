package main

const configPath = "configs/config.yaml"

func main() {
	// // 1. 初始化基础组件 (底层)
	// logger, _ := zap.NewProduction()
	// db := database.InitDB()
	// cfg := config.Load()

	// // 2. 初始化 Repository/Service (中间层)
	// userRepo := repository.NewUserRepository(db)
	// userService := service.NewUserService(userRepo)

	// // 3. 初始化 Controller (顶层)
	// // 此时将 service 注入到 controller 中
	// authCtrl := controller.NewAuthController(userService, logger)
	// fundCtrl := controller.NewFundController(db)
	// // ... 其他控制器同理

	// // 4. 调用 SetupRouter 并注入所有 Controller
	// r := router.SetupRouter(
	//     logger,
	//     cfg.JWTSecret,
	//     authCtrl,
	//     fundCtrl,
	//     intentCtrl,
	//     investorCtrl,
	// )

	// // 5. 启动服务
	// r.Run(":8080")
}
