package configs

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Ethereum EthereumConfig `mapstructure:"ethereum"`
	AI       AIConfig       `mapstructure:"ai"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"` // 服务器监听端口
	Mode string `mapstructure:"mode"` // 运行模式：debug、release
}

type DatabaseConfig struct {
	DSN                    string `mapstructure:"dsn"`                       // 数据库连接字符串
	MaxOpenConns           int    `mapstructure:"max_open_conns"`            // 最大打开连接数
	MaxIdleConns           int    `mapstructure:"max_idle_conns"`            // 最大空闲连接数
	ConnMaxLifetimeMinutes int    `mapstructure:"conn_max_lifetime_minutes"` // 连接最大生命周期（分钟）
}

type RedisConfig struct {
	Address  string `mapstructure:"address"`  // Redis 服务器地址
	Password string `mapstructure:"password"` // Redis 连接密码
	DB       int    `mapstructure:"db"`       // Redis 数据库编号
}

type EthereumConfig struct {
	RPCURL  string `mapstructure:"rpc_url"`  // Ethereum 节点的 RPC URL
	ChainID int    `mapstructure:"chain_id"` // Ethereum 链 ID
}

type AIConfig struct {
	OpenAIApiKey string `mapstructure:"openai_api_key"` // OpenAI API 密钥
	Model        string `mapstructure:"model"`          // 使用的模型名称
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)   // 指定配置文件
	v.SetConfigType("yaml") // 指定文件类型

	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %s", err))
	}

	var conf Config
	if err := v.Unmarshal(&conf); err != nil {
		panic(fmt.Errorf("解析配置失败: %s", err))
	}

	return &conf, nil
}
