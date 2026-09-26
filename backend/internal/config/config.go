// Package config 平台配置加载与保存
// 配置来源优先级：命令行 --config > 环境变量 NETOPS_CONFIG > 工作目录 config.yaml > 可执行文件同目录 config.yaml
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Server 服务配置
type Server struct {
	Host         string `yaml:"host"`          // 外部 Web 监听地址
	Port         int    `yaml:"port"`          // 外部 Web 端口（默认 30821）
	InternalHost string `yaml:"internal_host"` // 内部服务监听地址（默认 127.0.0.1）
	InternalPort int    `yaml:"internal_port"` // 内部服务端口（默认 30001，数据库通讯）
}

// Database 存储数据库配置（系统初始化时选择：mysql/oracle/dm/kingbase/sqlite(内置)）
type Database struct {
	Type     string `yaml:"type"`     // mysql | oracle | dm | kingbase | sqlite
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Params   string `yaml:"params"` // 附加连接参数
	MaxIdle  int    `yaml:"max_idle_conns"`
	MaxOpen  int    `yaml:"max_open_conns"`
	LogLevel int    `yaml:"log_level"` // 0=Silent 1=Error 2=Warn 3=Info
}

// TSDB 时序数据库配置（缓存/指标，系统初始化时选择：tdengine/influxdb/builtin）
type TSDB struct {
	Type   string `yaml:"type"` // tdengine | influxdb | builtin(内置)
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	User   string `yaml:"user"`
	Pass   string `yaml:"pass"`
	DB     string `yaml:"db"`
	Params string `yaml:"params"` // 附加参数（如 influxdb org/bucket/token 或 tdengine http 路径）
}

// Crypto 国密配置
type Crypto struct {
	SM2PrivateKey string `yaml:"sm2_private_key"` // 16进制
	SM2PublicKey  string `yaml:"sm2_public_key"`  // 16进制（不配置则启动时自动生成）
}

// Config 平台配置
type Config struct {
	App         App      `yaml:"app"`
	Server      Server   `yaml:"server"`
	Database    Database `yaml:"database"`
	TSDB        TSDB     `yaml:"tsdb"`
	Crypto      Crypto   `yaml:"crypto"`
	Initialized bool     `yaml:"initialized"` // 是否已完成初始化向导（选择存储库/时序库/创建管理员）
}

// App 应用级配置
type App struct {
	Name   string `yaml:"name"`
	JWTKey string `yaml:"jwt_secret"` // 不配置则启动时随机生成（重启后会话失效）
}

func defaultConfig() *Config {
	return &Config{
		App: App{Name: "NetOps"},
		Server: Server{
			Host:         "0.0.0.0",
			Port:         30821,
			InternalHost: "127.0.0.1",
			InternalPort: 30001,
		},
		Database: Database{Type: "sqlite", Port: 0, MaxIdle: 10, MaxOpen: 100, LogLevel: 1},
		TSDB:     TSDB{Type: "builtin"},
	}
}

// Load 加载配置
func Load() (*Config, error) {
	path := resolvePath()
	cfg := defaultConfig()
	if path == "" {
		// 无配置文件：首次运行，等待初始化向导（setup API 会写入配置）
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}
	return cfg, nil
}

// Save 保存配置（初始化向导调用）
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	path := resolvePath()
	if path == "" {
		path = "config.yaml"
	}
	return os.WriteFile(path, data, 0o600)
}

// ConfigPath 当前配置文件路径
func (c *Config) ConfigPath() string {
	return resolvePath()
}

func resolvePath() string {
	if p := flag.Lookup("config"); p != nil && p.Value.String() != "" {
		return p.Value.String()
	}
	if p := os.Getenv("NETOPS_CONFIG"); p != "" {
		return p
	}
	for _, p := range []string{"config.yaml", "config.yml"} {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	// 可执行文件同目录
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, p := range []string{"config.yaml", "config.yml"} {
			fp := filepath.Join(dir, p)
			if _, err := os.Stat(fp); err == nil {
				return fp
			}
		}
	}
	return ""
}
