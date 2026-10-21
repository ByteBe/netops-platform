// Package devicelog 设备日志：syslog UDP接收、Docker日志拉取、查询、清理
package devicelog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/middleware"
	"netops/internal/model"
	"netops/internal/modreg"
)

func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/list", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
		if page < 1 {
			page = 1
		}
		if size < 1 || size > 500 {
			size = 50
		}
		q := a.DB.Model(&model.DeviceLog{})
		if v := c.Query("source"); v != "" {
			q = q.Where("source = ?", v)
		}
		if v := c.Query("level"); v != "" {
			q = q.Where("level = ?", v)
		}
		if v := c.Query("device"); v != "" {
			q = q.Where("device_name LIKE ? OR ip LIKE ?", "%"+v+"%", "%"+v+"%")
		}
		if v := c.Query("keyword"); v != "" {
			q = q.Where("message LIKE ?", "%"+v+"%")
		}
		if v := c.Query("from"); v != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
				q = q.Where("created_at >= ?", t)
			}
		}
		if v := c.Query("to"); v != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
				q = q.Where("created_at <= ?", t)
			}
		}
		var total int64
		q.Count(&total)
		var list []model.DeviceLog
		q.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&list)
		response.OK(c, gin.H{"list": list, "total": total, "page": page, "size": size})
	})

	g.DELETE("/:id", middleware.RequireRole("admin"), func(c *gin.Context) {
		a.DB.Delete(&model.DeviceLog{}, c.Param("id"))
		response.OK(c, gin.H{"ok": true})
	})

	g.POST("/clear", middleware.RequireRole("admin"), func(c *gin.Context) {
		a.DB.Where("1 = 1").Delete(&model.DeviceLog{})
		response.OK(c, gin.H{"ok": true})
	})

	g.GET("/sources", func(c *gin.Context) {
		// 来源候选 = 固定核心（采集器始终在跑的 system/syslog/docker/mysql）+ 当前确有数据的来源。
		// 未纳管/无数据的服务（MongoDB/Redis 等）不再罗列，避免误导筛选。
		labels := map[string]string{
			"system": "系统管理", "syslog": "Syslog", "docker": "Docker", "k8s": "K8s", "nginx": "Nginx",
			"switch": "交换机", "router": "路由器", "server": "服务器",
			"mysql": "MySQL", "postgres": "PostgreSQL", "mongodb": "MongoDB",
			"redis": "Redis", "tdengine": "TDengine", "influxdb": "InfluxDB", "prometheus": "Prometheus", "elasticsearch": "Elasticsearch",
		}
		// 固定核心仅保留平台自身来源（system/syslog 一定有采集器在跑）；
		// 其余来源（docker/mysql/MongoDB 等）只在实际有数据时才出现，未纳管的服务不误导筛选
		core := []string{"system", "syslog"}
		var active []string
		a.DB.Model(&model.DeviceLog{}).Distinct().Order("source").Pluck("source", &active)
		seen := map[string]bool{}
		out := []gin.H{}
		for _, s := range append(core, active...) {
			if seen[s] || labels[s] == "" {
				continue
			}
			seen[s] = true
			out = append(out, gin.H{"label": labels[s], "value": s})
		}
		response.OK(c, out)
	})
}

// StartSyslogReceiver UDP syslog 接收
func StartSyslogReceiver(app *core.App) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 514})
	if err != nil {
		conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: 1514})
		if err != nil {
			fmt.Println("[devicelog] syslog listen failed:", err)
		} else {
			fmt.Println("[devicelog] syslog UDP listening on :1514")
		}
	} else {
		fmt.Println("[devicelog] syslog UDP listening on :514")
	}
	if conn != nil {
		go func() {
			buf := make([]byte, 65535)
			for {
				n, raddr, err := conn.ReadFromUDP(buf)
				if err != nil {
					time.Sleep(200 * time.Millisecond)
					continue
				}
				if app.DB == nil {
					continue
				}
				// 一个 UDP 包可能包含多条 syslog（\n 分隔），逐条解析入库
				for _, msg := range strings.Split(string(buf[:n]), "\n") {
					msg = strings.TrimSpace(msg)
					if msg == "" {
						continue
					}
					level, facility, content := parseSyslog(msg)
					src := "syslog"
					ip := raddr.IP.String()
					devName := ip
					var d model.MonitorDevice
					if app.DB != nil && app.DB.Where("ip = ?", ip).First(&d).Error == nil {
						devName = d.Name
						switch d.Type {
						case "switch", "router", "server", "firewall", "windows", "linux":
							src = d.Type
						}
					}
					app.DB.Create(&model.DeviceLog{
						Source: src, IP: ip, DeviceName: devName,
						Level: level, Facility: facility, Message: content, CreatedAt: time.Now(),
					})
				}
			}
		}()
	}
	// Docker 日志采集 + MySQL 日志采集 + 系统管理审计日志采集 + 清理
	go dockerLogCollector(app)
	go mysqlLogCollector(app)
	go systemLogCollector(app)
	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for range t.C {
			if app.DB != nil {
				app.DB.Where("created_at < ?", time.Now().AddDate(0, 0, -7)).Delete(&model.DeviceLog{})
			}
		}
	}()
}

// systemLogCollector 每 30 秒增量同步系统管理审计日志(audit_logs)到日志中心(device_logs, source=system)
// 用户可在此统一查看系统管理操作记录（登录/配置/用户/证书/升级等），无需再切换审计页
func systemLogCollector(app *core.App) {
	lastID := uint(0)
	sync := func() {
		if app.DB == nil {
			return
		}
		var rows []model.AuditLog
		app.DB.Where("id > ?", lastID).Order("id asc").Limit(500).Find(&rows)
		for _, r := range rows {
			msg := r.Action
			if r.Detail != "" {
				msg = r.Action + " | " + r.Detail
			}
			app.DB.Create(&model.DeviceLog{
				Source: "system", IP: r.IP, DeviceName: r.Username,
				Level: "info", Facility: "system", Message: msg, CreatedAt: r.CreatedAt,
			})
			lastID = r.ID
		}
	}
	sync()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		sync()
	}
}

// dockerLogCollector 每 60 秒增量拉取各 Docker 主机日志：
//   - 本机 docker（/var/run/docker.sock）无条件采集；
//   - 远程主机按 DockerHost 表 enabled=true 的记录采集；
//   - 首次拉取最近 50 行，之后按 since 时间戳增量拉取，保证低频日志也能入库
func dockerLogCollector(app *core.App) {
	// 增量游标：containerID -> 上次拉取完成时刻（秒级 since）
	lastFetch := map[string]time.Time{}
	// 轻量去重兜底（同一秒内 since 边界可能重复），1 小时清理
	seen := map[string]time.Time{}
	// 本地 docker socket client
	localClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.DialTimeout("unix", "/var/run/docker.sock", 5*time.Second)
			},
		},
	}
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	collect := func() {
		if app.DB == nil {
			return
		}
		type hostItem struct{ addr, name string }
		hosts := []hostItem{}
		// 1. 本机 docker 无条件采集（平台自身常驻 docker）
		if isLocalDocker() {
			hosts = append(hosts, hostItem{addr: "", name: "本机"})
		}
		// 2. 远程 DockerHost：仅采集启用项
		var rows []model.DockerHost
		app.DB.Where("enabled = ?", true).Find(&rows)
		for _, h := range rows {
			hosts = append(hosts, hostItem{addr: h.Address, name: h.Name})
		}
		seenHost := map[string]bool{}
		for _, h := range hosts {
			// 本机 docker（addr=""）必须采集；仅对远程主机做去重
			if h.addr != "" {
				if seenHost[h.addr] {
					continue
				}
				seenHost[h.addr] = true
			}
			var client *http.Client
			base := "http://localhost"
			if h.addr == "" {
				client = localClient
			} else {
				base = strings.TrimRight(h.addr, "/")
				client = &http.Client{Timeout: 10 * time.Second}
			}
			resp, err := client.Get(base + "/containers/json?all=1")
			if err != nil {
				continue
			}
			var containers []struct {
				ID   string   `json:"Id"`
				Name []string `json:"Names"`
			}
			json.NewDecoder(resp.Body).Decode(&containers)
			resp.Body.Close()
			for _, ct := range containers {
				ctID := ct.ID[:12]
				name := ctID
				if len(ct.Name) > 0 {
					name = strings.TrimPrefix(ct.Name[0], "/")
				}
				last, has := lastFetch[ctID]
				var url string
				if !has {
					// 首次：最近 50 行
					url = fmt.Sprintf("%s/containers/%s/logs?stdout=1&stderr=1&tail=50", base, ctID)
				} else {
					// 增量：since=上次拉取时刻（Unix 秒）
					url = fmt.Sprintf("%s/containers/%s/logs?stdout=1&stderr=1&since=%d", base, ctID, last.Unix())
				}
				lr, err := client.Get(url)
				if err != nil {
					continue
				}
				body, _ := io.ReadAll(lr.Body)
				lr.Body.Close()
				lines := splitDockerLog(body)
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					key := ctID + ":" + line
					if ts, ok := seen[key]; ok && time.Since(ts) < time.Hour {
						continue
					}
					seen[key] = time.Now()
					ip := h.addr
					if ip == "" {
						ip = "127.0.0.1"
					}
					// 容器日志统一归 source=docker（设备名=容器名），
					// 服务类来源（mysql/nginx/tdengine 等）由 syslog 等采集提供，避免 Docker 来源永远查不到数据
					src := "docker"
					app.DB.Create(&model.DeviceLog{
						Source: src, IP: ip, DeviceName: name,
						Level: guessLevel(line), Facility: "daemon",
						Message: line, CreatedAt: time.Now(),
					})
				}
				lastFetch[ctID] = time.Now()
			}
		}
		for k, v := range seen {
			if time.Since(v) > time.Hour {
				delete(seen, k)
			}
		}
	}
	collect()
	for range t.C {
		collect()
	}
}

// isLocalDocker 判断本机是否存在 docker unix socket
func isLocalDocker() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

// splitDockerLog 解析 docker logs 的多路复用帧流：每帧 8 字节头(stream+3pad+4BE size)
func splitDockerLog(b []byte) []string {
	var out []string
	for len(b) >= 8 {
		size := int(b[4])<<24 | int(b[5])<<16 | int(b[6])<<8 | int(b[7])
		if size < 0 || 8+size > len(b) {
			break
		}
		payload := b[8 : 8+size]
		out = append(out, strings.Split(string(payload), "\n")...)
		b = b[8+size:]
	}
	if len(out) == 0 {
		out = strings.Split(string(b), "\n")
	}
	return out
}

func guessLevel(msg string) string {
	low := strings.ToLower(msg)
	if strings.Contains(low, "error") || strings.Contains(low, "fail") || strings.Contains(low, "fatal") {
		return "critical"
	}
	if strings.Contains(low, "warn") {
		return "warning"
	}
	return "info"
}

func parseSyslog(raw string) (level, facility, content string) {
	level, facility = "info", "user"
	if strings.HasPrefix(raw, "<") {
		end := strings.Index(raw, ">")
		if end > 0 {
			if pri, err := strconv.Atoi(raw[1:end]); err == nil {
				facility = facilityName(pri / 8)
				level = levelName(pri % 8)
			}
			raw = raw[end+1:]
		}
	}
	content = strings.TrimSpace(raw)
	return
}

func levelName(s int) string {
	switch s {
	case 0, 1, 2, 3:
		return "critical"
	case 4:
		return "warning"
	case 5, 6:
		return "info"
	default:
		return "debug"
	}
}

func facilityName(f int) string {
	if f >= 16 && f <= 23 {
		return fmt.Sprintf("local%d", f-16)
	}
	names := map[int]string{0: "kern", 1: "user", 2: "mail", 3: "daemon", 4: "auth", 5: "syslog", 6: "lpr", 7: "news"}
	if n, ok := names[f]; ok {
		return n
	}
	return "user"
}

// mysqlLogCollector 每 5 分钟查询 performance_schema.error_log（MySQL 8.0+）
// 先检测表是否存在/可访问：5.7 无此表或连接用户无权限时打印提示并跳过，不再静默无数据
func mysqlLogCollector(app *core.App) {
	seen := map[string]time.Time{}
	hasErrLog := func() bool {
		if app.DB == nil {
			return false
		}
		var n int64
		err := app.DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='performance_schema' AND table_name='error_log'").Scan(&n).Error
		if err != nil || n == 0 {
			fmt.Println("[devicelog] MySQL error_log 不可用（MySQL 8.0+ 且需 performance_schema 权限）:", err)
			return false
		}
		return true
	}
	collect := func() {
		if app.DB == nil {
			return
		}
		if !hasErrLog() {
			return
		}
		type errLogRow struct {
			Logged    time.Time
			Code      string
			Subsystem string
			Message   string
		}
		var rows []errLogRow
		// MySQL 8.0 performance_schema.error_log 列：LOGGED, ERROR_CODE, SUBSYSTEM, DATA（无 level 列）
		if err := app.DB.Raw("SELECT logged, error_code AS code, subsystem, data AS message FROM performance_schema.error_log ORDER BY logged DESC LIMIT 50").Scan(&rows).Error; err != nil {
			fmt.Println("[devicelog] MySQL error_log 查询失败:", err)
			return
		}
		for _, r := range rows {
			key := r.Code + ":" + r.Message
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = time.Now()
			lvl := "info"
			msg := strings.ToLower(r.Message)
			switch {
			case strings.Contains(msg, "warning") || strings.Contains(msg, "warn"):
				lvl = "warning"
			case strings.Contains(msg, "error") || strings.Contains(msg, "critical"):
				lvl = "critical"
			}
			app.DB.Create(&model.DeviceLog{
				Source: "mysql", IP: "127.0.0.1", DeviceName: "mysql",
				Level: lvl, Facility: r.Subsystem, Message: r.Message, CreatedAt: r.Logged,
			})
		}
		for k, v := range seen {
			if time.Since(v) > time.Hour {
				delete(seen, k)
			}
		}
	}
	collect()
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		collect()
	}
}

func init() { modreg.RegisterProtected("devicelog", RegisterProtected) }
