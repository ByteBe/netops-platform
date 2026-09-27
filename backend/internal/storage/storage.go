// Package storage 存储数据库统一入口
// 支持：mysql / oracle / dm(达梦) / kingbase(人大金仓，PG协议) / sqlite(内置)
// 系统初始化时在向导中选择其一；连接失败返回明确错误
package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"netops/internal/config"
)

// Open 打开数据库连接
func Open(cfg *config.Database) (*gorm.DB, error) {
	dl, err := dialector(cfg)
	if err != nil {
		return nil, err
	}
	gcfg := &gorm.Config{}
	if cfg.LogLevel > 0 {
		gcfg.Logger = logger.Default.LogMode(logger.LogLevel(min(cfg.LogLevel, 4)))
	}
	db, err := gorm.Open(dl, gcfg)
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败(%s): %w", cfg.Type, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库探活失败(%s): %w", cfg.Type, err)
	}
	return db, nil
}

// TestConnection 测试连接（初始化向导使用）
func TestConnection(t config.Database) error {
	// MySQL: 先连接无数据库，自动创建数据库
	if strings.ToLower(t.Type) == "mysql" && t.Database != "" {
		adminDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
			t.User, t.Password, t.Host, t.Port)
		adminDB, err := sql.Open("mysql", adminDSN)
		if err == nil {
			_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci", t.Database))
			adminDB.Close()
		}
		if err != nil {
			return fmt.Errorf("自动创建数据库失败: %w", err)
		}
	}
	// Kingbase/PostgreSQL: 自动创建数据库
	if strings.ToLower(t.Type) == "kingbase" || strings.ToLower(t.Type) == "postgres" {
		adminDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
			t.Host, t.Port, t.User, t.Password)
		adminDB, err := sql.Open("postgres", adminDSN)
		if err == nil {
			var exists int
			adminDB.QueryRow("SELECT 1 FROM pg_database WHERE datname=$1", t.Database).Scan(&exists)
			if exists == 0 {
				_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE \"%s\"", t.Database))
			}
			adminDB.Close()
		}
		if err != nil {
			return fmt.Errorf("自动创建数据库失败: %w", err)
		}
	}
	dl, err := dialector(&t)
	if err != nil {
		return err
	}
	db, err := gorm.Open(dl, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("探活失败: %w", err)
	}
	return nil
}

// SupportedTypes 支持的数据库类型
func SupportedTypes() []string {
	return []string{"sqlite", "mysql", "oracle", "dm", "kingbase"}
}

func dialector(cfg *config.Database) (gorm.Dialector, error) {
	switch strings.ToLower(cfg.Type) {
	case "sqlite", "builtin":
		path := cfg.Path
		if path == "" {
			path = "data/netops.db"
		}
		return sqlite.Open(path), nil
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.Params)
		return mysql.Open(dsn), nil
	case "oracle":
		// go-ora DSN: user/pass@host:port/service
		service := cfg.Database
		if service == "" {
			service = "ORCL"
		}
		dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, service)
		return OpenOracle(dsn), nil
	case "dm", "dameng":
		dsn := fmt.Sprintf("dm://%s:%s@%s:%d?schema=%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, firstNonEmpty(cfg.Database, cfg.User))
		return OpenDM(dsn), nil
	case "kingbase", "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable %s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.Params)
		return postgres.Open(dsn), nil
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s（可选：%v）", cfg.Type, SupportedTypes())
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
