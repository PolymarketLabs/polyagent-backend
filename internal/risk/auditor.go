package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"polyagent-backend/internal/models"
	"polyagent-backend/internal/pkg/logger"
	"polyagent-backend/internal/repository"
)

// Auditor 风控审计器
type Auditor struct {
	repo   repository.Repository
	logger *logger.Logger
}

// AuditResult 审计结果
type AuditResult struct {
	Passed         bool              `json:"passed"`
	Checks         []RuleCheckResult `json:"checks"`
	TotalRiskScore int               `json:"total_risk_score"`
}

// RuleCheckResult 单规则检查结果
type RuleCheckResult struct {
	RuleType models.RiskRuleType `json:"rule_type"`
	Passed   bool                `json:"passed"`
	Score    int                 `json:"score"` // 0-100, 越高越危险
	Message  string              `json:"message"`
}

// NewAuditor 创建审计器
func NewAuditor(repo repository.Repository, logger *logger.Logger) *Auditor {
	return &Auditor{
		repo:   repo,
		logger: logger,
	}
}

// AuditIntent 审计交易意图
func (a *Auditor) AuditIntent(ctx context.Context, intent *models.TradeIntent) (*AuditResult, error) {
	a.logger.Info("开始风控审计",
		zap.String("intent_id", intent.ID.String()),
		zap.String("fund_id", intent.FundID.String()),
		zap.String("market_id", intent.MarketID))

	// 获取基金风控规则
	rules, err := a.repo.GetActiveRiskRules(ctx, intent.FundID)
	if err != nil {
		return nil, fmt.Errorf("获取风控规则失败: %w", err)
	}

	result := &AuditResult{
		Passed: true,
		Checks: make([]RuleCheckResult, 0),
	}

	// 获取当前持仓和基金信息
	positions, err := a.repo.GetFundPositions(ctx, intent.FundID)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	fund, err := a.repo.GetFund(ctx, intent.FundID)
	if err != nil {
		return nil, fmt.Errorf("获取基金信息失败: %w", err)
	}

	// 获取当前市场价格（模拟，实际应从Polymarket API获取）
	currentPrice := decimal.NewFromFloat(0.5) // 示例价格

	// 执行各项规则检查
	for _, rule := range rules {
		checkResult := a.checkRule(ctx, rule, intent, positions, fund, currentPrice)
		result.Checks = append(result.Checks, checkResult)
		result.TotalRiskScore += checkResult.Score

		if !checkResult.Passed {
			result.Passed = false
		}

		// 记录审计日志
		auditLog := &models.AuditLog{
			IntentID:  intent.ID,
			RuleType:  rule.RuleType,
			Result:    map[bool]string{true: "PASS", false: "FAIL"}[checkResult.Passed],
			Details:   checkResult.Message,
			CheckedAt: time.Now(),
		}
		if err := a.repo.CreateAuditLog(ctx, auditLog); err != nil {
			a.logger.Error("记录审计日志失败", zap.Error(err))
		}
	}

	// 更新意图状态
	if result.Passed {
		intent.Status = models.IntentStatusApproved
		intent.AuditResult = a.serializeResult(result)
	} else {
		intent.Status = models.IntentStatusRejected
		intent.RejectReason = a.formatRejectReason(result.Checks)
		intent.AuditResult = a.serializeResult(result)
	}

	if err := a.repo.UpdateTradeIntent(ctx, intent); err != nil {
		return nil, fmt.Errorf("更新意图状态失败: %w", err)
	}

	a.logger.Info("风控审计完成",
		zap.String("intent_id", intent.ID.String()),
		zap.Bool("passed", result.Passed),
		zap.Int("risk_score", result.TotalRiskScore))

	return result, nil
}

// checkRule 执行单条规则检查
func (a *Auditor) checkRule(ctx context.Context, rule models.RiskRule,
	intent *models.TradeIntent, positions []models.Position,
	fund *models.Fund, currentPrice decimal.Decimal) RuleCheckResult {

	params, err := ParseRuleParams(rule.RuleType, rule.Params)
	if err != nil {
		return RuleCheckResult{
			RuleType: rule.RuleType,
			Passed:   false,
			Score:    100,
			Message:  fmt.Sprintf("规则参数解析失败: %v", err),
		}
	}

	switch rule.RuleType {
	case models.RiskRuleTypePositionLimit:
		return a.checkPositionLimit(params.(PositionLimitParams), intent, positions, fund)
	case models.RiskRuleTypeDailyLossLimit:
		return a.checkDailyLossLimit(params.(DailyLossLimitParams), fund)
	case models.RiskRuleTypePriceDeviation:
		return a.checkPriceDeviation(params.(PriceDeviationParams), intent, currentPrice)
	case models.RiskRuleTypeConcentration:
		return a.checkConcentration(params.(ConcentrationParams), intent, positions, fund)
	case models.RiskRuleTypeStopLoss:
		return a.checkStopLoss(params.(StopLossParams), positions)
	default:
		return RuleCheckResult{
			RuleType: rule.RuleType,
			Passed:   false,
			Score:    50,
			Message:  "未知的规则类型",
		}
	}
}

// checkPositionLimit 检查仓位限制
func (a *Auditor) checkPositionLimit(params PositionLimitParams,
	intent *models.TradeIntent, positions []models.Position, fund *models.Fund) RuleCheckResult {

	// 检查单笔交易上限
	if intent.Size.GreaterThan(params.MaxSinglePosition) {
		return RuleCheckResult{
			RuleType: models.RiskRuleTypePositionLimit,
			Passed:   false,
			Score:    80,
			Message:  fmt.Sprintf("交易数量 %s 超过单笔上限 %s", intent.Size, params.MaxSinglePosition),
		}
	}

	// 计算该市场当前持仓
	var currentMarketSize decimal.Decimal
	for _, pos := range positions {
		if pos.MarketID == intent.MarketID && pos.OutcomeID == intent.OutcomeID {
			currentMarketSize = currentMarketSize.Add(pos.Size)
		}
	}

	// 检查单个市场最大仓位
	newSize := currentMarketSize.Add(intent.Size)
	if newSize.GreaterThan(params.MaxPositionSize) {
		return RuleCheckResult{
			RuleType: models.RiskRuleTypePositionLimit,
			Passed:   false,
			Score:    70,
			Message:  fmt.Sprintf("市场持仓 %s 将超过上限 %s", newSize, params.MaxPositionSize),
		}
	}

	// 检查总敞口
	var totalExposure decimal.Decimal
	for _, pos := range positions {
		totalExposure = totalExposure.Add(pos.Size.Mul(pos.CurrentPrice))
	}
	totalExposure = totalExposure.Add(intent.Size.Mul(intent.Price))

	if totalExposure.GreaterThan(params.MaxTotalExposure) {
		return RuleCheckResult{
			RuleType: models.RiskRuleTypePositionLimit,
			Passed:   false,
			Score:    75,
			Message:  fmt.Sprintf("总敞口 %s 将超过上限 %s", totalExposure, params.MaxTotalExposure),
		}
	}

	return RuleCheckResult{
		RuleType: models.RiskRuleTypePositionLimit,
		Passed:   true,
		Score:    10,
		Message:  "仓位检查通过",
	}
}

// checkDailyLossLimit 检查日亏损限制
func (a *Auditor) checkDailyLossLimit(params DailyLossLimitParams, fund *models.Fund) RuleCheckResult {
	// 获取今日已实现亏损
	todayLoss := a.calculateTodayLoss(fund.ID)

	if todayLoss.GreaterThan(params.MaxDailyLoss) {
		return RuleCheckResult{
			RuleType: models.RiskRuleTypeDailyLossLimit,
			Passed:   false,
			Score:    90,
			Message:  fmt.Sprintf("今日亏损 %s 已超过限制 %s", todayLoss, params.MaxDailyLoss),
		}
	}

	// 计算风险分数
	score := int(todayLoss.Div(params.MaxDailyLoss).Mul(decimal.NewFromInt(100)).IntPart())
	if score > 100 {
		score = 100
	}

	return RuleCheckResult{
		RuleType: models.RiskRuleTypeDailyLossLimit,
		Passed:   true,
		Score:    score,
		Message:  fmt.Sprintf("今日亏损 %s，限制 %s", todayLoss, params.MaxDailyLoss),
	}
}

// checkPriceDeviation 检查价格偏离
func (a *Auditor) checkPriceDeviation(params PriceDeviationParams,
	intent *models.TradeIntent, currentPrice decimal.Decimal) RuleCheckResult {

	if intent.Price.IsZero() {
		// 市价单不检查
		return RuleCheckResult{
			RuleType: models.RiskRuleTypePriceDeviation,
			Passed:   true,
			Score:    0,
			Message:  "市价单，跳过价格偏离检查",
		}
	}

	deviation := intent.Price.Sub(currentPrice).Abs().Div(currentPrice).Mul(decimal.NewFromInt(100))
	maxDeviation := params.MaxDeviationPercent

	if deviation.GreaterThan(maxDeviation) {
		return RuleCheckResult{
			RuleType: models.RiskRuleTypePriceDeviation,
			Passed:   false,
			Score:    int(deviation.IntPart()),
			Message:  fmt.Sprintf("价格偏离 %.2f%% 超过限制 %.2f%%", deviation, maxDeviation),
		}
	}

	return RuleCheckResult{
		RuleType: models.RiskRuleTypePriceDeviation,
		Passed:   true,
		Score:    int(deviation.IntPart()),
		Message:  fmt.Sprintf("价格偏离 %.2f%%，限制 %.2f%%", deviation, maxDeviation),
	}
}

// checkConcentration 检查集中度
func (a *Auditor) checkConcentration(params ConcentrationParams,
	intent *models.TradeIntent, positions []models.Position, fund *models.Fund) RuleCheckResult {

	// 计算该市场持仓价值
	var marketValue decimal.Decimal
	for _, pos := range positions {
		if pos.MarketID == intent.MarketID {
			marketValue = marketValue.Add(pos.Size.Mul(pos.CurrentPrice))
		}
	}
	marketValue = marketValue.Add(intent.Size.Mul(intent.Price))

	// 计算集中度
	if fund.TotalAUM.IsZero() {
		return RuleCheckResult{
			RuleType: models.RiskRuleTypeConcentration,
			Passed:   true,
			Score:    0,
			Message:  "AUM为零，跳过集中度检查",
		}
	}

	concentration := marketValue.Div(fund.TotalAUM).Mul(decimal.NewFromInt(100))
	maxConcentration := params.MaxConcentrationPercent

	if concentration.GreaterThan(maxConcentration) {
		return RuleCheckResult{
			RuleType: models.RiskRuleTypeConcentration,
			Passed:   false,
			Score:    int(concentration.IntPart()),
			Message:  fmt.Sprintf("市场集中度 %.2f%% 超过限制 %.2f%%", concentration, maxConcentration),
		}
	}

	return RuleCheckResult{
		RuleType: models.RiskRuleTypeConcentration,
		Passed:   true,
		Score:    int(concentration.IntPart()),
		Message:  fmt.Sprintf("市场集中度 %.2f%%，限制 %.2f%%", concentration, maxConcentration),
	}
}

// checkStopLoss 检查止损线（用于实时风控）
func (a *Auditor) checkStopLoss(params StopLossParams, positions []models.Position) RuleCheckResult {
	// 检查是否有持仓触发止损
	for _, pos := range positions {
		if pos.Size.IsZero() {
			continue
		}

		// 计算亏损百分比
		var lossPercent decimal.Decimal
		if pos.Size.GreaterThan(decimal.Zero) {
			// 多头
			lossPercent = pos.EntryPrice.Sub(pos.CurrentPrice).Div(pos.EntryPrice).Mul(decimal.NewFromInt(100))
		} else {
			// 空头
			lossPercent = pos.CurrentPrice.Sub(pos.EntryPrice).Div(pos.EntryPrice).Mul(decimal.NewFromInt(100))
		}

		if lossPercent.GreaterThan(params.StopLossPercent) {
			return RuleCheckResult{
				RuleType: models.RiskRuleTypeStopLoss,
				Passed:   false,
				Score:    100,
				Message:  fmt.Sprintf("持仓 %s 触发止损，亏损 %.2f%%", pos.MarketID, lossPercent),
			}
		}
	}

	return RuleCheckResult{
		RuleType: models.RiskRuleTypeStopLoss,
		Passed:   true,
		Score:    0,
		Message:  "未触发止损",
	}
}

// calculateTodayLoss 计算今日亏损（简化实现）
func (a *Auditor) calculateTodayLoss(fundID interface{}) decimal.Decimal {
	// 实际应从数据库查询今日交易盈亏
	return decimal.Zero
}

// serializeResult 序列化审计结果
func (a *Auditor) serializeResult(result *AuditResult) string {
	data, _ := json.Marshal(result)
	return string(data)
}

// formatRejectReason 格式化拒绝原因
func (a *Auditor) formatRejectReason(checks []RuleCheckResult) string {
	for _, check := range checks {
		if !check.Passed {
			return fmt.Sprintf("[%s] %s", check.RuleType, check.Message)
		}
	}
	return "未知原因"
}
