// Package snmptrap 接收设备主动上报的 SNMP Trap（只读平台侧，不改设备）
package snmptrap

import (
	"log"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gosnmp/gosnmp"

	"netops/internal/core"
	"netops/internal/modreg"
	"netops/internal/module/alert"
)

var started bool

func start(a *core.App) {
	if started {
		return
	}
	started = true
	go listen(a)
}

func listen(a *core.App) {
	l := &gosnmp.TrapListener{
		OnNewTrap: func(p *gosnmp.SnmpPacket, addr *net.UDPAddr) {
			handle(a, addr.IP.String(), p)
		},
		Params: &gosnmp.GoSNMP{
			Port: 162,
		},
	}
	log.Printf("[snmptrap] listening UDP :162")
	if err := l.Listen("udp://0.0.0.0:162"); err != nil {
		log.Printf("[snmptrap] listen failed: %v", err)
	}
}

func handle(a *core.App, addr string, p *gosnmp.SnmpPacket) {
	varBinds := []string{}
	for _, v := range p.Variables {
		val := formatValue(v.Value)
		varBinds = append(varBinds, v.Name+"="+val)
	}
	msg := strings.Join(varBinds, "; ")
	log.Printf("[snmptrap] from=%s type=%s msg=%s", addr, p.PDUType, msg)

	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "bgppeerdown") || strings.Contains(lower, "bgppeerstate"):
		log.Printf("[snmptrap] BGP neighbor event from=%s", addr)
		alert.Notify(a, "critical", "BGP邻居状态变化", "源="+addr+"\n"+msg)
	case strings.Contains(lower, "linkdown") || strings.Contains(lower, "ifoperstatus"):
		log.Printf("[snmptrap] interface event from=%s", addr)
		alert.Notify(a, "warning", "接口状态变化", "源="+addr+"\n"+msg)
	default:
		alert.Notify(a, "info", "SNMP Trap", "源="+addr+"\n"+msg)
	}
}

func formatValue(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

// RegisterProtected 路由
func RegisterProtected(a *core.App, g *gin.RouterGroup) {
	start(a)
	g.GET("/status", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "port": 162})
	})
}

func init() {
	modreg.RegisterProtected("snmptrap", RegisterProtected)
}
