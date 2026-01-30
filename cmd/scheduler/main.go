package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"polyagent-backend/configs"
	"polyagent-backend/internal/executor"
	"polyagent-backend/internal/pkg/logger"
	"polyagent-backend/internal/repository"
	"polyagent-backend/internal/risk"
	"polyagent-backend/internal/scheduler"

	"go.uber.org/zap"
)

const configPath = "configs/config.yaml"

func main() {
	// 初始化日志
	log := logger.NewLogger()
	defer log.Sync()

	// 加载配置
	cfg, _ := configs.LoadConfig(configPath)

	// 初始化数据库
	repo, err := repository.NewPostgresRepository(cfg.Database)
	if err != nil {
		log.Fatal("初始化数据库失败", zap.Error(err))
	}

	// 初始化Polymarket客户端
	pmClient, err := executor.NewPolymarketClient(
		cfg.Polymarket.BaseURL,
		cfg.Polymarket.APIKey,
		cfg.Polymarket.APISecret,
		cfg.Polymarket.Passphrase,
		cfg.Polymarket.PrivateKey,
	)
	if err != nil {
		log.Fatal("初始化Polymarket客户端失败", zap.Error(err))
	}

	// 初始化组件
	auditor := risk.NewAuditor(repo, log)
	exec := executor.NewExecutor(repo, pmClient, log, cfg.WorkerCount)
	rtEngine := risk.NewRealtimeRiskEngine(repo, auditor, log, cfg.RealtimeCheckInterval)

	// 初始化调度器
	schedConfig := scheduler.Config{
		AuditInterval:         30 * time.Second,
		AuditBatchSize:        100,
		ExecuteInterval:       1 * time.Minute,
		ExecuteBatchSize:      50,
		SettlementTime:        "0 0 * * *", // 每天UTC 00:00
		AggregationInterval:   10 * time.Second,
		RealtimeCheckInterval: cfg.RealtimeCheckInterval,
	}

	sched, err := scheduler.NewScheduler(repo, auditor, exec, rtEngine, log, schedConfig)
	if err != nil {
		log.Fatal("初始化调度器失败", zap.Error(err))
	}

	// 启动所有组件
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exec.Start(ctx)
	if err := sched.Start(ctx); err != nil {
		log.Fatal("启动调度器失败", zap.Error(err))
	}

	log.Info("Polymarket定时调度系统已启动",
		zap.Int("workers", cfg.WorkerCount),
		zap.Duration("audit_interval", schedConfig.AuditInterval),
		zap.Duration("execute_interval", schedConfig.ExecuteInterval))

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("正在关闭系统...")
	sched.Stop()
	exec.Stop()

	log.Info("系统已安全关闭")
}
