//go:build windows

package aiinspect

// Windows 平台本机资源采集：不直接读取系统性能数据（避免平台 API 差异），
// 本机资源通过纳管设备方式（SNMP/SSH）采集，未纳管时该字段为空

func readCPU() (idle, total uint64, ok bool) { return 0, 0, false }

func readMem() (float64, bool) { return 0, false }

func readDisk() (float64, bool) { return 0, false }

func readUptime() (float64, bool) { return 0, false }
