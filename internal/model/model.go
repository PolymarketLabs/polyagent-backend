package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 对应设计文档 2.1：用户与角色
type User struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	Address    string         `gorm:"type:varchar(42);uniqueIndex;not null" json:"address"` // 钱包地址
	Role       string         `gorm:"type:varchar(20);default:'INVESTOR'" json:"role"`      // INVESTOR, MANAGER
	Bio        string         `gorm:"type:text" json:"bio"`                                 // 个人简介
	IsVerified bool           `gorm:"default:false" json:"is_verified"`                     // 经理审核状态
	KYCStatus  string         `gorm:"type:varchar(20);default:'NONE'" json:"kyc_status"`    // KYC 状态
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Funds []Fund `gorm:"foreignKey:ManagerID" json:"managed_funds,omitempty"`
}

// Fund 对应设计文档 2.2：基金详情
type Fund struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	Name             string `gorm:"type:varchar(100);not null" json:"name"`
	Description      string `gorm:"type:text" json:"description"`
	VaultAddress     string `gorm:"type:varchar(42);uniqueIndex;not null" json:"vault_address"`     // 链上 Vault 地址
	ExecutionAddress string `gorm:"type:varchar(42);uniqueIndex;not null" json:"execution_address"` // 执行 EOA 地址
	ManagerID        uint   `json:"manager_id"`
	Manager          User   `gorm:"foreignKey:ManagerID" json:"manager"`

	// 策略配置 (JSON)
	StrategyConfig string `gorm:"type:jsonb" json:"strategy_config"` // 包含白名单、滑点、止损等

	// 财务状态
	CurrentNAV float64 `gorm:"type:numeric(20,8);default:1.0" json:"current_nav"`
	AUMTotal   float64 `gorm:"type:numeric(20,8);default:0.0" json:"aum_total"`
	Status     string  `gorm:"type:varchar(20);default:'PREPARING'" json:"status"` // PREPARING, ACTIVE, CLOSED

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Intent 对应设计文档 2.3：交易意图
type Intent struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	FundID    uint      `gorm:"index" json:"fund_id"`
	MarketID  string    `gorm:"type:varchar(255);not null" json:"market_id"` // Polymarket 市场 ID
	Side      string    `gorm:"type:varchar(10);not null" json:"side"`       // BUY, SELL
	OrderData string    `gorm:"type:jsonb" json:"order_data"`                // 价格、数量等原始数据

	// 状态流转
	Status   string `gorm:"type:varchar(20);default:'PENDING'" json:"status"` // PENDING, VALIDATING, EXECUTING, SUCCESS, FAILED
	TxHash   string `gorm:"type:varchar(66)" json:"tx_hash"`                  // Polymarket 交易哈希
	ErrorLog string `gorm:"type:text" json:"error_log"`                       // 错误信息

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NavHistory 用于展示净值走势图
type NavHistory struct {
	ID          uint      `gorm:"primaryKey"`
	FundID      uint      `gorm:"index"`
	NavPerShare float64   `gorm:"type:numeric(20,8)"`
	TotalAUM    float64   `gorm:"type:numeric(20,8)"`
	RecordedAt  time.Time `gorm:"index"` // 记录时间
}

// Transaction 记录投资人的申赎行为（聚合链上事件）
type Transaction struct {
	ID        uint    `gorm:"primaryKey"`
	UserID    uint    `gorm:"index"`
	FundID    uint    `gorm:"index"`
	Type      string  `gorm:"type:varchar(20)"` // DEPOSIT, REDEEM
	Amount    float64 `gorm:"type:numeric(20,8)"`
	Shares    float64 `gorm:"type:numeric(20,8)"`
	TxHash    string  `gorm:"type:varchar(66);uniqueIndex"`
	Status    string  `gorm:"type:varchar(20)"` // CONFIRMED, FAILED
	CreatedAt time.Time
}
