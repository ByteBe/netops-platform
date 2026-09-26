// Package containermon Docker / Kubernetes 监控模块
package containermon

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/collector/docker"
	"netops/internal/common/response"
	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/modreg"
)

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	g.GET("/docker/status", func(c *gin.Context) {
		ver, err := docker.DockerAvailable()
		response.OK(c, gin.H{"enabled": a.DockerM.Enabled(), "available": err == nil, "version": ver, "error": errMsg(err)})
	})
	g.GET("/docker/containers", func(c *gin.Context) {
		response.OK(c, a.DockerM.Snapshot())
	})
	g.POST("/docker/collect", func(c *gin.Context) {
		if !a.DockerM.Enabled() { response.Bad(c, "Docker 监控未启用"); return }
		a.DockerM.Collect()
		response.OK(c, gin.H{"ok": true, "count": len(a.DockerM.Snapshot())})
	})

	g.GET("/docker/hosts", func(c *gin.Context) {
		var hosts []model.DockerHost
		a.DB.Order("id asc").Find(&hosts)
		status := a.DockerM.HostStatus()
		result := []gin.H{}
		for _, h := range hosts {
			online, has := status[h.Address]
			if h.Address == "" { online, has = status["本机"] }
			if !has { online = false }
			result = append(result, gin.H{"id": h.ID, "name": h.Name, "address": h.Address, "enabled": h.Enabled, "remark": h.Remark, "online": online})
		}
		response.OK(c, result)
	})
	g.POST("/docker/hosts", func(c *gin.Context) {
		var h model.DockerHost
		if err := c.ShouldBindJSON(&h); err != nil { response.Bad(c, "参数错误"); return }
		if h.Name == "" { response.Bad(c, "主机名称必填"); return }
		a.DB.Create(&h)
		applyHosts(a)
		response.OK(c, h)
	})
	g.PUT("/docker/hosts/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var h model.DockerHost
		if err := a.DB.First(&h, id).Error; err != nil { response.NotFound(c, "不存在"); return }
		var req model.DockerHost
		c.ShouldBindJSON(&req)
		a.DB.Model(&h).Updates(map[string]any{"name": req.Name, "address": req.Address, "enabled": req.Enabled, "remark": req.Remark})
		applyHosts(a)
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/docker/hosts/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.DockerHost{}, id)
		applyHosts(a)
		response.OK(c, gin.H{"ok": true})
	})
	g.POST("/docker/test", func(c *gin.Context) {
		var req struct{ Host string `json:"host"` }
		c.ShouldBindJSON(&req)
		ver, err := docker.TestHost(req.Host)
		if err != nil { response.OK(c, gin.H{"ok": false, "error": err.Error()}); return }
		response.OK(c, gin.H{"ok": true, "version": ver})
	})

	g.GET("/k8s/status", func(c *gin.Context) { response.OK(c, gin.H{"enabled": a.K8sM.Enabled()}) })
	g.GET("/k8s/nodes", func(c *gin.Context) { response.OK(c, a.K8sM.Snapshot()) })
	g.POST("/k8s/collect", func(c *gin.Context) {
		if !a.K8sM.Enabled() { response.Bad(c, "K8s未启用"); return }
		a.K8sM.Collect()
		response.OK(c, gin.H{"ok": true})
	})
	g.POST("/k8s/test", func(c *gin.Context) {
		var req struct {
			APIServer string `json:"api_server"`
			Token     string `json:"token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.APIServer == "" {
			response.Bad(c, "API Server必填")
			return
		}
		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		client := &http.Client{Transport: tr, Timeout: 8 * time.Second}
		req2, _ := http.NewRequest("GET", req.APIServer+"/version", nil)
		if req.Token != "" {
			req2.Header.Set("Authorization", "Bearer "+req.Token)
		}
		resp, err := client.Do(req2)
		if err != nil {
			response.OK(c, gin.H{"ok": false, "error": err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			response.OK(c, gin.H{"ok": false, "error": "HTTP " + resp.Status})
			return
		}
		var v struct {
			GitVersion string `json:"gitVersion"`
		}
		json.NewDecoder(resp.Body).Decode(&v)
		response.OK(c, gin.H{"ok": true, "version": v.GitVersion})
	})

	// K8s集群CRUD
	g.GET("/k8s/clusters", func(c *gin.Context) {
		var list []model.K8sCluster
		a.DB.Order("id asc").Find(&list)
		status := a.K8sM.ClusterStatus()
		result := []gin.H{}
		for _, k := range list {
			online, has := status[k.Name]
			if !has { online = false }
			result = append(result, gin.H{"id": k.ID, "name": k.Name, "api_server": k.APIServer, "token": k.Token, "enabled": k.Enabled, "remark": k.Remark, "online": online})
		}
		response.OK(c, result)
	})
	g.POST("/k8s/clusters", func(c *gin.Context) {
		var k model.K8sCluster
		if err := c.ShouldBindJSON(&k); err != nil { response.Bad(c, "参数错误"); return }
		if k.Name == "" || k.APIServer == "" { response.Bad(c, "名称和API Server必填"); return }
		a.DB.Create(&k)
		response.OK(c, k)
	})
	g.PUT("/k8s/clusters/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var k model.K8sCluster
		if err := a.DB.First(&k, id).Error; err != nil { response.NotFound(c, "不存在"); return }
		var req model.K8sCluster
		c.ShouldBindJSON(&req)
		a.DB.Model(&k).Updates(map[string]any{"name": req.Name, "api_server": req.APIServer, "token": req.Token, "enabled": req.Enabled, "remark": req.Remark})
		response.OK(c, gin.H{"ok": true})
	})
	g.DELETE("/k8s/clusters/:id", func(c *gin.Context) {
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		a.DB.Delete(&model.K8sCluster{}, id)
		response.OK(c, gin.H{"ok": true})
	})

	g.POST("/config", func(c *gin.Context) {
		var req struct {
			DockerEnable   bool   `json:"docker_enable"`
			DockerInterval int    `json:"docker_interval"`
			K8sEnable      bool   `json:"k8s_enable"`
			K8sAPIServer   string `json:"k8s_api_server"`
			K8sToken       string `json:"k8s_token"`
			K8sInterval    int    `json:"k8s_interval"`
		}
		if err := c.ShouldBindJSON(&req); err != nil { response.Bad(c, "参数错误"); return }
		set(a, "docker_enable", boolStr(req.DockerEnable))
		set(a, "docker_interval", itoa(req.DockerInterval))
		set(a, "k8s_enable", boolStr(req.K8sEnable))
		set(a, "k8s_api_server", req.K8sAPIServer)
		set(a, "k8s_token", req.K8sToken)
		set(a, "k8s_interval", itoa(req.K8sInterval))
		a.DockerM.SetConfig(req.DockerEnable, req.DockerInterval, hostsJSON(a))
		a.K8sM.SetConfigJSON(req.K8sEnable, req.K8sInterval, k8sClustersJSON(a))
		response.OK(c, gin.H{"ok": true})
	})
	g.GET("/config", func(c *gin.Context) {
		response.OK(c, gin.H{
			"docker_enable": get(a, "docker_enable") == "true",
			"docker_interval": atoi(get(a, "docker_interval"), 30),
			"k8s_enable": get(a, "k8s_enable") == "true",
			"k8s_api_server": get(a, "k8s_api_server"),
			"k8s_token": get(a, "k8s_token"),
			"k8s_interval": atoi(get(a, "k8s_interval"), 60),
		})
	})
}

func hostsJSON(a *core.App) string {
	var hosts []model.DockerHost
	a.DB.Where("enabled = ?", true).Find(&hosts)
	addrs := []string{}
	for _, h := range hosts {
		if h.Address != "" { addrs = append(addrs, h.Address) }
	}
	b, _ := json.Marshal(addrs)
	return string(b)
}

func k8sClustersJSON(a *core.App) string {
	var clusters []model.K8sCluster
	a.DB.Where("enabled = ?", true).Find(&clusters)
	type kc struct {
		Name      string `json:"name"`
		APIServer string `json:"api_server"`
		Token     string `json:"token"`
	}
	arr := []kc{}
	for _, cl := range clusters {
		arr = append(arr, kc{Name: cl.Name, APIServer: cl.APIServer, Token: cl.Token})
	}
	b, _ := json.Marshal(arr)
	return string(b)
}

func applyHosts(a *core.App) {
	enable := get(a, "docker_enable") == "true"
	interval := atoi(get(a, "docker_interval"), 30)
	a.DockerM.SetConfig(enable, interval, hostsJSON(a))
	kenable := get(a, "k8s_enable") == "true"
	kinterval := atoi(get(a, "k8s_interval"), 60)
	a.K8sM.SetConfigJSON(kenable, kinterval, k8sClustersJSON(a))
}

func set(a *core.App, k, v string) {
	var s model.SystemSetting
	if err := a.DB.Where("key = ?", k).First(&s).Error; err != nil {
		a.DB.Create(&model.SystemSetting{Key: k, Value: v})
	} else {
		a.DB.Model(&s).Update("value", v)
	}
}
func get(a *core.App, k string) string {
	var s model.SystemSetting
	if err := a.DB.Where("key = ?", k).First(&s).Error; err != nil { return "" }
	return s.Value
}
func boolStr(b bool) string { if b { return "true" }; return "false" }
func itoa(v int) string { return strconv.Itoa(v) }
func atoi(s string, def int) int {
	v, err := strconv.Atoi(s); if err != nil { return def }; return v
}
func errMsg(err error) string { if err == nil { return "" }; return err.Error() }

func init() { modreg.RegisterProtected("containermon", RegisterProtected) }