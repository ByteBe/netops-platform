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

// probeTDengine 通过 REST API 采集 TDengine
func (m *Manager) probeTDengine(d Instance, snap Snapshot) Snapshot {
	dbn := d.DBName
	if dbn == "" {
		dbn = "log"
	}
	tdQuery := func(sql string) (any, error) {
		url := fmt.Sprintf("http://%s:%d/rest/sql/%s", d.Host, d.Port, dbn)
		req, _ := http.NewRequest("POST", url, strings.NewReader(sql))
		req.SetBasicAuth(d.User, d.Password)
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
	if v, err := tdQuery("SELECT SERVER_VERSION()"); err == nil && v != nil {
		snap.Version = fmt.Sprint(v)
	}
	snap.Up = true
	snap.Status = "up"
	// TDengine 无连接数概念，改为采集数据库数量（ins_databases）
	if v, err := tdQuery("SELECT COUNT(*) FROM information_schema.ins_databases"); err == nil && v != nil {
		snap.Conns, _ = strconv.ParseFloat(fmt.Sprint(v), 64)
	}
	if v, err := tdQuery("SELECT COUNT(*) FROM information_schema.ins_tables"); err == nil && v != nil {
		snap.TableCount, _ = strconv.ParseFloat(fmt.Sprint(v), 64)
	}
	// 数据目录大小
	for _, dir := range []string{"/opt/tdengine/data", "/var/lib/taos"} {
		if out, err := exec.Command("du", "-sb", dir).Output(); err == nil {
			fields := strings.Fields(strings.TrimSpace(string(out)))
			if n, err := strconv.ParseFloat(fields[0], 64); err == nil && n > 0 {
				snap.DataSize = n
				break
			}
		}
	}
	var uptime float64
	snap.CpuPercent, snap.MemUsed, snap.MemTotal, _, uptime = hostStats(d.ID, d.Host)
	if snap.Uptime == 0 {
		snap.Uptime = uptime
	}
	return snap
}
