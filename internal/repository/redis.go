package repository

import (
	"context"
	"polyagent-backend/configs"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig 定义 Redis 连接池配置
type RedisConfig struct {
	Addr         string // 地址 "localhost:6379"
	Password     string // 密码
	DB           int    // 数据库索引
	PoolSize     int    // 连接池最大连接数
	MinIdleConns int    // 最小空闲连接数 (保持热连接)
}

// RedisRepository 封装 Redis 操作接口
type RedisRepository interface {
	SetNonce(ctx context.Context, address string, nonce string, expiration time.Duration) error
	GetNonce(ctx context.Context, address string) (string, error)
	DeleteNonce(ctx context.Context, address string) error
	Close() error
}

type redisRepo struct {
	client *redis.Client
}

// NewRedisRepository 初始化 Redis 客户端及连接池
func NewRedisRepository(cfg configs.RedisConfig) (RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,

		// --- 连接池核心配置 ---
		PoolSize:     cfg.PoolSize,     // 一般设置为 CPU 核心数的 10-20 倍
		MinIdleConns: cfg.MinIdleConns, // 即使没有请求也保持的连接数
		PoolTimeout:  30 * time.Second, // 当连接池满时，等待连接的超时时间
	})

	// 测试连接是否连通
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, err
	}

	return &redisRepo{client: client}, nil
}

// SetNonce 存储登录 Nonce
func (r *redisRepo) SetNonce(ctx context.Context, address string, nonce string, expiration time.Duration) error {
	key := "nonce:" + address
	return r.client.Set(ctx, key, nonce, expiration).Err()
}

// GetNonce 获取并校验 Nonce
func (r *redisRepo) GetNonce(ctx context.Context, address string) (string, error) {
	key := "nonce:" + address
	return r.client.Get(ctx, key).Result()
}

// DeleteNonce 验签成功后立即作废 Nonce (防止重放攻击)
func (r *redisRepo) DeleteNonce(ctx context.Context, address string) error {
	key := "nonce:" + address
	return r.client.Del(ctx, key).Err()
}

// Close 关闭连接池
func (r *redisRepo) Close() error {
	return r.client.Close()
}
