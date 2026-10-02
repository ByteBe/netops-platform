// Package dbmigrate 数据库迁移：查看/切换当前数据库并自动重启
package dbmigrate

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"netops/internal/appinit"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/modreg"
)

func init() {
	modreg.RegisterProtected("dbmigrate", RegisterProtected)
}

// RegisterProtected 挂载受保护路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/info", func(c *gin.Context) {
		if a.DB == nil {
			response.OK(c, gin.H{"ready": false})
			return
		}
		dialect := a.DB.Dialector.Name()
		sqlDB, _ := a.DB.DB()
		stats := sqlDB.Stats()
		cfg := a.Cfg.Database
		response.OK(c, gin.H{
			"ready":   true,
			"type":    dialect,
			"host":    cfg.Host,
			"port":    cfg.Port,
			"db":      cfg.Database,
			"user":    cfg.User,
			"path":    cfg.Path,
			"open":    stats.OpenConnections,
			"inuse":   stats.InUse,
			"max":     stats.MaxOpenConnections,
			"tsdb":    a.Cfg.TSDB.Type,
			"started": a.Started.Format(time.RFC3339),
		})
	})

	g.GET("/config", func(c *gin.Context) {
		db := a.Cfg.Database
		ts := a.Cfg.TSDB
		response.OK(c, gin.H{
			"database": gin.H{
				"type": db.Type, "host": db.Host, "port": db.Port,
				"user": db.User, "password": db.Password,
				"database": db.Database, "path": db.Path, "params": db.Params,
			},
			"tsdb": gin.H{
				"type": ts.Type, "host": ts.Host, "port": ts.Port,
				"user": ts.User, "pass": ts.Pass, "db": ts.DB, "params": ts.Params,
			},
		})
	})

	g.GET("/tables", func(c *gin.Context) {
		if a.DB == nil {
			response.OK(c, gin.H{"tables": []any{}})
			return
		}
		type tableInfo struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}
		var tables []tableInfo
		for _, m := range appinit.AllModels() {
			stmt := &gorm.Statement{DB: a.DB}
			if err := stmt.Parse(m); err != nil {
				continue
			}
			tbl := stmt.Table
			var cnt int64
			a.DB.Table(tbl).Count(&cnt)
			tables = append(tables, tableInfo{Name: tbl, Count: cnt})
		}
		response.OK(c, gin.H{"tables": tables})
	})

	// POST /run 在当前数据库执行 AutoMigrate
	g.POST("/run", func(c *gin.Context) {
		if a.DB == nil {
			response.Bad(c, "数据库未初始化")
			return
		}
		if err := a.DB.AutoMigrate(appinit.AllModels()...); err != nil {
			response.Err(c, fmt.Errorf("迁移失败: %w", err))
			return
		}
		response.OK(c, gin.H{"ok": true, "msg": fmt.Sprintf("已在当前 %s 库完成 AutoMigrate", a.DB.Dialector.Name())})
	})

	// POST /save 保存数据库配置并重启服务
	g.POST("/save", func(c *gin.Context) {
		var req struct {
			Database struct {
				Type     string `json:"type"`
				Host     string `json:"host"`
				Port     int    `json:"port"`
				User     string `json:"user"`
				Password string `json:"password"`
				Database string `json:"database"`
				Path     string `json:"path"`
				Params   string `json:"params"`
			} `json:"database"`
			TSDB struct {
				Type   string `json:"type"`
				Host   string `json:"host"`
				Port   int    `json:"port"`
				User   string `json:"user"`
				Pass   string `json:"pass"`
				DB     string `json:"db"`
				Params string `json:"params"`
			} `json:"tsdb"`
			Restart bool `json:"restart"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Bad(c, "参数错误")
			return
		}
		a.Cfg.Database.Type = req.Database.Type
		a.Cfg.Database.Host = req.Database.Host
		a.Cfg.Database.Port = req.Database.Port
		a.Cfg.Database.User = req.Database.User
		a.Cfg.Database.Password = req.Database.Password
		a.Cfg.Database.Database = req.Database.Database
		a.Cfg.Database.Path = req.Database.Path
		a.Cfg.Database.Params = req.Database.Params

		a.Cfg.TSDB.Type = req.TSDB.Type
		a.Cfg.TSDB.Host = req.TSDB.Host
		a.Cfg.TSDB.Port = req.TSDB.Port
		a.Cfg.TSDB.User = req.TSDB.User
		a.Cfg.TSDB.Pass = req.TSDB.Pass
		a.Cfg.TSDB.DB = req.TSDB.DB
		a.Cfg.TSDB.Params = req.TSDB.Params

		if err := a.Cfg.Save(); err != nil {
			response.Err(c, fmt.Errorf("保存失败: %w", err))
			return
		}
		response.OK(c, gin.H{"ok": true, "msg": "配置已保存"})

		if req.Restart {
			go func() {
				time.Sleep(1 * time.Second)
				exe, _ := os.Executable()
				var cmd *exec.Cmd
				if runtime.GOOS == "windows" {
					cmd = exec.Command("cmd", "/C", "start", "", exe)
				} else {
					cmd = exec.Command("sh", "-c", "nohup "+exe+" >/dev/null 2>&1 &")
				}
				_ = cmd.Start()
				os.Exit(0)
			}()
		}
	})
}
