// Package repository handles database operations.
package repository

import (
	"context"
	"fmt"
	"polyagent-backend/configs"
	models "polyagent-backend/internal/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
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

// Repository 数据访问接口
type Repository interface {
	// Fund operations
	GetFund(ctx context.Context, id uuid.UUID) (*models.Fund, error)
	GetActiveFunds(ctx context.Context) ([]models.Fund, error)
	UpdateFund(ctx context.Context, fund *models.Fund) error

	// Trade intent operations
	CreateTradeIntent(ctx context.Context, intent *models.TradeIntent) error
	GetTradeIntent(ctx context.Context, id uuid.UUID) (*models.TradeIntent, error)
	GetPendingIntents(ctx context.Context, limit int) ([]models.TradeIntent, error)
	GetStaleApprovedIntents(ctx context.Context, staleTime time.Duration, limit int) ([]models.TradeIntent, error)
	UpdateTradeIntent(ctx context.Context, intent *models.TradeIntent) error

	// Position operations
	GetFundPositions(ctx context.Context, fundID uuid.UUID) ([]models.Position, error)
	GetPosition(ctx context.Context, fundID uuid.UUID, marketID, outcomeID string) (*models.Position, error)
	SavePosition(ctx context.Context, position *models.Position) error
	GetAllPositions(ctx context.Context) ([]models.Position, error)

	// Risk operations
	GetActiveRiskRules(ctx context.Context, fundID uuid.UUID) ([]models.RiskRule, error)
	GetRiskRulesByType(ctx context.Context, fundID uuid.UUID, ruleType models.RiskRuleType) ([]models.RiskRule, error)
	CreateRiskEvent(ctx context.Context, event *models.RiskEvent) error
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error

	// Market operations
	GetActiveMarkets(ctx context.Context) ([]models.MarketData, error)

	// Close database connection
	Close() error
}

// 初始化数据库连接
func InitRepository(conf configs.DatabaseConfig) error {
	db, err := NewPostgresDB(conf)
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

// DBConfig 数据库配置结构
type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewPostgresDB 初始化 PostgreSQL 连接并配置连接池
func NewPostgresDB(cfg configs.DatabaseConfig) (*gorm.DB, error) {
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
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)

	return db, nil
}

func NewPostgresRepository(cfg configs.DatabaseConfig) (Repository, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(
		&models.Fund{},
		&models.TradeIntent{},
		&models.Position{},
		&models.RiskRule{},
		&models.RiskEvent{},
		&models.AuditLog{},
		&models.MarketData{},
	); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return &postgresRepository{db: db}, nil
}

// postgresRepository 实现
type postgresRepository struct {
	db *gorm.DB
}

// TODO
func (p postgresRepository) GetFund(ctx context.Context, id uuid.UUID) (*models.Fund, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetActiveFunds(ctx context.Context) ([]models.Fund, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) UpdateFund(ctx context.Context, fund *models.Fund) error {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) CreateTradeIntent(ctx context.Context, intent *models.TradeIntent) error {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetTradeIntent(ctx context.Context, id uuid.UUID) (*models.TradeIntent, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetPendingIntents(ctx context.Context, limit int) ([]models.TradeIntent, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetStaleApprovedIntents(ctx context.Context, staleTime time.Duration, limit int) ([]models.TradeIntent, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) UpdateTradeIntent(ctx context.Context, intent *models.TradeIntent) error {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetFundPositions(ctx context.Context, fundID uuid.UUID) ([]models.Position, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetPosition(ctx context.Context, fundID uuid.UUID, marketID, outcomeID string) (*models.Position, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) SavePosition(ctx context.Context, position *models.Position) error {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetAllPositions(ctx context.Context) ([]models.Position, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetActiveRiskRules(ctx context.Context, fundID uuid.UUID) ([]models.RiskRule, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetRiskRulesByType(ctx context.Context, fundID uuid.UUID, ruleType models.RiskRuleType) ([]models.RiskRule, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) CreateRiskEvent(ctx context.Context, event *models.RiskEvent) error {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) GetActiveMarkets(ctx context.Context) ([]models.MarketData, error) {
	//TODO implement me
	panic("implement me")
}

func (p postgresRepository) Close() error {
	//TODO implement me
	panic("implement me")
}
