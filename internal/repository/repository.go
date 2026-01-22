// Package repository handles database operations.
package repository

import (
	"polyagent-backend/configs"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	RoleInvestor = iota
	RoleManager
)

type User struct {
	gorm.Model
	Username   string `gorm:"uniqueIndex;not null"`
	Email      string `gorm:"uniqueIndex"`
	Role       int    `gorm:"not null"` // e.g., RoleManager, RoleInvestor
	Address    string `gorm:"not null"` // e.g., "0x123abc..."
	IsVerified bool   `gorm:"not null"` // 经理审核状态
	KYCStatus  string // 可选，用于合规性扩展
}

// 基金详情 (Funds)
type Fund struct {
	gorm.Model
	VaultAddress     string  `gorm:"uniqueIndex;not null"` // 链上 Vault 合约地址
	ExecutionAddress string  `gorm:"uniqueIndex;not null"` // 对应的 Polymarket 执行 EOA 地址
	ManagerID        uint    `gorm:"not null"`             // 关联 Users.id
	StrategyConfig   string  `gorm:"type:jsonb;not null"`  // JSON (包含允许交易的市场类别、最大滑点、止损线)
	CurrentNAV       float64 // 最新结算净值
	AUMTotal         float64 // 资产管理总规模 (Vault + Exec Wallet + Position)
}

// 交易意图 (Intents)
type Intent struct {
	gorm.Model
	FundID    uint   `gorm:"not null"`                 // 关联 Funds.id
	MarketID  string `gorm:"not null"`                 // Polymarket 市场 ID
	Side      string `gorm:"not null"`                 // BUY / SELL
	OrderData string `gorm:"type:jsonb;not null"`      // JSON (价格、数量、订单类型)
	Status    string `gorm:"not null"`                 // PENDING, VALIDATING, EXECUTING, SUCCESS, FAILED
	TxHash    string `gorm:"uniqueIndex;default:null"` // Polymarket 成交后的交易哈希
}

// 初始化数据库连接

func InitRepository(conf *configs.DatabaseConfig) error {
	dbCfg := DBConfig{
		DSN:             conf.DSN,
		MaxOpenConns:    conf.MaxOpenConns,
		MaxIdleConns:    conf.MaxIdleConns,
		ConnMaxLifetime: time.Duration(conf.ConnMaxLifetimeMinutes) * time.Minute,
	}

	db, err := NewPostgresDB(dbCfg)
	if err != nil {
		return err
	}

	// 自动迁移数据库结构
	err = db.AutoMigrate(&User{}, &Fund{}, &Intent{})
	if err != nil {
		return err
	}
	return nil
}

// --- 1. 数据库基础配置与连接池 ---

// DBConfig 数据库配置结构
type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewPostgresDB 初始化 PostgreSQL 连接并配置连接池
func NewPostgresDB(cfg DBConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		PrepareStmt: true, // 开启预编译语句，提高重复执行 SQL 的性能
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 配置连接池，防止高并发时数据库过载
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db, nil
}
