// Package netutil 本机网络信息工具：获取非回环 IPv4（供展示"本机"真实地址使用）
package netutil

import "net"

// LocalIPv4 返回本机第一个非回环、已启用的 IPv4 地址。
// 遍历所有网卡，跳过 loopback 与未启用网卡（与 certsvc.collectAddrs 同源策略），
// 找不到时回退返回 "127.0.0.1" 占位。
func LocalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return "127.0.0.1"
}

// IsLoopbackAddr 判断给定 IP 是否为回环/本机地址（127.0.0.1、::1、localhost）
func IsLoopbackAddr(ip string) bool {
	switch ip {
	case "127.0.0.1", "::1", "localhost", "":
		return true
	}
	return false
}
