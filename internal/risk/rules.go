package risk

import (
	"encoding/json"
	"fmt"
	"polyagent-backend/internal/models"

	"github.com/shopspring/decimal"
)

// RuleParams 规则参数接口
type RuleParams interface {
	Validate() error
}

// PositionLimitParams 仓位限制参数
type PositionLimitParams struct {
	MaxPositionSize   decimal.Decimal `json:"max_position_size"`   // 单个市场最大仓位
	MaxTotalExposure  decimal.Decimal `json:"max_total_exposure"`  // 总敞口上限
	MaxSinglePosition decimal.Decimal `json:"max_single_position"` // 单笔交易上限
}

func (p PositionLimitParams) Validate() error {
	if p.MaxPositionSize.IsZero() || p.MaxTotalExposure.IsZero() {
		return fmt.Errorf("position limits must be positive")
	}
	return nil
}

// DailyLossLimitParams 日亏损限制参数
type DailyLossLimitParams struct {
	MaxDailyLoss decimal.Decimal `json:"max_daily_loss"` // 日最大亏损
}

func (p DailyLossLimitParams) Validate() error {
	if p.MaxDailyLoss.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("max_daily_loss must be positive")
	}
	return nil
}

// PriceDeviationParams 价格偏离参数
type PriceDeviationParams struct {
	MaxDeviationPercent decimal.Decimal `json:"max_deviation_percent"` // 最大偏离百分比
}

// Validate 实现 RuleParams 接口
func (p PriceDeviationParams) Validate() error {
	// 最大偏离百分比必须大于 0 且不超过 100
	if p.MaxDeviationPercent.LessThanOrEqual(decimal.Zero) ||
		p.MaxDeviationPercent.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("max_deviation_percent must be in (0, 100]")
	}
	return nil
}

// ConcentrationParams 集中度参数
type ConcentrationParams struct {
	MaxConcentrationPercent decimal.Decimal `json:"max_concentration_percent"` // 单市场最大集中度
}

func (p ConcentrationParams) Validate() error {
	if p.MaxConcentrationPercent.LessThanOrEqual(decimal.Zero) ||
		p.MaxConcentrationPercent.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("max_concentration_percent must be in (0, 100]")
	}
	return nil
}

// StopLossParams 止损参数
type StopLossParams struct {
	StopLossPercent decimal.Decimal `json:"stop_loss_percent"` // 止损百分比
}

// 实现 RuleParams 接口
func (p StopLossParams) Validate() error {
	// 简单校验：止损比例必须 > 0 且 <= 100
	if p.StopLossPercent.LessThanOrEqual(decimal.Zero) ||
		p.StopLossPercent.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("stop_loss_percent must be in (0, 100]")
	}
	return nil
}

// ParseRuleParams 解析规则参数
func ParseRuleParams(ruleType models.RiskRuleType, data string) (RuleParams, error) {
	switch ruleType {
	case models.RiskRuleTypePositionLimit:
		var params PositionLimitParams
		if err := json.Unmarshal([]byte(data), &params); err != nil {
			return nil, err
		}
		return params, params.Validate()
	case models.RiskRuleTypeDailyLossLimit:
		var params DailyLossLimitParams
		err := json.Unmarshal([]byte(data), &params)
		return params, err
	case models.RiskRuleTypePriceDeviation:
		var params PriceDeviationParams
		err := json.Unmarshal([]byte(data), &params)
		return params, err
	case models.RiskRuleTypeConcentration:
		var params ConcentrationParams
		err := json.Unmarshal([]byte(data), &params)
		return params, err
	case models.RiskRuleTypeStopLoss:
		var params StopLossParams
		err := json.Unmarshal([]byte(data), &params)
		return params, err
	default:
		return nil, fmt.Errorf("unknown rule type: %s", ruleType)
	}
}
