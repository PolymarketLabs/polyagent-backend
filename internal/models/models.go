package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// 交易意图状态
type IntentStatus string

const (
	IntentStatusPending   IntentStatus = "PENDING"   // 待执行
	IntentStatusAuditing  IntentStatus = "AUDITING"  // 风控审计中
	IntentStatusApproved  IntentStatus = "APPROVED"  // 审计通过
	IntentStatusRejected  IntentStatus = "REJECTED"  // 审计拒绝
	IntentStatusExecuting IntentStatus = "EXECUTING" // 执行中
	IntentStatusCompleted IntentStatus = "COMPLETED" // 执行完成
	IntentStatusFailed    IntentStatus = "FAILED"    // 执行失败
	IntentStatusCancelled IntentStatus = "CANCELLED" // 已取消
)

// 交易方向
type TradeSide string

const (
	TradeSideBuy  TradeSide = "BUY"
	TradeSideSell TradeSide = "SELL"
)

// 风控规则类型
type RiskRuleType string

const (
	RiskRuleTypePositionLimit  RiskRuleType = "POSITION_LIMIT"   // 仓位限制
	RiskRuleTypeDailyLossLimit RiskRuleType = "DAILY_LOSS_LIMIT" // 日亏损限制
	RiskRuleTypePriceDeviation RiskRuleType = "PRICE_DEVIATION"  // 价格偏离
	RiskRuleTypeConcentration  RiskRuleType = "CONCENTRATION"    // 集中度限制
	RiskRuleTypeStopLoss       RiskRuleType = "STOP_LOSS"        // 止损线
)

// Fund 基金
type Fund struct {
	ID              uuid.UUID       `gorm:"type:uuid;primary_key" json:"id"`
	Name            string          `gorm:"size:100;not null" json:"name"`
	ManagerID       uuid.UUID       `gorm:"type:uuid;not null" json:"manager_id"`
	TotalAUM        decimal.Decimal `gorm:"type:decimal(20,8)" json:"total_aum"`
	DailyLossLimit  decimal.Decimal `gorm:"type:decimal(20,8)" json:"daily_loss_limit"`
	StopLossPercent decimal.Decimal `gorm:"type:decimal(5,2)" json:"stop_loss_percent"` // 止损百分比
	Status          string          `gorm:"size:20;default:'ACTIVE'" json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// TradeIntent 交易意图
type TradeIntent struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key" json:"id"`
	FundID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"fund_id"`
	ManagerID     uuid.UUID       `gorm:"type:uuid;not null" json:"manager_id"`
	MarketID      string          `gorm:"size:100;not null" json:"market_id"`  // Polymarket市场ID
	OutcomeID     string          `gorm:"size:100;not null" json:"outcome_id"` // 预测结果ID
	Side          TradeSide       `gorm:"size:10;not null" json:"side"`
	Size          decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"size"` // 交易数量
	Price         decimal.Decimal `gorm:"type:decimal(20,8)" json:"price"`         // 目标价格
	OrderType     string          `gorm:"size:20;default:'MARKET'" json:"order_type"`
	Status        IntentStatus    `gorm:"size:20;default:'PENDING'" json:"status"`
	AuditResult   string          `gorm:"type:text" json:"audit_result,omitempty"`
	RejectReason  string          `gorm:"size:500" json:"reject_reason,omitempty"`
	ExecutedTx    string          `gorm:"size:100" json:"executed_tx,omitempty"`
	ExecutedPrice decimal.Decimal `gorm:"type:decimal(20,8)" json:"executed_price"`
	ExecutedAt    *time.Time      `json:"executed_at,omitempty"`
	ExpiresAt     *time.Time      `json:"expires_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Position 持仓
type Position struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key" json:"id"`
	FundID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"fund_id"`
	MarketID      string          `gorm:"size:100;not null" json:"market_id"`
	OutcomeID     string          `gorm:"size:100;not null" json:"outcome_id"`
	Size          decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"size"`
	EntryPrice    decimal.Decimal `gorm:"type:decimal(20,8);not null" json:"entry_price"`
	CurrentPrice  decimal.Decimal `gorm:"type:decimal(20,8)" json:"current_price"`
	UnrealizedPnL decimal.Decimal `gorm:"type:decimal(20,8)" json:"unrealized_pnl"`
	LastUpdated   time.Time       `json:"last_updated"`
	CreatedAt     time.Time       `json:"created_at"`
}

// RiskRule 风控规则
type RiskRule struct {
	ID          uuid.UUID    `gorm:"type:uuid;primary_key" json:"id"`
	FundID      uuid.UUID    `gorm:"type:uuid;not null" json:"fund_id"`
	RuleType    RiskRuleType `gorm:"size:30;not null" json:"rule_type"`
	Params      string       `gorm:"type:jsonb" json:"params"` // JSON格式参数
	IsActive    bool         `gorm:"default:true" json:"is_active"`
	Description string       `gorm:"size:500" json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
}

// RiskEvent 风控事件
type RiskEvent struct {
	ID          uuid.UUID    `gorm:"type:uuid;primary_key" json:"id"`
	FundID      uuid.UUID    `gorm:"type:uuid;not null" json:"fund_id"`
	RuleType    RiskRuleType `gorm:"size:30;not null" json:"rule_type"`
	Severity    string       `gorm:"size:20;not null" json:"severity"` // WARNING, CRITICAL
	MarketID    string       `gorm:"size:100" json:"market_id"`
	Description string       `gorm:"type:text" json:"description"`
	TriggeredAt time.Time    `json:"triggered_at"`
	IsHandled   bool         `gorm:"default:false" json:"is_handled"`
}

// AuditLog 审计日志
type AuditLog struct {
	ID        uuid.UUID    `gorm:"type:uuid;primary_key" json:"id"`
	IntentID  uuid.UUID    `gorm:"type:uuid;not null;index" json:"intent_id"`
	RuleType  RiskRuleType `gorm:"size:30" json:"rule_type"`
	Result    string       `gorm:"size:20;not null" json:"result"` // PASS, FAIL
	Details   string       `gorm:"type:text" json:"details"`
	CheckedAt time.Time    `json:"checked_at"`
}

// MarketData 市场数据缓存表对应结构体
type MarketData struct {
	ID          string          `gorm:"primaryKey;type:varchar(100)" json:"market_id"`
	Question    string          `gorm:"type:varchar(500)" json:"question"`
	Description string          `gorm:"type:text" json:"description"`
	EndDate     time.Time       `gorm:"column:end_date" json:"end_date"`
	Active      bool            `gorm:"default:true" json:"active"`
	Closed      bool            `gorm:"default:false" json:"closed"`
	BestBid     decimal.Decimal `gorm:"type:decimal(20,8)" json:"best_bid"`
	BestAsk     decimal.Decimal `gorm:"type:decimal(20,8)" json:"best_ask"`
	LastPrice   decimal.Decimal `gorm:"type:decimal(20,8)" json:"last_price"`
	Volume      decimal.Decimal `gorm:"type:decimal(20,8)" json:"volume"`
	Liquidity   decimal.Decimal `gorm:"type:decimal(20,8)" json:"liquidity"`
	UpdatedAt   time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedAt   time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

// BeforeCreate GORM钩子
func (f *Fund) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

func (t *TradeIntent) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (p *Position) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
