package scheduler

import (
	"context"
	"time"

	"polyagent-backend/internal/executor"
	"polyagent-backend/internal/models"
	"polyagent-backend/internal/pkg/logger"
	"polyagent-backend/internal/repository"
	"polyagent-backend/internal/risk"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Scheduler 定时调度器
type Scheduler struct {
	scheduler gocron.Scheduler
	repo      repository.Repository
	auditor   *risk.Auditor
	executor  *executor.Executor
	rtEngine  *risk.RealtimeRiskEngine
	logger    *logger.Logger

	// 配置
	config Config
}

// Config 调度配置
type Config struct {
	// 审计任务
	AuditInterval  time.Duration
	AuditBatchSize int

	// 执行任务
	ExecuteInterval  time.Duration
	ExecuteBatchSize int

	// 结算任务
	SettlementTime string // Cron表达式，如 "0 0 * * *" 每天零点

	// 数据聚合
	AggregationInterval time.Duration

	// 实时风控
	RealtimeCheckInterval time.Duration
}

// NewScheduler 创建调度器
func NewScheduler(repo repository.Repository, auditor *risk.Auditor,
	exec *executor.Executor, rtEngine *risk.RealtimeRiskEngine,
	logger *logger.Logger, config Config) (*Scheduler, error) {

	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		scheduler: s,
		repo:      repo,
		auditor:   auditor,
		executor:  exec,
		rtEngine:  rtEngine,
		logger:    logger,
		config:    config,
	}, nil
}

// Start 启动调度
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info("启动定时调度器")
	namespace := uuid.NameSpaceURL
	// 1. 风控审计任务 - 每30秒检查一次待审计意图
	if _, err := s.scheduler.NewJob(
		gocron.DurationJob(s.config.AuditInterval),
		gocron.NewTask(s.auditPendingIntents, ctx),
		gocron.WithIdentifier(uuid.NewSHA1(namespace, []byte("risk_audit"))),
		gocron.WithName("风控审计任务"),
	); err != nil {
		return err
	}

	// 2. 交易执行任务 - 每分钟检查一次已批准意图
	if _, err := s.scheduler.NewJob(
		gocron.DurationJob(s.config.ExecuteInterval),
		gocron.NewTask(s.executeApprovedIntents, ctx),
		gocron.WithIdentifier(uuid.NewSHA1(namespace, []byte("trade_execute"))),
		gocron.WithName("交易执行任务"),
	); err != nil {
		return err
	}

	// 3. 每日结算任务
	if _, err := s.scheduler.NewJob(
		gocron.CronJob(s.config.SettlementTime, false),
		gocron.NewTask(s.dailySettlement, ctx),
		gocron.WithIdentifier(uuid.NewSHA1(namespace, []byte("daily_settlement"))),
		gocron.WithName("每日结算任务"),
	); err != nil {
		return err
	}

	// 4. 数据聚合任务
	if _, err := s.scheduler.NewJob(
		gocron.DurationJob(s.config.AggregationInterval),
		gocron.NewTask(s.aggregateData, ctx),
		gocron.WithIdentifier(uuid.NewSHA1(namespace, []byte("data_aggregate"))),
		gocron.WithName("数据聚合任务"),
	); err != nil {
		return err
	}

	// 启动调度器
	s.scheduler.Start()

	// 启动实时风控
	s.rtEngine.SetStopLossExecutor(s.executor.ExecuteStopLoss)
	s.rtEngine.Start(ctx)

	return nil
}

// Stop 停止调度
func (s *Scheduler) Stop() {
	s.logger.Info("停止定时调度器")
	s.rtEngine.Stop()
	if err := s.scheduler.Shutdown(); err != nil {
		s.logger.Error("关闭调度器失败", zap.Error(err))
	}
}

// auditPendingIntents 审计待处理意图
func (s *Scheduler) auditPendingIntents(ctx context.Context) {
	s.logger.Debug("执行风控审计任务")

	// 获取待审计意图
	intents, err := s.repo.GetPendingIntents(ctx, s.config.AuditBatchSize)
	if err != nil {
		s.logger.Error("获取待审计意图失败", zap.Error(err))
		return
	}

	if len(intents) == 0 {
		return
	}

	s.logger.Info("开始批量风控审计", zap.Int("count", len(intents)))

	for _, intent := range intents {
		// 更新为审计中状态
		intent.Status = models.IntentStatusAuditing
		if err := s.repo.UpdateTradeIntent(ctx, &intent); err != nil {
			s.logger.Error("更新审计状态失败",
				zap.String("intent_id", intent.ID.String()),
				zap.Error(err))
			continue
		}

		// 执行审计
		result, err := s.auditor.AuditIntent(ctx, &intent)
		if err != nil {
			s.logger.Error("审计失败",
				zap.String("intent_id", intent.ID.String()),
				zap.Error(err))
			continue
		}

		if result.Passed {
			s.logger.Info("审计通过，提交执行",
				zap.String("intent_id", intent.ID.String()))
			// 提交到执行队列
			s.executor.SubmitTask(intent.ID)
		} else {
			s.logger.Warn("审计拒绝",
				zap.String("intent_id", intent.ID.String()),
				zap.String("reason", intent.RejectReason))
		}
	}
}

// executeApprovedIntents 执行已批准意图（兜底，主要依赖异步队列）
func (s *Scheduler) executeApprovedIntents(ctx context.Context) {
	// 检查是否有长时间未执行的已批准意图
	intents, err := s.repo.GetStaleApprovedIntents(ctx, 5*time.Minute, s.config.ExecuteBatchSize)
	if err != nil {
		s.logger.Error("获取滞留意图失败", zap.Error(err))
		return
	}

	for _, intent := range intents {
		s.logger.Warn("发现滞留意图，重新提交",
			zap.String("intent_id", intent.ID.String()),
			zap.Time("approved_at", intent.UpdatedAt))
		s.executor.SubmitTask(intent.ID)
	}
}

// dailySettlement 每日结算
func (s *Scheduler) dailySettlement(ctx context.Context) {
	s.logger.Info("执行每日结算")

	// 1. 计算所有基金NAV
	funds, err := s.repo.GetActiveFunds(ctx)
	if err != nil {
		s.logger.Error("获取基金列表失败", zap.Error(err))
		return
	}

	for _, fund := range funds {
		if err := s.calculateFundNAV(ctx, fund); err != nil {
			s.logger.Error("计算NAV失败",
				zap.String("fund_id", fund.ID.String()),
				zap.Error(err))
		}
	}

	// 2. 处理赎回请求
	if err := s.processRedemptions(ctx); err != nil {
		s.logger.Error("处理赎回失败", zap.Error(err))
	}

	// 3. 生成日报
	s.generateDailyReport(ctx)
}

// calculateFundNAV 计算基金NAV
func (s *Scheduler) calculateFundNAV(ctx context.Context, fund models.Fund) error {
	positions, err := s.repo.GetFundPositions(ctx, fund.ID)
	if err != nil {
		return err
	}

	var totalValue decimal.Decimal
	for _, pos := range positions {
		totalValue = totalValue.Add(pos.Size.Mul(pos.CurrentPrice))
	}

	// 更新基金AUM
	fund.TotalAUM = totalValue
	return s.repo.UpdateFund(ctx, &fund)
}

// processRedemptions 处理赎回
func (s *Scheduler) processRedemptions(ctx context.Context) error {
	// 实现赎回逻辑
	return nil
}

// generateDailyReport 生成日报
func (s *Scheduler) generateDailyReport(ctx context.Context) {
	// 实现报告生成逻辑
	s.logger.Info("生成每日结算报告")
}

// aggregateData 数据聚合
func (s *Scheduler) aggregateData(ctx context.Context) {
	s.logger.Debug("执行数据聚合")

	// 1. 更新市场价格
	if err := s.updateMarketPrices(ctx); err != nil {
		s.logger.Error("更新市场价格失败", zap.Error(err))
	}

	// 2. 更新持仓盈亏
	if err := s.updatePositionPnL(ctx); err != nil {
		s.logger.Error("更新持仓盈亏失败", zap.Error(err))
	}
}

// updateMarketPrices 更新市场价格
func (s *Scheduler) updateMarketPrices(ctx context.Context) error {
	// 获取所有活跃市场
	markets, err := s.repo.GetActiveMarkets(ctx)
	if err != nil {
		return err
	}

	for _, market := range markets {
		// 从Polymarket获取最新价格
		// 更新到数据库
		_ = market
	}

	return nil
}

// updatePositionPnL 更新持仓盈亏
func (s *Scheduler) updatePositionPnL(ctx context.Context) error {
	positions, err := s.repo.GetAllPositions(ctx)
	if err != nil {
		return err
	}

	for _, pos := range positions {
		// 获取当前市场价格
		currentPrice := pos.CurrentPrice // 实际应从市场数据获取

		// 计算未实现盈亏
		if pos.Size.GreaterThan(decimal.Zero) {
			pos.UnrealizedPnL = currentPrice.Sub(pos.EntryPrice).Mul(pos.Size)
		} else {
			pos.UnrealizedPnL = pos.EntryPrice.Sub(currentPrice).Mul(pos.Size.Abs())
		}

		pos.CurrentPrice = currentPrice
		pos.LastUpdated = time.Now()

		if err := s.repo.SavePosition(ctx, &pos); err != nil {
			s.logger.Error("更新持仓失败", zap.Error(err))
		}
	}

	return nil
}
