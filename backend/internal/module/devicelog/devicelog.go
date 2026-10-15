// Package devicelog 设备日志：syslog UDP接收、Docker日志拉取、查询、清理
package devicelog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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
		if page < 1 { page = 1 }
		if size < 1 || size > 500 { size = 50 }
		q := a.DB.Model(&model.DeviceLog{})
		if v := c.Query("source"); v != "" { q = q.Where("source = ?", v) }
		if v := c.Query("level"); v != "" { q = q.Where("level = ?", v) }
		if v := c.Query("device"); v != "" {
			q = q.Where("device_name LIKE ? OR ip LIKE ?", "%"+v+"%", "%"+v+"%")
		}
		if v := c.Query("keyword"); v != "" { q = q.Where("message LIKE ?", "%"+v+"%") }
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
		var sources []string
		a.DB.Model(&model.DeviceLog{}).Distinct("source").Pluck("source", &sources)
		labels := map[string]string{
			"syslog":"Syslog","docker":"Docker","k8s":"K8s","switch":"交换机","router":"路由器",
			"server":"服务器","mysql":"MySQL","postgres":"PostgreSQL","mongodb":"MongoDB",
			"redis":"Redis","tdengine":"TDengine","influxdb":"InfluxDB","prometheus":"Prometheus","elasticsearch":"Elasticsearch",
		}
		out := []gin.H{}
		for _, s := range sources {
			label := labels[s]
			if label == "" { label = s }
			out = append(out, gin.H{"label": label, "value": s})
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
			if err != nil { time.Sleep(200 * time.Millisecond); continue }
			if app.DB == nil { continue }
			msg := strings.TrimSpace(string(buf[:n]))
				level, facility, content := parseSyslog(msg)
				src := "syslog"
				ip := raddr.IP.String()
				devName := ip
				var d model.MonitorDevice
				if app.DB != nil && app.DB.Where("ip = ?", ip).First(&d).Error == nil {
					devName = d.Name
					if d.Type == "switch" || d.Type == "router" || d.Type == "server" {
						src = d.Type
					}
				}
				app.DB.Create(&model.DeviceLog{
					Source: src, IP: ip, DeviceName: devName,
					Level: level, Facility: facility, Message: content, CreatedAt: time.Now(),
				})
			}
		}()
	}
	// Docker 日志采集 + MySQL 日志采集 + 清理
	go dockerLogCollector(app)
	go mysqlLogCollector(app)
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

// dockerLogCollector 每 5 分钟拉取各 Docker 主机最近日志
func dockerLogCollector(app *core.App) {
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
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	collect := func() {
		if app.DB == nil { return }
		var hosts []model.DockerHost
		app.DB.Find(&hosts)
		for _, h := range hosts {
			var client *http.Client
			base := "http://localhost"
			if h.Address == "" {
				// 本地 unix socket
				client = localClient
			} else {
				base = strings.TrimRight(h.Address, "/")
				client = &http.Client{Timeout: 10 * time.Second}
			}
			resp, err := client.Get(base + "/containers/json?all=1")
			if err != nil { continue }
			var containers []struct {
				ID   string   `json:"Id"`
				Name []string `json:"Names"`
			}
			json.NewDecoder(resp.Body).Decode(&containers)
			resp.Body.Close()
			for _, ct := range containers {
				name := ct.ID[:12]
				if len(ct.Name) > 0 { name = strings.TrimPrefix(ct.Name[0], "/") }
				lr, err := client.Get(fmt.Sprintf("%s/containers/%s/logs?stdout=1&stderr=1&tail=10", base, ct.ID[:12]))
				if err != nil { continue }
				body, _ := io.ReadAll(lr.Body)
				lr.Body.Close()
				lines := splitDockerLog(body)
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" { continue }
					key := ct.ID[:12] + ":" + line
					if _, ok := seen[key]; ok { continue }
					seen[key] = time.Now()
					ip := h.Address
					if ip == "" { ip = "127.0.0.1" }
					src := "docker"
					ln := strings.ToLower(name)
					switch {
					case strings.Contains(ln, "mysql") || strings.Contains(ln, "mariadb"):
						src = "mysql"
					case strings.Contains(ln, "postgres"):
						src = "postgres"
					case strings.Contains(ln, "mongo"):
						src = "mongodb"
					case strings.Contains(ln, "redis"):
						src = "redis"
					case strings.Contains(ln, "taos") || strings.Contains(ln, "tdengine"):
						src = "tdengine"
					case strings.Contains(ln, "influxdb"):
						src = "influxdb"
					case strings.Contains(ln, "prometheus"):
						src = "prometheus"
					case strings.Contains(ln, "elastic"):
						src = "elasticsearch"
					}
					app.DB.Create(&model.DeviceLog{
						Source: src, IP: ip, DeviceName: name,
						Level: guessLevel(line), Facility: "daemon",
						Message: line, CreatedAt: time.Now(),
					})
				}
			}
		}
		for k, v := range seen {
			if time.Since(v) > time.Hour { delete(seen, k) }
		}
	}
	collect()
	for range t.C { collect() }
}

// splitDockerLog 解析 docker logs 的多路复用帧流：每帧 8 字节头(stream+3pad+4BE size)
func splitDockerLog(b []byte) []string {
	var out []string
	for len(b) >= 8 {
		size := int(b[4])<<24 | int(b[5])<<16 | int(b[6])<<8 | int(b[7])
		if size < 0 || 8+size > len(b) { break }
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
	if strings.Contains(low, "warn") { return "warning" }
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
	case 0, 1, 2, 3: return "critical"
	case 4: return "warning"
	case 5, 6: return "info"
	default: return "debug"
	}
}

func facilityName(f int) string {
	if f >= 16 && f <= 23 {
		return fmt.Sprintf("local%d", f-16)
	}
	names := map[int]string{0:"kern",1:"user",2:"mail",3:"daemon",4:"auth",5:"syslog",6:"lpr",7:"news"}
	if n, ok := names[f]; ok { return n }
	return "user"
}

// mysqlLogCollector 每 5 分钟查询 performance_schema.error_log
func mysqlLogCollector(app *core.App) {
	seen := map[string]time.Time{}
	collect := func() {
		if app.DB == nil { return }
		type errLogRow struct {
			Logged    time.Time
			Level     string
			Code      string
			Subsystem string
			Message   string
		}
		var rows []errLogRow
		app.DB.Raw("SELECT logged, level, code, subsystem, data AS message FROM performance_schema.error_log ORDER BY logged DESC LIMIT 50").Scan(&rows)
		for _, r := range rows {
			key := r.Code + ":" + r.Message
			if _, ok := seen[key]; ok { continue }
			seen[key] = time.Now()
			lvl := "info"
			switch strings.ToLower(r.Level) {
			case "system", "note": lvl = "info"
			case "warning": lvl = "warning"
			case "error": lvl = "critical"
			}
			app.DB.Create(&model.DeviceLog{
				Source: "mysql", IP: "127.0.0.1", DeviceName: "mysql",
				Level: lvl, Facility: r.Subsystem, Message: r.Message, CreatedAt: r.Logged,
			})
		}
		for k, v := range seen {
			if time.Since(v) > time.Hour { delete(seen, k) }
		}
	}
	collect()
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C { collect() }
}

func init() { modreg.RegisterProtected("devicelog", RegisterProtected) }
