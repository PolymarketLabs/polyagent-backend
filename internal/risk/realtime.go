package risk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"polyagent-backend/internal/models"
	"polyagent-backend/internal/pkg/logger"
	"polyagent-backend/internal/repository"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// RealtimeRiskEngine 实时风控引擎
type RealtimeRiskEngine struct {
	repo    repository.Repository
	auditor *Auditor
	logger  *logger.Logger

	// 监控配置
	checkInterval time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup

	// 止损执行器回调
	stopLossExecutor func(ctx context.Context, position models.Position) error
}

// NewRealtimeRiskEngine 创建实时风控引擎
func NewRealtimeRiskEngine(repo repository.Repository, auditor *Auditor,
	logger *logger.Logger, checkInterval time.Duration) *RealtimeRiskEngine {
	return &RealtimeRiskEngine{
		repo:          repo,
		auditor:       auditor,
		logger:        logger,
		checkInterval: checkInterval,
		stopCh:        make(chan struct{}),
	}
}

// SetStopLossExecutor 设置止损执行器
func (r *RealtimeRiskEngine) SetStopLossExecutor(executor func(ctx context.Context, position models.Position) error) {
	r.stopLossExecutor = executor
}

// Start 启动实时风控
func (r *RealtimeRiskEngine) Start(ctx context.Context) {
	r.logger.Info("启动实时风控引擎", zap.Duration("interval", r.checkInterval))
	r.wg.Add(1)
	go r.run(ctx)
}

// Stop 停止实时风控
func (r *RealtimeRiskEngine) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	r.logger.Info("实时风控引擎已停止")
}

// run 主循环
func (r *RealtimeRiskEngine) run(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	// 立即执行一次
	r.checkAllFunds(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.checkAllFunds(ctx)
		}
	}
}

// checkAllFunds 检查所有基金
func (r *RealtimeRiskEngine) checkAllFunds(ctx context.Context) {
	funds, err := r.repo.GetActiveFunds(ctx)
	if err != nil {
		r.logger.Error("获取活跃基金失败", zap.Error(err))
		return
	}

	for _, fund := range funds {
		if err := r.checkFund(ctx, fund); err != nil {
			r.logger.Error("检查基金风控失败",
				zap.String("fund_id", fund.ID.String()),
				zap.Error(err))
		}
	}
}

// checkFund 检查单个基金
func (r *RealtimeRiskEngine) checkFund(ctx context.Context, fund models.Fund) error {
	// 获取持仓
	positions, err := r.repo.GetFundPositions(ctx, fund.ID)
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 获取止损规则
	rules, err := r.repo.GetRiskRulesByType(ctx, fund.ID, models.RiskRuleTypeStopLoss)
	if err != nil || len(rules) == 0 {
		// 使用基金默认止损设置
		return r.checkStopLossWithDefault(ctx, fund, positions)
	}

	// 解析止损参数
	params, err := ParseRuleParams(models.RiskRuleTypeStopLoss, rules[0].Params)
	if err != nil {
		return fmt.Errorf("解析止损参数失败: %w", err)
	}
	stopLossParams := params.(StopLossParams)

	// 检查每个持仓
	for _, pos := range positions {
		if pos.Size.IsZero() {
			continue
		}

		// 计算当前亏损百分比
		lossPercent := r.calculateLossPercent(pos)

		if lossPercent.GreaterThan(stopLossParams.StopLossPercent) {
			r.logger.Warn("触发止损",
				zap.String("fund_id", fund.ID.String()),
				zap.String("market_id", pos.MarketID),
				zap.String("loss_percent", lossPercent.String()))

			// 记录风控事件
			event := &models.RiskEvent{
				FundID:   fund.ID,
				RuleType: models.RiskRuleTypeStopLoss,
				Severity: "CRITICAL",
				MarketID: pos.MarketID,
				Description: fmt.Sprintf("持仓亏损 %.2f%%，触发止损线 %.2f%%",
					lossPercent, stopLossParams.StopLossPercent),
				TriggeredAt: time.Now(),
			}
			if err := r.repo.CreateRiskEvent(ctx, event); err != nil {
				r.logger.Error("记录风控事件失败", zap.Error(err))
			}

			// 执行止损平仓
			if r.stopLossExecutor != nil {
				if err := r.stopLossExecutor(ctx, pos); err != nil {
					r.logger.Error("执行止损平仓失败", zap.Error(err))
					// 继续处理其他持仓
				}
			}
		}
	}

	return nil
}

// checkStopLossWithDefault 使用默认设置检查止损
func (r *RealtimeRiskEngine) checkStopLossWithDefault(ctx context.Context,
	fund models.Fund, positions []models.Position) error {

	if fund.StopLossPercent.IsZero() {
		return nil // 未设置止损
	}

	for _, pos := range positions {
		lossPercent := r.calculateLossPercent(pos)

		if lossPercent.GreaterThan(fund.StopLossPercent) {
			r.logger.Warn("触发默认止损",
				zap.String("fund_id", fund.ID.String()),
				zap.String("market_id", pos.MarketID))

			event := &models.RiskEvent{
				FundID:      fund.ID,
				RuleType:    models.RiskRuleTypeStopLoss,
				Severity:    "CRITICAL",
				MarketID:    pos.MarketID,
				Description: fmt.Sprintf("触发默认止损线 %.2f%%", fund.StopLossPercent),
				TriggeredAt: time.Now(),
			}
			r.repo.CreateRiskEvent(ctx, event)

			if r.stopLossExecutor != nil {
				r.stopLossExecutor(ctx, pos)
			}
		}
	}
	return nil
}

// calculateLossPercent 计算亏损百分比
func (r *RealtimeRiskEngine) calculateLossPercent(pos models.Position) decimal.Decimal {
	if pos.EntryPrice.IsZero() {
		return decimal.Zero
	}

	var lossPercent decimal.Decimal
	if pos.Size.GreaterThan(decimal.Zero) {
		// 多头仓位
		lossPercent = pos.EntryPrice.Sub(pos.CurrentPrice).
			Div(pos.EntryPrice).Mul(decimal.NewFromInt(100))
	} else {
		// 空头仓位
		lossPercent = pos.CurrentPrice.Sub(pos.EntryPrice).
			Div(pos.EntryPrice).Mul(decimal.NewFromInt(100))
	}

	if lossPercent.LessThan(decimal.Zero) {
		lossPercent = decimal.Zero
	}

	return lossPercent
}
