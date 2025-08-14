package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Cfg 是一个全局变量，用于存储所有应用程序的配置
var Cfg *Config

// Config 结构体定义了应用程序的所有配置项
// 它与 config.yaml 文件的结构完全对应
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
}

// ServerConfig 定义了服务器相关的配置
type ServerConfig struct {
	Mode    string     `mapstructure:"mode"`
	Address string     `mapstructure:"address"`
	Cors    CorsConfig `mapstructure:"cors"`
}

// CorsConfig 定义了CORS相关的配置
type CorsConfig struct {
	AllowedOrigins []string `mapstructure:"allowedOrigins"`
}

// DatabaseConfig 定义了数据库和缓存相关的配置
type DatabaseConfig struct {
	Redis  RedisConfig  `mapstructure:"redis"`
	Sqlite SqliteConfig `mapstructure:"sqlite"`
}

// RedisConfig 定义了Redis的配置
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// SqliteConfig 定义了内存缓存的配置
type SqliteConfig struct {
	MaxCacheSizeKB int64 `mapstructure:"maxCacheSizeKB"`
}

// LoadConfig 函数负责查找、加载和解析配置文件
// 它会在指定的路径中查找名为 config.yaml 的文件
func LoadConfig() (*Config, error) {
	v := viper.New()

	// 1. 设置配置文件名和类型
	v.SetConfigName("config") // 文件名 (不带扩展名)
	v.SetConfigType("yaml")   // 文件类型

	// 2. 添加配置文件搜索路径
	// 可以添加多个路径，Viper会按顺序查找
	v.AddConfigPath("./config") // `config/config.yaml`
	v.AddConfigPath(".")        // `./config.yaml` (如果在根目录)

	// 3. 设置环境变量支持 (可选，但推荐)
	// 允许通过环境变量覆盖配置，例如 SERVER_PORT=8888
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 4. 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	// 5. 将配置反序列化到结构体中
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// 6. 将加载的配置赋值给全局变量
	Cfg = &cfg

	return Cfg, nil
}
