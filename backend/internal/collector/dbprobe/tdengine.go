package dbprobe

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var tdHTTPClient = &http.Client{Timeout: 15 * time.Second}

// tdRestQuery TDengine REST 查询，返回首行首列值
func tdRestQuery(host string, port int, db, user, pass, sql string) (any, error) {
	url := fmt.Sprintf("http://%s:%d/rest/sql/%s", host, port, db)
	req, _ := http.NewRequest("POST", url, strings.NewReader(sql))
	req.SetBasicAuth(user, pass)
	resp, err := tdHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Code int             `json:"code"`
		Desc string          `json:"desc"`
		Data [][]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.Code != 0 {
		return nil, fmt.Errorf("%s", body.Desc)
	}
	if len(body.Data) > 0 && len(body.Data[0]) > 0 {
		return body.Data[0][0], nil
	}
	return nil, nil
}

// tdNodeUptime 通过 SHOW DNODES 的 reboot_time 计算 TDengine 节点真实运行时长（秒）
func tdNodeUptime(host string, port int, db, user, pass string) float64 {
	url := fmt.Sprintf("http://%s:%d/rest/sql/%s", host, port, db)
	req, _ := http.NewRequest("POST", url, strings.NewReader("SHOW DNODES"))
	req.SetBasicAuth(user, pass)
	resp, err := tdHTTPClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var body struct {
		Code int             `json:"code"`
		Data [][]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Code != 0 || len(body.Data) == 0 {
		return 0
	}
	// data[0]: [id, endpoint, vnodes, support_vnodes, status, create_time, reboot_time, note]
	row := body.Data[0]
	if len(row) < 7 {
		return 0
	}
	ts, ok := row[6].(string)
	if !ok || ts == "" {
		return 0
	}
	var t time.Time
	for _, layout := range []string{"2006-01-02T15:04:05.000Z", "2006-01-02T15:04:05Z", "2006-01-02 15:04:05.000", "2006-01-02 15:04:05"} {
		if tt, err := time.Parse(layout, ts); err == nil {
			t = tt
			break
		}
	}
	if t.IsZero() {
		return 0
	}
	d := time.Since(t).Seconds()
	if d > 0 {
		return d
	}
	return 0
}

// taosProcUptime 通过 taosd 进程获取运行秒数（回退方案）
func taosProcUptime() (float64, error) {
	out, err := exec.Command("sh", "-c", "pgrep -f taosd | head -1").Output()
	if err != nil {
		return 0, err
	}
	pid := strings.TrimSpace(string(out))
	if pid == "" {
		return 0, fmt.Errorf("taosd 进程未找到")
	}
	out2, err := exec.Command("ps", "-o", "etimes=", "-p", pid).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out2)), 64)
}

// probeTDengine 通过 REST API 采集 TDengine
func (m *Manager) probeTDengine(d Instance, snap Snapshot) Snapshot {
	dbn := d.DBName
	if dbn == "" {
		dbn = "log"
	}
	// 查询优先使用实例库，失败回退系统库 log（避免 DBName 配置不当导致采集全空）
	tryQuery := func(sql string) (any, error) {
		if v, err := tdRestQuery(d.Host, d.Port, dbn, d.User, d.Password, sql); err == nil {
			return v, nil
		} else if dbn != "log" {
			if v2, err2 := tdRestQuery(d.Host, d.Port, "log", d.User, d.Password, sql); err2 == nil {
				return v2, nil
			}
		}
		// 认证/连接失败场景直接抛错，供上层判定 down
		return tdRestQuery(d.Host, d.Port, dbn, d.User, d.Password, sql)
	}
	// 连接判定：版本查询成功才视为 up
	if v, err := tryQuery("SELECT SERVER_VERSION()"); err == nil && v != nil {
		snap.Version = fmt.Sprint(v)
		snap.Up = true
		snap.Status = "up"
	} else {
		snap.Up = false
		snap.Status = "down"
		if err != nil {
			snap.Message = "TDengine 连接失败: " + err.Error()
		} else {
			snap.Message = "TDengine 连接失败: 版本查询无返回"
		}
		snap.CpuPercent, snap.MemUsed, snap.MemTotal, _, _ = hostStats(d.ID, d.Host)
		if u, uerr := taosProcUptime(); uerr == nil {
			snap.Uptime = u
		}
		return snap
	}
	// TDengine 无连接数概念，改为采集数据库数量（ins_databases）
	if v, err := tryQuery("SELECT COUNT(*) FROM information_schema.ins_databases"); err == nil && v != nil {
		snap.Conns, _ = strconv.ParseFloat(fmt.Sprint(v), 64)
	}
	if v, err := tryQuery("SELECT COUNT(*) FROM information_schema.ins_tables"); err == nil && v != nil {
		snap.TableCount, _ = strconv.ParseFloat(fmt.Sprint(v), 64)
	}
	// 数据目录大小
	for _, dir := range []string{"/root/tdengine/data", "/opt/tdengine/data", "/var/lib/taos"} {
		if out, err := exec.Command("du", "-sb", dir).Output(); err == nil {
			fields := strings.Fields(strings.TrimSpace(string(out)))
			if n, err := strconv.ParseFloat(fields[0], 64); err == nil && n > 0 {
				snap.DataSize = n
				break
			}
		}
	}
	// 运行时长：SHOW DNODES reboot_time 优先（节点真实运行时长），回退进程时长
	snap.Uptime = tdNodeUptime(d.Host, d.Port, dbn, d.User, d.Password)
	if snap.Uptime <= 0 {
		if u, uerr := taosProcUptime(); uerr == nil {
			snap.Uptime = u
		}
	}
	// 宿主 CPU/内存（TDengine 与 mysql 同机部署时取宿主机资源）
	snap.CpuPercent, snap.MemUsed, snap.MemTotal, _, _ = hostStats(d.ID, d.Host)
	return snap
}
